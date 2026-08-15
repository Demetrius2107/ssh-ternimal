// knownhosts 主机密钥校验: known_hosts 文件管理 + 三种模式
//
// 模式 (SshConfig.HostKeyMode):
//   off         不校验 (等同旧行为 InsecureIgnoreHostKey)
//   accept-new  首次连接自动记录主机密钥, 密钥变更则拒绝 (默认)
//   strict      首次连接返回 UnknownHostKeyError, 由前端确认后 AcceptHostKey 记录
package sshcore

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsMu 保护 known_hosts 文件写入
var knownHostsMu sync.Mutex

// KnownHostsPath 返回 known_hosts 文件路径: %AppData%/ssh-terminal/known_hosts
func KnownHostsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取配置目录失败: %v", err)
	}
	return filepath.Join(dir, "ssh-terminal", "known_hosts"), nil
}

// ensureKnownHostsFile 确保文件存在 (knownhosts.New 要求文件已存在)
func ensureKnownHostsFile() (string, error) {
	path, err := KnownHostsPath()
	if err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	return path, f.Close()
}

// UnknownHostKeyError 未知主机密钥 (首次连接, 需用户确认)
type UnknownHostKeyError struct {
	Host        string
	Port        int
	Fingerprint string
	Key         ssh.PublicKey
}

func (e *UnknownHostKeyError) Error() string {
	return fmt.Sprintf("主机密钥未验证 (SHA256 指纹: %s)", e.Fingerprint)
}

// HostKeyCallback 构建主机密钥校验回调
func HostKeyCallback(mode string) (ssh.HostKeyCallback, error) {
	if mode == "off" {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	path, err := ensureKnownHostsFile()
	if err != nil {
		return nil, err
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("加载 known_hosts 失败: %v", err)
	}
	strict := mode == "strict"
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil {
			return nil // 已知主机且密钥匹配
		}
		var ke *knownhosts.KeyError
		if errors.As(err, &ke) && len(ke.Want) == 0 {
			// 未知主机 (首次连接)
			host, port := splitHostPort(hostname)
			if !strict {
				// accept-new: 自动记录并放行
				if aerr := appendKnownHostKey(host, port, key); aerr != nil {
					return aerr
				}
				return nil
			}
			return &UnknownHostKeyError{Host: host, Port: port, Fingerprint: ssh.FingerprintSHA256(key), Key: key}
		}
		// 已知主机但密钥已变更 (疑似中间人攻击)
		return fmt.Errorf("主机密钥已变更, 可能遭受中间人攻击!\n请检查并删除 %s 中对应条目后重试", path)
	}, nil
}

// splitHostPort 解析回调传入的 "host:port", 失败时回退默认端口 22
func splitHostPort(hostport string) (string, int) {
	host, portStr, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, 22
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return host, 22
	}
	return host, port
}

// AcceptHostKey 将主机密钥写入 known_hosts (strict 模式前端确认后调用)
func AcceptHostKey(host string, port int, key ssh.PublicKey) error {
	return appendKnownHostKey(host, port, key)
}

func appendKnownHostKey(host string, port int, key ssh.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	path, err := KnownHostsPath()
	if err != nil {
		return err
	}
	line := knownhosts.Line([]string{net.JoinHostPort(host, strconv.Itoa(port))}, key)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}
