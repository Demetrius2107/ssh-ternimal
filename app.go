package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-terminal/internal/localfs"
	"ssh-terminal/internal/model"
	"ssh-terminal/internal/store"
	"ssh-terminal/internal/sshcore"
	"ssh-terminal/internal/telnetcore"
	"ssh-terminal/internal/transfer"
)

// SshOutput 终端输出事件负载
type SshOutput struct {
	SessionID uint64 `json:"sessionId"`
	Data      string `json:"data"`
}

// SshExit 会话退出事件负载
type SshExit struct {
	SessionID uint64 `json:"sessionId"`
	Error     string `json:"error"`
}

// SshReconnect 断线重连事件负载
type SshReconnect struct {
	SessionID uint64 `json:"sessionId"`
	Attempt   int    `json:"attempt"`
	Max       int    `json:"max"`
}

// App 应用结构 (wails 绑定层, 方法转发到 internal 包)
type App struct {
	ctx      context.Context
	sessions map[uint64]model.TermSession
	history  map[uint64]*historyFile
	nextID   uint64
	mu       sync.Mutex

	connConfigs map[uint64]model.SshConfig // 断线重连用的连接配置
	manualClose map[uint64]bool            // 用户主动断开标记 (不自动重连)
	cpuPrev     map[uint64]cpuSample       // CPU 采样差值基准 (资源监控)

	engine *transfer.Engine
	store  *store.Store

	tunnels         map[uint64]*model.Tunnel
	tunnelListeners map[uint64]net.Listener
	nextTunnelID    uint64
	tunnelMu        sync.Mutex
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{
		sessions:        make(map[uint64]model.TermSession),
		history:         make(map[uint64]*historyFile),
		connConfigs:     make(map[uint64]model.SshConfig),
		manualClose:     make(map[uint64]bool),
		cpuPrev:         make(map[uint64]cpuSample),
		tunnels:         make(map[uint64]*model.Tunnel),
		tunnelListeners: make(map[uint64]net.Listener),
		engine:          transfer.NewEngine(),
	}
}

// startup 保存上下文、初始化会话存储、接线进度事件
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	st, err := store.Open()
	if err != nil {
		println("初始化会话存储失败:", err.Error())
		return
	}
	a.store = st
	a.engine.SetProgressHandler(func(t model.TransferTask) {
		runtime.EventsEmit(ctx, "sftp-progress", t)
	})
}

// getSession 按 ID 取会话 (通用终端接口)
func (a *App) getSession(id uint64) (model.TermSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.sessions[id]; ok {
		return s, nil
	}
	return nil, errors.New("会话不存在")
}

// getSSH 取 SSH 会话 (SFTP 等仅 SSH 支持的操作)
func (a *App) getSSH(id uint64) (*sshcore.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.sessions[id]; ok {
		if ssh, ok := s.(*sshcore.Session); ok {
			return ssh, nil
		}
		return nil, errors.New("该连接不支持此操作 (仅 SSH)")
	}
	return nil, errors.New("会话不存在")
}

// ---------- SSH 终端 ----------

// Connect 建立终端会话 (SSH/Telnet), 返回会话 ID
func (a *App) Connect(cfg model.SshConfig) (uint64, error) {
	proto := cfg.Protocol
	if proto == "" {
		proto = "ssh"
	}
	var sess model.TermSession
	var err error
	switch proto {
	case "telnet":
		sess, err = telnetcore.Connect(cfg.Host, cfg.Port)
	default:
		sess, err = sshcore.Connect(cfg)
	}
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	a.sessions[id] = sess
	a.connConfigs[id] = cfg
	a.mu.Unlock()

	// 历史记录持久化 (会话输出实时落盘)
	hf, herr := openHistory(fmt.Sprintf("%s:%s:%d", proto, cfg.Host, cfg.Port))
	if herr != nil {
		println("历史记录不可用:", herr.Error())
	}
	if hf != nil {
		a.mu.Lock()
		a.history[id] = hf
		a.mu.Unlock()
	}

	a.attachSession(id, sess, cfg)
	return id, nil
}

