// Package sshcore 封装 SSH 连接与会话 (纯 Go, 不依赖 wails)
package sshcore

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ssh-terminal/internal/model"
)

// OutputMsg 终端输出消息
type OutputMsg struct {
	Data string
}

// Session 单个 SSH 会话
type Session struct {
	client     *ssh.Client
	session    *ssh.Session
	stdin      io.WriteCloser
	sftpClient *sftp.Client
	sftpMu     sync.Mutex
	mu         sync.Mutex
	closed     bool
	output     chan OutputMsg
	done       chan error
	stop       chan struct{} // keepalive 停止信号
	closeOnce  sync.Once
}

// Connect 建立连接、认证并打开远程 PTY shell; 对瞬时错误自动重试一次
func Connect(cfg model.SshConfig) (*Session, error) {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		s, err := connectOnce(cfg)
		if err == nil {
			return s, nil
		}
		lastErr = err
		if isAuthError(err) {
			return nil, err // 认证失败 (密码/私钥错误) 不重试
		}
		logConnect("retry", fmt.Sprintf("attempt %d failed: %v", attempt, err))
		if attempt < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil, lastErr
}

// connectOnce 单次连接: 拨号、握手、认证、PTY、shell
func connectOnce(cfg model.SshConfig) (*Session, error) {
	if cfg.Host == "" || cfg.Username == "" {
		return nil, errors.New("host 和 username 不能为空")
	}

	var methods []ssh.AuthMethod
	if cfg.PrivateKeyPath != "" {
		pemBytes, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败: %v", err)
		}
		cfg.PrivateKey = string(pemBytes)
	}
	if cfg.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if cfg.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(cfg.PrivateKey), []byte(cfg.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
		// 部分服务器 (含堡垒机) 只提供 keyboard-interactive, 用同一密码应答
		methods = append(methods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = cfg.Password
			}
			return answers, nil
		}))
	}
	if len(methods) == 0 {
		return nil, errors.New("请提供密码或私钥")
	}

	clientCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: known_hosts 校验
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	// 手动拨号 + 全程 deadline: ClientConfig.Timeout 只覆盖 TCP 拨号,
	// 握手/认证/PTY/Shell 无超时会无限卡顿 (服务器半开连接), 统一限制 30s
	t0 := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		logConnect("dial-fail", err.Error())
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	logConnect("dial", time.Since(t0).String())
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, clientCfg)
	if err != nil {
		conn.Close()
		logConnect("handshake+auth-fail", err.Error())
		if isAuthError(err) {
			return nil, fmt.Errorf("认证失败: %v", err)
		}
		return nil, fmt.Errorf("连接失败(握手/超时): %v", err)
	}
	logConnect("handshake+auth", time.Since(t0).String())
	client := ssh.NewClient(sshConn, chans, reqs)

	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("创建会话失败: %v", err)
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		client.Close()
		return nil, fmt.Errorf("申请 PTY 失败: %v", err)
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		client.Close()
		return nil, err
	}
	if err := sess.Shell(); err != nil {
		client.Close()
		return nil, fmt.Errorf("启动 shell 失败: %v", err)
	}
	logConnect("pty+shell", time.Since(t0).String())
	// shell 已启动, 清除 deadline, 否则 30s 后终端会被强制切断
	_ = conn.SetDeadline(time.Time{})

	s := &Session{
		client:  client,
		session: sess,
		stdin:   stdin,
		output:  make(chan OutputMsg, 64),
		done:    make(chan error, 1),
		stop:    make(chan struct{}),
	}
	go s.pump(stdout)
	go s.pump(stderr)
	go s.keepAlive()
	go func() {
		err := sess.Wait()
		if err == io.EOF {
			err = nil
		}
		s.done <- err
		s.Close()
	}()
	logConnect("total", time.Since(t0).String())
	return s, nil
}

