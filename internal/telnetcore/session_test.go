package telnetcore

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

// fakeConn 丢弃写入的连接, 供测试 Session 的 negotiate 写响应时使用
type fakeConn struct{}

func (fakeConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (fakeConn) Write(p []byte) (int, error)      { return len(p), nil }
func (fakeConn) Close() error                     { return nil }
func (fakeConn) LocalAddr() net.Addr              { return nil }
func (fakeConn) RemoteAddr() net.Addr             { return nil }
func (fakeConn) SetDeadline(time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(time.Time) error { return nil }

// newTestSession 返回带假连接的 Session (process 内 negotiate 会写响应字节)
func newTestSession() *Session {
	return &Session{conn: fakeConn{}}
}

// TestProcessPlainText 普通文本原样透传
func TestProcessPlainText(t *testing.T) {
	s := newTestSession()
	data, rest := s.process([]byte("hello world\r\n"))
	if rest != nil {
		t.Fatalf("rest 应为 nil, got %v", rest)
	}
	if string(data) != "hello world\r\n" {
		t.Fatalf("透传不符: %q", data)
	}
}

// TestProcessIACLiteral 双 0xFF (IAC IAC) 折叠为单字节 0xFF
func TestProcessIACLiteral(t *testing.T) {
	s := newTestSession()
	data, _ := s.process([]byte{iac, iac, 'a', 'b'})
	want := []byte{0xFF, 'a', 'b'}
	if !bytes.Equal(data, want) {
		t.Fatalf("IAC 字面量处理不符: % x, want % x", data, want)
	}
}

// TestProcessNegotiation 选项协商被消费且不进入输出
func TestProcessNegotiation(t *testing.T) {
	s := newTestSession()
	// WILL ECHO (0xFF 0xFB 0x01) 后跟文本
	data, rest := s.process([]byte{iac, will, optEcho, 'x'})
	if rest != nil {
		t.Fatalf("rest 应为 nil, got %v", rest)
	}
	if string(data) != "x" {
		t.Fatalf("协商应被消费, 仅剩文本: %q", data)
	}
}

// TestProcessUnsupportedOptions 不支持的选项被拒绝 (WONT 响应)
func TestProcessUnsupportedOptions(t *testing.T) {
	s := newTestSession()
	data, _ := s.process([]byte{iac, will, 200, 'y'})
	if string(data) != "y" {
		t.Fatalf("不支持的选项应被消费: %q", data)
	}
}

// TestProcessSBSubnegotiation 子协商 (SB ... SE) 整体消费
func TestProcessSBSubnegotiation(t *testing.T) {
	s := newTestSession()
	// SB NAWS (0xFF 0xFA 0x1F 0x00 0x28 0x00 0x78 0xFF 0xF0) 后跟 'z'
	data, rest := s.process([]byte{iac, sb, optNAWS, 0x00, 0x28, 0x00, 0x78, iac, se, 'z'})
	if rest != nil {
		t.Fatalf("rest 应为 nil, got %v", rest)
	}
	if string(data) != "z" {
		t.Fatalf("子协商应被消费, 仅剩文本: %q", data)
	}
}

// TestProcessIncompleteTail IAC 序列跨包: 尾部不完整应原样返回等下一包
func TestProcessIncompleteTail(t *testing.T) {
	s := newTestSession()
	// 只剩 IAC + WILL, 缺选项字节
	data, rest := s.process([]byte{'a', iac, will})
	if string(data) != "a" {
		t.Fatalf("已完整部分应输出 'a': %q", data)
	}
	if rest == nil || len(rest) != 2 || rest[0] != iac {
		t.Fatalf("未消费尾部应为 [IAC WILL], got % x", rest)
	}
}

// TestProcessNopCommands NOP/GA 等单字节命令被丢弃
func TestProcessNopCommands(t *testing.T) {
	s := newTestSession()
	data, _ := s.process([]byte{iac, 241 /* NOP */, 'q'})
	if string(data) != "q" {
		t.Fatalf("NOP 应被丢弃: %q", data)
	}
}

// TestNegotiateEcho 服务器 WILL ECHO → 回复 DO ECHO (fakeConn 丢弃写入, 不 panic)
func TestNegotiateEcho(t *testing.T) {
	s := newTestSession()
	s.negotiate(will, optEcho)
}

// TestSendAndClose 关闭后 Send 报错
func TestSendAndClose(t *testing.T) {
	s := &Session{closed: true}
	if err := s.Send("x"); err == nil {
		t.Fatal("关闭后 Send 应报错")
	}
}