// attachSession 挂接会话输出泵与断线处理 (初次连接与自动重连共用)
func (a *App) attachSession(id uint64, sess model.TermSession, cfg model.SshConfig) {
	a.mu.Lock()
	hf := a.history[id]
	a.mu.Unlock()

	go func() {
		for msg := range sess.Output() {
			runtime.EventsEmit(a.ctx, "ssh-output", SshOutput{SessionID: id, Data: msg.Data})
			if hf != nil {
				hf.write(msg.Data)
			}
		}
	}()
	go func() {
		err := <-sess.Done()
		// 意外断开 (非用户主动关闭) 时自动重连
		if err != nil && !a.isManualClose(id) {
			if a.tryReconnect(id, cfg) {
				return
			}
		}
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		runtime.EventsEmit(a.ctx, "ssh-exit", SshExit{SessionID: id, Error: msg})
		a.mu.Lock()
		delete(a.sessions, id)
		delete(a.connConfigs, id)
		delete(a.manualClose, id)
		if hf != nil {
			hf.close()
			delete(a.history, id)
		}
		a.mu.Unlock()
	}()
}

// tryReconnect 断线重连: 最多 3 次, 退避 2s/4s/6s
func (a *App) tryReconnect(id uint64, cfg model.SshConfig) bool {
	for attempt := 1; attempt <= 3; attempt++ {
		runtime.EventsEmit(a.ctx, "ssh-reconnect", SshReconnect{SessionID: id, Attempt: attempt, Max: 3})
		time.Sleep(time.Duration(attempt*2) * time.Second)
		var ns model.TermSession
		var err error
		if cfg.Protocol == "telnet" {
			ns, err = telnetcore.Connect(cfg.Host, cfg.Port)
		} else {
			ns, err = sshcore.Connect(cfg)
		}
		if err != nil {
			continue
		}
		a.mu.Lock()
		a.sessions[id] = ns
		a.mu.Unlock()
		a.attachSession(id, ns, cfg)
		return true
	}
	return false
}

// isManualClose 是否为用户主动断开
func (a *App) isManualClose(id uint64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.manualClose[id]
}

// SshSend 发送终端输入
func (a *App) SshSend(id uint64, data string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Send(data)
}

// SshResize 调整远程 PTY 尺寸
func (a *App) SshResize(id uint64, rows, cols int) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Resize(rows, cols)
}

// SshClose 关闭会话
func (a *App) SshClose(id uint64) {
	a.mu.Lock()
	sess, ok := a.sessions[id]
	delete(a.sessions, id)
	if hf, ok2 := a.history[id]; ok2 {
		hf.close()
		delete(a.history, id)
	}
	a.manualClose[id] = true
	a.mu.Unlock()
	// 会话关闭时联动停止其隧道
	a.tunnelMu.Lock()
	for tid, t := range a.tunnels {
		if t.SessionID == id {
			if ln, ok := a.tunnelListeners[tid]; ok {
				ln.Close()
				delete(a.tunnelListeners, tid)
			}
			t.Status = "stopped"
		}
	}
	a.tunnelMu.Unlock()
	if ok {
		sess.Close()
	}
}

// StartTunnel 启动 SSH 端口转发 (local/dynamic/remote), 返回隧道 ID
func (a *App) StartTunnel(id uint64, tunnelType string, listenPort int, targetAddr string) (uint64, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return 0, err
	}
	listenAddr := fmt.Sprintf("127.0.0.1:%d", listenPort)
	var ln net.Listener
	switch tunnelType {
	case "local":
		ln, err = startTunnelLocal(sess, listenAddr, targetAddr)
	case "dynamic":
		ln, err = startTunnelDynamic(sess, listenAddr)
	case "remote":
		ln, err = startTunnelRemote(sess, listenAddr, targetAddr)
	default:
		return 0, fmt.Errorf("未知隧道类型: %s", tunnelType)
	}
	if err != nil {
		return 0, err
	}
	a.tunnelMu.Lock()
	a.nextTunnelID++
	tid := a.nextTunnelID
	a.tunnels[tid] = &model.Tunnel{ID: tid, SessionID: id, Type: tunnelType, ListenAddr: listenAddr, TargetAddr: targetAddr, Status: "running"}
	a.tunnelListeners[tid] = ln
	a.tunnelMu.Unlock()
	return tid, nil
}

// StopTunnel 停止隧道
func (a *App) StopTunnel(tid uint64) error {
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	ln, ok := a.tunnelListeners[tid]
	if !ok {
		return errors.New("隧道不存在")
	}
	ln.Close()
	delete(a.tunnelListeners, tid)
	if t, ok := a.tunnels[tid]; ok {
		t.Status = "stopped"
	}
	return nil
}

// ListTunnels 列出全部隧道
func (a *App) ListTunnels() []model.Tunnel {
	a.tunnelMu.Lock()
	defer a.tunnelMu.Unlock()
	out := make([]model.Tunnel, 0, len(a.tunnels))
	for _, t := range a.tunnels {
		out = append(out, *t)
	}
	return out
}

