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
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"ssh-terminal/internal/enc"
	"ssh-terminal/internal/model"
)

// Session 单个 SSH 会话
type Session struct {
	client      *ssh.Client
	session     *ssh.Session
	stdin       io.WriteCloser
	sftpClient  *sftp.Client
	sftpMu      sync.Mutex
	mu          sync.Mutex
	closed      bool
	output      chan model.OutputMsg
	done        chan error
	stop        chan struct{} // keepalive 停止信号
	closeOnce   sync.Once
	rxBytes     atomic.Int64
	txBytes     atomic.Int64
	keepAliveMs atomic.Int64
	jumpClient  *ssh.Client // 跳板机连接 (需保持存活, 会话关闭时一并关闭)
	encoding    string      // 输出编码: auto / utf-8 / gbk (空=auto)
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
		var uke *UnknownHostKeyError
		if errors.As(err, &uke) {
			return nil, err // 主机密钥未验证, 交上层处理, 不重试
		}
		logConnect("retry", fmt.Sprintf("attempt %d failed: %v", attempt, err))
		if attempt < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil, lastErr
}

// buildAuthMethods 构建认证方法 (密码/私钥/keyboard-interactive 含 OTP 应答)
func buildAuthMethods(user, password, privateKey, privateKeyPath, passphrase, otp string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if privateKeyPath != "" {
		pemBytes, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败: %v", err)
		}
		privateKey = string(pemBytes)
	}
	if privateKey != "" {
		var signer ssh.Signer
		var err error
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(privateKey), []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(privateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %v", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if password != "" || otp != "" {
		methods = append(methods, ssh.Password(password))
		// 部分服务器 (含堡垒机) 只提供 keyboard-interactive; OTP/验证码类问题用 OTP 应答
		methods = append(methods, ssh.KeyboardInteractive(func(u, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i, q := range questions {
				low := strings.ToLower(q)
				if otp != "" && (strings.Contains(low, "code") || strings.Contains(low, "otp") ||
					strings.Contains(low, "token") || strings.Contains(low, "验证")) {
					answers[i] = otp
				} else {
					answers[i] = password
				}
			}
			return answers, nil
		}))
	}
	if len(methods) == 0 {
		return nil, errors.New("请提供密码或私钥")
	}
	return methods, nil
}

// dialClient 拨号 + 握手 + 认证 (带 30s 全程 deadline), 返回客户端与底层连接
func dialClient(addr string, cfg *ssh.ClientConfig) (*ssh.Client, net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return nil, nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return ssh.NewClient(sshConn, chans, reqs), conn, nil
}

// connectOnce 单次连接: 拨号、握手、认证、PTY、shell
func connectOnce(cfg model.SshConfig) (*Session, error) {
	if cfg.Host == "" || cfg.Username == "" {
		return nil, errors.New("host 和 username 不能为空")
	}

	// 目标端认证 (密码/私钥/OTP)
	methods, err := buildAuthMethods(cfg.Username, cfg.Password, cfg.PrivateKey, cfg.PrivateKeyPath, cfg.Passphrase, cfg.OTP)
	if err != nil {
		return nil, err
	}
	hostKeyCb, err := HostKeyCallback(cfg.HostKeyMode)
	if err != nil {
		return nil, err
	}
	clientCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            methods,
		HostKeyCallback: hostKeyCb,
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	t0 := time.Now()

	var client *ssh.Client
	var conn net.Conn
	var jumpClient *ssh.Client
	if cfg.JumpHost != "" {
		// 跳板机: 先连跳板, 再经 direct-tcpip 通道连目标
		jumpMethods, jerr := buildAuthMethods(cfg.JumpUser, cfg.JumpPassword, "", cfg.JumpPrivateKeyPath, cfg.JumpPassphrase, "")
		if jerr != nil {
			return nil, jerr
		}
		jumpCfg := &ssh.ClientConfig{
			User:            cfg.JumpUser,
			Auth:            jumpMethods,
			HostKeyCallback: hostKeyCb,
			Timeout:         15 * time.Second,
		}
		jumpAddr := net.JoinHostPort(cfg.JumpHost, strconv.Itoa(cfg.JumpPort))
		var jconn net.Conn
		jumpClient, jconn, jerr = dialClient(jumpAddr, jumpCfg)
		if jerr != nil {
			logConnect("jump-fail", jerr.Error())
			return nil, fmt.Errorf("跳板机连接失败: %v", jerr)
		}
		_ = jconn.SetDeadline(time.Time{}) // 跳板握手完成, 清除 deadline
		conn, jerr = jumpClient.Dial("tcp", addr)
		if jerr != nil {
			jumpClient.Close()
			return nil, fmt.Errorf("经跳板连接目标失败: %v", jerr)
		}
		sshConn, chans, reqs, herr := ssh.NewClientConn(conn, addr, clientCfg)
		if herr != nil {
			jumpClient.Close()
			conn.Close()
			logConnect("handshake+auth-fail", herr.Error())
			var uke *UnknownHostKeyError
			if errors.As(herr, &uke) {
				return nil, uke
			}
			return nil, fmt.Errorf("经跳板握手失败: %v", herr)
		}
		client = ssh.NewClient(sshConn, chans, reqs)
	} else {
		// 直连: 手动拨号 + 全程 deadline (握手/认证/PTY/Shell 统一 30s)
		conn, err = net.DialTimeout("tcp", addr, 8*time.Second)
		if err != nil {
			logConnect("dial-fail", err.Error())
			return nil, fmt.Errorf("连接失败: %v", err)
		}
		logConnect("dial", time.Since(t0).String())
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		sshConn, chans, reqs, herr := ssh.NewClientConn(conn, addr, clientCfg)
		if herr != nil {
			conn.Close()
			logConnect("handshake+auth-fail", herr.Error())
			var uke *UnknownHostKeyError
			if errors.As(herr, &uke) {
				return nil, uke
			}
			if isAuthError(herr) {
				return nil, fmt.Errorf("认证失败: %v", herr)
			}
			return nil, fmt.Errorf("连接失败(握手/超时): %v", herr)
		}
		logConnect("handshake+auth", time.Since(t0).String())
		client = ssh.NewClient(sshConn, chans, reqs)
	}

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
		client:   client,
		session:  sess,
		stdin:    stdin,
		encoding: cfg.Encoding,
		output:   make(chan model.OutputMsg, 64),
		done:     make(chan error, 1),
		stop:     make(chan struct{}),
	}
	if jumpClient != nil {
		s.jumpClient = jumpClient
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
			t0 := time.Now()
			if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
			s.keepAliveMs.Store(time.Since(t0).Milliseconds())
		}
	}
}

