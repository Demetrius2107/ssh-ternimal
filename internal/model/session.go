package model

// OutputMsg 终端输出消息
type OutputMsg struct {
	Data string
}

// Metrics 会话实时指标 (状态栏轮询展示)
type Metrics struct {
	BytesIn     int64 `json:"bytesIn"`
	BytesOut    int64 `json:"bytesOut"`
	KeepAliveMs int64 `json:"keepAliveMs"` // 最近一次保活 RTT (毫秒), 0=未知
}

// TermSession 终端会话统一接口 (SSH / Telnet 等协议实现)
type TermSession interface {
	Send(data string) error
	Resize(rows, cols int) error
	Close()
	Output() <-chan OutputMsg
	Done() <-chan error
	Metrics() Metrics
	KeepAlive() (int64, error)
}