// GetSessionMetrics 会话实时指标 (前端状态栏轮询)
func (a *App) GetSessionMetrics(id uint64) (model.Metrics, error) {
	a.mu.Lock()
	sess, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return model.Metrics{}, errors.New("会话不存在")
	}
	return sess.Metrics(), nil
}

// SshKeepAlive 手动保活, 返回 RTT 毫秒
func (a *App) SshKeepAlive(id uint64) (int64, error) {
	a.mu.Lock()
	sess, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return 0, errors.New("会话不存在")
	}
	return sess.KeepAlive()
}

// cpuSample CPU 采样
type cpuSample struct {
	busy  uint64
	total uint64
}

// GetSysMetrics 远程主机资源指标 (CPU 使用率 / 内存 / 网络 / 运行时长)
func (a *App) GetSysMetrics(id uint64) (model.SysMetrics, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return model.SysMetrics{}, err
	}
	const cmd = "cat /proc/stat /proc/meminfo /proc/net/dev /proc/uptime"
	out, err := sess.Exec(cmd)
	if err != nil {
		return model.SysMetrics{}, err
	}
	r := parseProcMetrics(out)

	// CPU 使用率: 与上次采样差值计算
	a.mu.Lock()
	prev, ok := a.cpuPrev[id]
	a.cpuPrev[id] = cpuSample{busy: r.cpuBusy, total: r.cpuTotal}
	a.mu.Unlock()
	if ok && prev.total > 0 && r.cpuTotal > prev.total {
		db := r.cpuBusy - prev.busy
		dt := r.cpuTotal - prev.total
		if dt > 0 {
			r.m.CPUPercent = float64(db) / float64(dt) * 100
			if r.m.CPUPercent > 100 {
				r.m.CPUPercent = 100
			}
		}
	}
	return r.m, nil
}

// procRaw /proc 输出解析结果
type procRaw struct {
	m        model.SysMetrics
	cpuBusy  uint64
	cpuTotal uint64
}

// parseProcMetrics 解析 cat /proc/stat /proc/meminfo /proc/net/dev /proc/uptime 的输出
func parseProcMetrics(out string) procRaw {
	var r procRaw
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "cpu "):
			f := strings.Fields(line)
			var vals [8]uint64
			for i := 1; i < len(f) && i <= 8; i++ {
				vals[i-1], _ = strconv.ParseUint(f[i], 10, 64)
			}
			idle := vals[3] + vals[4] // idle + iowait
			for _, v := range vals {
				r.cpuTotal += v
			}
			r.cpuBusy = r.cpuTotal - idle
		case strings.HasPrefix(line, "MemTotal:"):
			f := strings.Fields(line)
			r.m.MemTotal, _ = strconv.ParseUint(f[1], 10, 64)
		case strings.HasPrefix(line, "MemAvailable:"):
			f := strings.Fields(line)
			if v, err := strconv.ParseUint(f[1], 10, 64); err == nil {
				r.m.MemUsed = r.m.MemTotal - v
			}
		case strings.Contains(line, ":"):
			// /proc/net/dev 行: iface: rx_bytes ... tx_bytes ...
			if strings.HasPrefix(line, "Inter-") || strings.HasPrefix(line, "face") {
				continue
			}
			idx := strings.Index(line, ":")
			fields := strings.Fields(line[idx+1:])
			if len(fields) >= 9 {
				rx, _ := strconv.ParseUint(fields[0], 10, 64)
				tx, _ := strconv.ParseUint(fields[8], 10, 64)
				r.m.NetIn += rx
				r.m.NetOut += tx
			}
		default:
			// /proc/uptime: 两个浮点数
			f := strings.Fields(line)
			if len(f) == 2 {
				if a, err := strconv.ParseFloat(f[0], 64); err == nil {
					if _, err2 := strconv.ParseFloat(f[1], 64); err2 == nil {
						r.m.Uptime = a
					}
				}
			}
		}
	}
	return r
}

// ---------- SFTP 远程文件 ----------

// SftpPwd 远程当前目录
func (a *App) SftpPwd(id uint64) (string, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return "", err
	}
	return sess.Pwd()
}

// SftpListDir 列出远程目录
func (a *App) SftpListDir(id uint64, dir string) ([]model.FileEntry, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return nil, err
	}
	return sess.ListDir(dir)
}

// SftpMkdir 远程新建目录
func (a *App) SftpMkdir(id uint64, dir string) error {
	sess, err := a.getSSH(id)
	if err != nil {
		return err
	}
	return sess.Mkdir(dir)
}