// isAuthError 判断是否为认证失败 (密码/私钥错误), 此类错误不重试
func isAuthError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "unable to authenticate") || strings.Contains(s, "认证失败")
}

// logConnect 连接阶段计时/事件写入 %TEMP%/ssh-terminal-connect.log, 用于定位慢连接与偶发失败
func logConnect(stage, detail string) {
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "ssh-terminal-connect.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), stage, detail)
}

// keepAlive 每 30s 发送保活请求, 会话停止时退出
func (s *Session) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// pump 读取输出并投递到 channel
func (s *Session) pump(r io.Reader) {
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.output <- OutputMsg{Data: string(buf[:n])}
		}
		if err != nil {
			return
		}
	}
}

// Output 返回终端输出流 (只读)
func (s *Session) Output() <-chan OutputMsg { return s.output }

// Done 返回会话结束信号 (阻塞到退出)
func (s *Session) Done() <-chan error { return s.done }

// Send 发送终端输入
func (s *Session) Send(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("会话已关闭")
	}
	_, err := s.stdin.Write([]byte(data))
	return err
}

// Resize 调整远程 PTY 尺寸
func (s *Session) Resize(rows, cols int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("会话已关闭")
	}
	return s.session.WindowChange(rows, cols)
}

// Close 关闭会话 (幂等)
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.sftpMu.Lock()
		if s.sftpClient != nil {
			s.sftpClient.Close()
		}
		s.sftpMu.Unlock()
		s.session.Close()
		s.client.Close()
	})
}

// SFTP 懒加载返回 sftp 客户端 (同一连接复用)
func (s *Session) SFTP() (*sftp.Client, error) {
	s.sftpMu.Lock()
	defer s.sftpMu.Unlock()
	if s.closed {
		return nil, errors.New("会话已关闭")
	}
	if s.sftpClient == nil {
		c, err := sftp.NewClient(s.client)
		if err != nil {
			return nil, fmt.Errorf("初始化 SFTP 失败: %v", err)
		}
		s.sftpClient = c
	}
	return s.sftpClient, nil
}

// ---------- 远程文件操作 ----------

// Pwd 返回远程当前工作目录
func (s *Session) Pwd() (string, error) {
	c, err := s.SFTP()
	if err != nil {
		return "", err
	}
	return c.Getwd()
}

// ListDir 列出远程目录, 目录在前
func (s *Session) ListDir(dir string) ([]model.FileEntry, error) {
	c, err := s.SFTP()
	if err != nil {
		return nil, err
	}
	if dir == "" {
		dir = "."
	}
	infos, err := c.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]model.FileEntry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries, model.FileEntry{
			Name:    fi.Name(),
			Path:    path.Join(dir, fi.Name()),
			IsDir:   fi.IsDir(),
			Size:    fi.Size(),
			ModTime: fi.ModTime().Format("2006-01-02 15:04"),
			Mode:    fi.Mode().String(),
		})
	}
	sortEntries(entries)
	return entries, nil
}

// Mkdir 远程新建目录
func (s *Session) Mkdir(dir string) error {
	c, err := s.SFTP()
	if err != nil {
		return err
	}
	return c.Mkdir(dir)
}

// Delete 远程删除文件或目录 (目录递归)
func (s *Session) Delete(p string) error {
	c, err := s.SFTP()
	if err != nil {
		return err
	}
	fi, err := c.Stat(p)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return c.RemoveAll(p)
	}
	return c.Remove(p)
}

// Rename 远程重命名
func (s *Session) Rename(oldP, newP string) error {
	c, err := s.SFTP()
	if err != nil {
		return err
	}
	return c.Rename(oldP, newP)
}

// Chmod 远程修改权限
func (s *Session) Chmod(p string, mode uint32) error {
	c, err := s.SFTP()
	if err != nil {
		return err
	}
	return c.Chmod(p, os.FileMode(mode))
}

// sortEntries 目录在前, 名称字典序 (不区分大小写)
func sortEntries(entries []model.FileEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}