// pump 读取输出, 按会话编码转码为 UTF-8 后投递到 channel
func (s *Session) pump(r io.Reader) {
	conv := enc.NewConverter(s.encoding)
	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.rxBytes.Add(int64(n))
			s.output <- model.OutputMsg{Data: conv.Decode(buf[:n])}
		}
		if err != nil {
			return
		}
	}
}

// Output 返回终端输出流 (只读)
func (s *Session) Output() <-chan model.OutputMsg { return s.output }

// Done 返回会话结束信号 (阻塞到退出)
func (s *Session) Done() <-chan error { return s.done }

// KeepAlive 手动发送保活请求, 返回 RTT 毫秒
func (s *Session) KeepAlive() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("会话已关闭")
	}
	t0 := time.Now()
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		return 0, err
	}
	ms := time.Since(t0).Milliseconds()
	s.keepAliveMs.Store(ms)
	return ms, nil
}

// Metrics 返回会话实时指标
func (s *Session) Metrics() model.Metrics {
	return model.Metrics{
		BytesIn:     s.rxBytes.Load(),
		BytesOut:    s.txBytes.Load(),
		KeepAliveMs: s.keepAliveMs.Load(),
	}
}

// Dial 打开到目标的 direct-tcpip 通道 (本地/动态转发用)
func (s *Session) Dial(addr string) (net.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("会话已关闭")
	}
	return s.client.Dial("tcp", addr)
}

// ListenRemote 请求远程端口转发 (远程转发 -R 用)
func (s *Session) ListenRemote(addr string) (net.Listener, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("会话已关闭")
	}
	return s.client.Listen("tcp", addr)
}

// Exec 在远程执行命令并返回输出 (资源监控等一次性命令用)
func (s *Session) Exec(cmd string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("会话已关闭")
	}
	sess, err := s.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(cmd)
	return string(out), err
}

// Send 发送终端输入
func (s *Session) Send(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("会话已关闭")
	}
	n, err := s.stdin.Write([]byte(data))
	if n > 0 {
		s.txBytes.Add(int64(n))
	}
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
		if s.jumpClient != nil {
			s.jumpClient.Close()
		}
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

// FetchFile 下载远程单个文件内容 (远程编辑用)
func (s *Session) FetchFile(remotePath string) ([]byte, error) {
	c, err := s.SFTP()
	if err != nil {
		return nil, err
	}
	remote, err := c.Open(remotePath)
	if err != nil {
		return nil, err
	}
	defer remote.Close()
	return io.ReadAll(remote)
}

// PutFile 上传内容覆盖远程文件 (远程编辑回传用)
func (s *Session) PutFile(remotePath string, data []byte) error {
	c, err := s.SFTP()
	if err != nil {
		return err
	}
	remote, err := c.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer remote.Close()
	_, err = remote.Write(data)
	return err
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
