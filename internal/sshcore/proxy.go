// proxy SSH 代理拨号: HTTP CONNECT / SOCKS5, 纯标准库实现
package sshcore

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// proxy 代理配置 (从 model.SshConfig 提取)
type proxy struct {
	Type     string // "" / http / socks5
	Host     string
	Port     int
	User     string
	Password string
}

// enabled 是否配置了可用代理
func (p proxy) enabled() bool {
	return p.Type != "" && p.Host != "" && p.Port > 0
}

// dialTCP 拨号: 有代理则经代理建立连接, 否则直连
func dialTCP(addr string, p proxy) (net.Conn, error) {
	if p.enabled() {
		return p.dial(addr, 8*time.Second)
	}
	return net.DialTimeout("tcp", addr, 8*time.Second)
}

// dial 经代理连接目标地址 (host:port), 返回已建立的连接
func (p proxy) dial(addr string, timeout time.Duration) (net.Conn, error) {
	if !p.enabled() {
		return nil, errors.New("代理未配置")
	}
	proxyAddr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("连接代理 %s 失败: %v", proxyAddr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout + 10*time.Second))
	switch p.Type {
	case "http":
		err = p.httpConnect(conn, addr)
	case "socks5":
		err = p.socks5Handshake(conn, addr)
	default:
		err = fmt.Errorf("不支持的代理类型: %s", p.Type)
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{}) // 握手完成, 清除 deadline
	return conn, nil
}

// httpConnect HTTP CONNECT 隧道 (RFC 7231)
func (p proxy) httpConnect(conn net.Conn, addr string) error {
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if p.User != "" {
		token := base64.StdEncoding.EncodeToString([]byte(p.User + ":" + p.Password))
		req += "Proxy-Authorization: Basic " + token + "\r\n"
	}
	req += "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return fmt.Errorf("发送 CONNECT 失败: %v", err)
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取代理响应失败: %v", err)
	}
	// HTTP/1.1 200 Connection established
	status := 0
	if _, err := fmt.Sscanf(line, "HTTP/%*s %d", &status); err != nil || status != 200 {
		return fmt.Errorf("代理 CONNECT 被拒绝: %s", trimSpace(line))
	}
	// 丢弃剩余响应头 (至空行)
	for {
		l, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("读取代理响应头失败: %v", err)
		}
		if len(l) <= 2 { // 空行 (\r\n)
			break
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}

// socks5Handshake SOCKS5 握手 (RFC 1928): 协商 + 认证 + CONNECT
func (p proxy) socks5Handshake(conn net.Conn, addr string) error {
	// 1. 方法协商
	methods := []byte{0x00} // 无认证
	if p.User != "" {
		methods = []byte{0x00, 0x02} // 无认证 / 用户名密码
	}
	greet := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greet); err != nil {
		return fmt.Errorf("SOCKS5 协商失败: %v", err)
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("SOCKS5 协商响应失败: %v", err)
	}
	if resp[0] != 0x05 {
		return errors.New("SOCKS5 版本不受支持")
	}
	switch resp[1] {
	case 0x00: // 无认证
	case 0x02: // 用户名密码
		if err := p.socks5Auth(conn); err != nil {
			return err
		}
	default:
		return fmt.Errorf("SOCKS5 拒绝认证方式 (0x%02x)", resp[1])
	}

	// 2. CONNECT 请求: 目标地址 (域名优先, 回退 IP)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("目标地址无效: %s", addr)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("目标端口无效: %s", portStr)
	}
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return errors.New("目标域名过长")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("SOCKS5 CONNECT 失败: %v", err)
	}

	// 3. 响应: VER REP RSV ATYP BND.ADDR BND.PORT
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return fmt.Errorf("SOCKS5 响应失败: %v", err)
	}
	if head[0] != 0x05 {
		return errors.New("SOCKS5 响应版本错误")
	}
	if head[1] != 0x00 {
		return fmt.Errorf("SOCKS5 连接被拒绝 (代码 0x%02x)", head[1])
	}
	// 丢弃绑定地址
	switch head[3] {
	case 0x01:
		_, _ = io.CopyN(io.Discard, conn, 4+2)
	case 0x04:
		_, _ = io.CopyN(io.Discard, conn, 16+2)
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return err
		}
		_, _ = io.CopyN(io.Discard, conn, int64(l[0])+2)
	default:
		return errors.New("SOCKS5 响应地址类型无效")
	}
	return nil
}

// socks5Auth 用户名/密码认证 (RFC 1929)
func (p proxy) socks5Auth(conn net.Conn) error {
	u, pw := []byte(p.User), []byte(p.Password)
	if len(u) > 255 || len(pw) > 255 {
		return errors.New("SOCKS5 认证凭据过长")
	}
	msg := append([]byte{0x01, byte(len(u))}, u...)
	msg = append(msg, byte(len(pw)))
	msg = append(msg, pw...)
	if _, err := conn.Write(msg); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x01 || resp[1] != 0x00 {
		return errors.New("SOCKS5 认证失败 (用户名/密码错误)")
	}
	return nil
}

// trimSpace 去除行尾 CR/LF
func trimSpace(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\r' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
