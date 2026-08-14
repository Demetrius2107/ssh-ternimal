package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/crypto/ssh"
)

// SshConfig 连接配置（由前端 JSON 传入）
type SshConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"` // 私钥 PEM 内容
	Passphrase string `json:"passphrase"`
}

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

// SshSession 单个 SSH 会话
type SshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	mu      sync.Mutex
	closed  bool
}

// App 应用结构
type App struct {
	ctx      context.Context
	sessions map[uint64]*SshSession
	nextID   uint64
	mu       sync.Mutex
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{sessions: make(map[uint64]*SshSession)}
}

// startup 保存上下文，供事件发射使用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SshConnect 建立 SSH 连接并打开远程 PTY shell，返回会话 ID
func (a *App) SshConnect(cfg SshConfig) (uint64, error) {
	if cfg.Host == "" || cfg.Username == "" {
		return 0, fmt.Errorf("host 和 username 不能为空")
	}

	var methods []ssh.AuthMethod
	if cfg.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		}
		if err != nil {
			return 0, fmt.Errorf("解析私钥失败: %v", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}
	if len(methods) == 0 {
		return 0, fmt.Errorf("请提供密码或私钥")
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // M1: 暂不做 known_hosts 校验
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	client, err := ssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return 0, fmt.Errorf("连接失败: %v", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return 0, fmt.Errorf("创建会话失败: %v", err)
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		client.Close()
		return 0, fmt.Errorf("申请 PTY 失败: %v", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		client.Close()
		return 0, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		client.Close()
		return 0, err
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		client.Close()
		return 0, err
	}
	if err := session.Shell(); err != nil {
		client.Close()
		return 0, fmt.Errorf("启动 shell 失败: %v", err)
	}

	a.mu.Lock()
	a.nextID++
	id := a.nextID
	a.sessions[id] = &SshSession{client: client, session: session, stdin: stdin}
	a.mu.Unlock()

	go a.pumpOutput(id, stdout)
	go a.pumpOutput(id, stderr)
	go func() {
		err := session.Wait()
		msg := ""
		if err != nil && err != io.EOF {
			msg = err.Error()
		}
		runtime.EventsEmit(a.ctx, "ssh-exit", SshExit{SessionID: id, Error: msg})
		a.mu.Lock()
		if ss, ok := a.sessions[id]; ok {
			ss.mu.Lock()
			ss.closed = true
			ss.mu.Unlock()
		}
		a.mu.Unlock()
	}()
	return id, nil
}

// SshSend 向前端输入通道写入数据
func (a *App) SshSend(id uint64, data string) error {
	a.mu.Lock()
	ss, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话 %d 不存在", id)
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.closed {
		return fmt.Errorf("会话 %d 已关闭", id)
	}
	_, err := ss.stdin.Write([]byte(data))
	return err
}

// SshResize 调整远程 PTY 尺寸
func (a *App) SshResize(id uint64, rows, cols int) error {
	a.mu.Lock()
	ss, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("会话 %d 不存在", id)
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.closed {
		return fmt.Errorf("会话 %d 已关闭", id)
	}
	return ss.session.WindowChange(rows, cols)
}

// SshClose 关闭会话
func (a *App) SshClose(id uint64) {
	a.mu.Lock()
	ss, ok := a.sessions[id]
	delete(a.sessions, id)
	a.mu.Unlock()
	if ok {
		ss.mu.Lock()
		ss.closed = true
		ss.mu.Unlock()
		ss.session.Close()
		ss.client.Close()
	}
}

// pumpOutput 读取 stdout/stderr 并通过事件发射给前端
func (a *App) pumpOutput(id uint64, r io.Reader) {
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			runtime.EventsEmit(a.ctx, "ssh-output", SshOutput{
				SessionID: id,
				Data:      string(buf[:n]),
			})
		}
		if err != nil {
			return
		}
	}
}
