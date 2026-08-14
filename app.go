package main

import (
	"context"
	"errors"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-terminal/internal/localfs"
	"ssh-terminal/internal/model"
	"ssh-terminal/internal/store"
	"ssh-terminal/internal/sshcore"
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

// App 应用结构 (wails 绑定层, 方法转发到 internal 包)
type App struct {
	ctx      context.Context
	sessions map[uint64]*sshcore.Session
	nextID   uint64
	mu       sync.Mutex

	engine *transfer.Engine
	store  *store.Store
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{
		sessions: make(map[uint64]*sshcore.Session),
		engine:   transfer.NewEngine(),
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

// getSession 按 ID 取会话
func (a *App) getSession(id uint64) (*sshcore.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.sessions[id]; ok {
		return s, nil
	}
	return nil, errors.New("会话不存在")
}

// ---------- SSH 终端 ----------

// SshConnect 建立 SSH 会话, 返回会话 ID
func (a *App) SshConnect(cfg model.SshConfig) (uint64, error) {
	sess, err := sshcore.Connect(cfg)
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	a.sessions[id] = sess
	a.mu.Unlock()

	go func() {
		for msg := range sess.Output() {
			runtime.EventsEmit(a.ctx, "ssh-output", SshOutput{SessionID: id, Data: msg.Data})
		}
	}()
	go func() {
		err := <-sess.Done()
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		runtime.EventsEmit(a.ctx, "ssh-exit", SshExit{SessionID: id, Error: msg})
		a.mu.Lock()
		delete(a.sessions, id)
		a.mu.Unlock()
	}()
	return id, nil
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
	a.mu.Unlock()
	if ok {
		sess.Close()
	}
}

// ---------- SFTP 远程文件 ----------

// SftpPwd 远程当前目录
func (a *App) SftpPwd(id uint64) (string, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return "", err
	}
	return sess.Pwd()
}

// SftpListDir 列出远程目录
func (a *App) SftpListDir(id uint64, dir string) ([]model.FileEntry, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return nil, err
	}
	return sess.ListDir(dir)
}

// SftpMkdir 远程新建目录
func (a *App) SftpMkdir(id uint64, dir string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Mkdir(dir)
}

// SftpDelete 远程删除
func (a *App) SftpDelete(id uint64, p string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Delete(p)
}

// SftpRename 远程重命名
func (a *App) SftpRename(id uint64, oldP, newP string) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Rename(oldP, newP)
}

// SftpChmod 远程修改权限
func (a *App) SftpChmod(id uint64, p string, mode uint32) error {
	sess, err := a.getSession(id)
	if err != nil {
		return err
	}
	return sess.Chmod(p, mode)
}

// SftpUpload 异步上传, 返回任务 ID
func (a *App) SftpUpload(id uint64, localPath, remotePath string) (uint64, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return 0, err
	}
	return a.engine.Upload(id, sess, localPath, remotePath)
}

// SftpDownload 异步下载, 返回任务 ID
func (a *App) SftpDownload(id uint64, remotePath, localPath string) (uint64, error) {
	sess, err := a.getSession(id)
	if err != nil {
		return 0, err
	}
	return a.engine.Download(id, sess, remotePath, localPath)
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