// SftpDelete 远程删除
func (a *App) SftpDelete(id uint64, p string) error {
	sess, err := a.getSSH(id)
	if err != nil {
		return err
	}
	return sess.Delete(p)
}

// SftpRename 远程重命名
func (a *App) SftpRename(id uint64, oldP, newP string) error {
	sess, err := a.getSSH(id)
	if err != nil {
		return err
	}
	return sess.Rename(oldP, newP)
}

// SftpChmod 远程修改权限
func (a *App) SftpChmod(id uint64, p string, mode uint32) error {
	sess, err := a.getSSH(id)
	if err != nil {
		return err
	}
	return sess.Chmod(p, mode)
}

// SftpUpload 异步上传 (文件或目录), 返回任务 ID; conflict: overwrite/skip/rename
func (a *App) SftpUpload(id uint64, localPath, remotePath, conflict string) (uint64, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return 0, err
	}
	return a.engine.Upload(id, sess, localPath, remotePath, conflict)
}

// SftpDownload 异步下载 (文件或目录), 返回任务 ID; conflict: overwrite/skip/rename
func (a *App) SftpDownload(id uint64, remotePath, localPath, conflict string) (uint64, error) {
	sess, err := a.getSSH(id)
	if err != nil {
		return 0, err
	}
	return a.engine.Download(id, sess, remotePath, localPath, conflict)
}

// SftpTasks 返回全部传输任务快照
func (a *App) SftpTasks() []model.TransferTask {
	return a.engine.Tasks()
}

// ---------- 本地文件 ----------

// LocalListDir 列出本地目录
func (a *App) LocalListDir(dir string) ([]model.FileEntry, error) {
	return localfs.ListDir(dir)
}

// LocalParent 本地上级目录 (根目录返回空串)
func (a *App) LocalParent(dir string) string {
	return localfs.Parent(dir)
}

// LocalMkdir 本地新建目录
func (a *App) LocalMkdir(dir string) error {
	return localfs.Mkdir(dir)
}

// LocalDelete 本地删除
func (a *App) LocalDelete(p string) error {
	return localfs.Delete(p)
}

// LocalRename 本地重命名
func (a *App) LocalRename(oldP, newP string) error {
	return localfs.Rename(oldP, newP)
}

// ---------- 原生文件对话框 ----------

// PickFile 弹出文件选择对话框, 返回选中路径
func (a *App) PickFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择文件"})
}

// PickDir 弹出目录选择对话框, 返回选中路径
func (a *App) PickDir() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择目录"})
}

// LaunchRdp 外接系统 RDP 客户端 (Windows mstsc, 生成临时 .rdp 配置)
func (a *App) LaunchRdp(host string, port int, username string) error {
	if host == "" {
		return errors.New("主机不能为空")
	}
	f, err := os.CreateTemp("", "ssh-terminal-*.rdp")
	if err != nil {
		return err
	}
	name := f.Name()
	content := fmt.Sprintf("full address:s:%s:%d\nusername:s:%s\nprompt for credentials:i:1\n", host, port, username)
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	f.Close()
	cmd := exec.Command("mstsc", name)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 mstsc 失败: %v", err)
	}
	return nil
}

// ---------- 会话管理 ----------

// SaveSession 保存会话配置, 密码存系统凭据库, 返回会话 ID
func (a *App) SaveSession(name, host string, port int, username, password string) (string, error) {
	if a.store == nil {
		return "", errors.New("会话存储未初始化")
	}
	return a.store.Save(model.StoredSession{Name: name, Host: host, Port: port, Username: username}, password)
}

// ListSessions 列出全部保存的会话
func (a *App) ListSessions() ([]model.StoredSession, error) {
	if a.store == nil {
		return nil, errors.New("会话存储未初始化")
	}
	return a.store.List()
}

// DeleteSession 删除会话及其密码
func (a *App) DeleteSession(id string) error {
	if a.store == nil {
		return errors.New("会话存储未初始化")
	}
	return a.store.Delete(id)
}

// LoadSession 加载会话配置与密码
func (a *App) LoadSession(id string) (model.SshConfig, error) {
	if a.store == nil {
		return model.SshConfig{}, errors.New("会话存储未初始化")
	}
	sess, pw, err := a.store.Load(id)
	if err != nil {
		return model.SshConfig{}, err
	}
	return model.SshConfig{Host: sess.Host, Port: sess.Port, Username: sess.Username, Password: pw}, nil
}
