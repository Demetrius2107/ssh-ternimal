package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"ssh-terminal/internal/sshcore"
)

// startTunnelLocal 本地转发 -L: 本地端口 → 目标 (经 SSH 通道)
func startTunnelLocal(sess *sshcore.Session, listenAddr, targetAddr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				remote, err := sess.Dial(targetAddr)
				if err != nil {
					return
				}
				defer remote.Close()
				pipe(c, remote)
			}(conn)
		}
	}()
	return ln, nil
}

// startTunnelDynamic 动态转发 -D: 本地 SOCKS5 代理
func startTunnelDynamic(sess *sshcore.Session, listenAddr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleSocks5(conn, sess)
		}
	}()
	return ln, nil
}

// handleSocks5 最小 SOCKS5 代理 (仅 CONNECT, 无认证)
func handleSocks5(c net.Conn, sess *sshcore.Session) {
	defer c.Close()
	buf := make([]byte, 262)
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return
	}
	nmethods := int(buf[1])
	if _, err := io.ReadFull(c, buf[:nmethods]); err != nil {
		return
	}
	if _, err := c.Write([]byte{0x05, 0x00}); err != nil { // 无认证
		return
	}
	if _, err := io.ReadFull(c, buf[:4]); err != nil {
		return
	}
	if buf[1] != 0x01 { // 仅支持 CONNECT
		_, _ = c.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	var host string
	switch buf[3] {
	case 0x01: // IPv4
		if _, err := io.ReadFull(c, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case 0x03: // 域名
		if _, err := io.ReadFull(c, buf[:1]); err != nil {
			return
		}
		l := int(buf[0])
		if l == 0 || l > 255 {
			return
		}
		if _, err := io.ReadFull(c, buf[:l]); err != nil {
			return
		}
		host = string(buf[:l])
	case 0x04: // IPv6
		if _, err := io.ReadFull(c, buf[:16]); err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		return
	}
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[:2])
	remote, err := sess.Dial(net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		_, _ = c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()
	_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	pipe(c, remote)
}

// startTunnelRemote 远程转发 -R: 远程端口 → 本地目标
func startTunnelRemote(sess *sshcore.Session, remoteAddr, targetAddr string) (net.Listener, error) {
	ln, err := sess.ListenRemote(remoteAddr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				target, err := net.Dial("tcp", targetAddr)
				if err != nil {
					return
				}
				defer target.Close()
				pipe(c, target)
			}(conn)
		}
	}()
	return ln, nil
}

// pipe 双向拷贝, 任一端关闭即结束
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(b, a); done <- struct{}{} }()
	go func() { _, _ = io.Copy(a, b); done <- struct{}{} }()
	<-done
}
