package model

// OutputMsg 终端输出消息
type OutputMsg struct {
	Data string
}

// TermSession 终端会话统一接口 (SSH / Telnet 等协议实现)
type TermSession interface {
	Send(data string) error
	Resize(rows, cols int) error
	Close()
	Output() <-chan OutputMsg
	Done() <-chan error
}
