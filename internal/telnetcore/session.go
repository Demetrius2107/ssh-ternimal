// Package telnetcore 封装 Telnet 连接与会话 (纯 Go, 不依赖 wails)
package telnetcore

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ssh-terminal/internal/model"
)

// Telnet IAC 命令
const (
	iac = 255 // IAC 开始符
	dont = 254
	do   = 253
	wont = 252
	will = 251
	sb   = 250 // 子协商开始
	se   = 240 // 子协商结束

	// 常用选项
	optEcho = 1  // ECHO
	optSGA  = 3  // SUPPRESS GO AHEAD
	optNAWS = 31 // 窗口大小
)

// Session Telnet 会话
type Session struct {
	conn      net.Conn
	output    chan model.OutputMsg
	done      chan error
	mu        sync.Mutex
	closed    bool
	closeOnce sync.Once
	rows      int
	cols      int
	rxBytes   atomic.Int64
	txBytes   atomic.Int64
}

// Connect 建立 Telnet 连接
func Connect(host string, port int) (*Session, error) {
	if host == "" {
		return nil, errors.New("host 不能为空")
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	s := &Session{
		conn:   conn,
		output: make(chan model.OutputMsg, 64),
		done:   make(chan error, 1),
		rows:   40,
		cols:   120,
	}
	go s.pump()
	return s, nil
}

// pump 读取数据, 处理 IAC 协商, 其余数据投递到输出流
func (s *Session) pump() {
	buf := make([]byte, 8192)
	var leftover []byte
	for {
		n, err := s.conn.Read(buf)
		if n > 0 {
			s.rxBytes.Add(int64(n))
			chunk := append(leftover, buf[:n]...)
			leftover = nil
			data, rest := s.process(chunk)
			if rest != nil {
				leftover = rest // IAC 序列跨包, 与下一包拼接
			}
			if len(data) > 0 {
				s.output <- model.OutputMsg{Data: string(data)}
			}
		}
		if err != nil {
			select {
			case s.done <- err:
			default:
			}
			s.Close()
			return
		}
	}
}

// process 处理原始字节流, 返回 (可显示数据, 未消费尾部)
// 未消费尾部: 以 IAC 开头但序列不完整的剩余字节
func (s *Session) process(in []byte) ([]byte, []byte) {
	out := make([]byte, 0, len(in))
	i := 0
	for i < len(in) {
		b := in[i]
		if b != iac {
			out = append(out, b)
			i++
			continue
		}
		if i+1 >= len(in) {
			return out, in[i:] // 只剩 IAC, 等下一包
		}
		cmd := in[i+1]
		switch cmd {
		case iac: // 字面 0xFF
			out = append(out, iac)
			i += 2
		case will, wont, do, dont:
			if i+2 >= len(in) {
				return out, in[i:]
			}
			s.negotiate(cmd, in[i+2])
			i += 3
		case sb:
			end := -1
			for j := i + 2; j < len(in)-1; j++ {
				if in[j] == iac && in[j+1] == se {
					end = j
					break
				}
			}
			if end < 0 {
				return out, in[i:]
			}
			i = end + 2
		default: // NOP/IP/AO/AYT/EC/EL/GA 等, 直接丢弃
			i += 2
		}
	}
	return out, nil
}

// negotiate 选项协商: 接受 ECHO/SGA/NAWS, 拒绝其余
func (s *Session) negotiate(cmd, opt byte) {
	switch cmd {
	case will: // 服务器要启用选项
		switch opt {
		case optEcho, optSGA:
			s.writeCmd(do, opt) // 同意, 由服务器回显
		default:
			s.writeCmd(dont, opt)
		}
	case do: // 服务器要求我们启用选项
		switch opt {
		case optNAWS:
			s.writeCmd(will, opt)
			s.mu.Lock()
			s.sendNAWSLocked()
			s.mu.Unlock()
		default:
			s.writeCmd(wont, opt)
		}
	}
	// wont/dont: 忽略
}

func (s *Session) writeCmd(cmd, opt byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	_, _ = s.conn.Write([]byte{iac, cmd, opt})
}

// sendNAWSLocked 发送窗口大小子协商 (调用方需持锁)
func (s *Session) sendNAWSLocked() {
	if s.closed {
		return
	}
	hiR, loR := byte(s.rows>>8), byte(s.rows)
	hiC, loC := byte(s.cols>>8), byte(s.cols)
	_, _ = s.conn.Write([]byte{iac, sb, optNAWS, hiR, loR, hiC, loC, iac, se})
}

// Send 发送输入 (原始字节透传)
func (s *Session) Send(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("会话已关闭")
	}
	n, err := s.conn.Write([]byte(data))
	if n > 0 {
		s.txBytes.Add(int64(n))
	}
	return err
}

// Resize 记录尺寸并通过 NAWS 通知服务器 (需服务器支持)
func (s *Session) Resize(rows, cols int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("会话已关闭")
	}
	if s.rows == rows && s.cols == cols {
		return nil
	}
	s.rows, s.cols = rows, cols
	s.sendNAWSLocked()
	return nil
}

// Output 返回终端输出流 (只读)
func (s *Session) Output() <-chan model.OutputMsg { return s.output }

// Done 返回会话结束信号
func (s *Session) Done() <-chan error { return s.done }

// KeepAlive Telnet 无保活机制
func (s *Session) KeepAlive() (int64, error) {
	return 0, errors.New("Telnet 不支持保活")
}

// Metrics 返回会话实时指标
func (s *Session) Metrics() model.Metrics {
	return model.Metrics{
		BytesIn:  s.rxBytes.Load(),
		BytesOut: s.txBytes.Load(),
	}
}

// Close 关闭连接 (幂等)
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		_ = s.conn.Close()
	})
}
