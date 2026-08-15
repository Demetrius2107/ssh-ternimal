// Package enc 终端字节流编码转换: 支持 UTF-8 / GBK 自动探测与强制模式
//
// Converter 是有状态的流式转换器: 每个会话输出流 (SSH stdout/stderr、Telnet 数据流)
// 一个实例, 跨 chunk 保留未完成的多字节序列, 避免分块边界截断汉字。
//
// 探测策略 (auto): chunk 整体是合法 UTF-8 (含纯 ASCII) 则原样输出;
// 出现非法字节序列则锁定 GBK, 之后整条流按 GBK 解码。
package enc

import (
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// 编码模式值 (对应 model.SshConfig.Encoding)
const (
	ModeAuto = "auto" // 默认: 合法 UTF-8 原样, 非法字节出现后锁定 GBK
	ModeUTF8 = "utf-8"
	ModeGBK  = "gbk"
)

// Converter 流式编码转换器 (每会话输出流一个实例)
type Converter struct {
	mode     string // 当前生效模式 (auto 探测后会锁定为 gbk)
	leftover []byte // 跨 chunk 未完成的多字节序列 (UTF-8 前缀 或 GBK 孤立前导字节)
}

// NewConverter 创建转换器, 未知模式回退 auto
func NewConverter(mode string) *Converter {
	switch mode {
	case ModeUTF8, ModeGBK:
	default:
		mode = ModeAuto
	}
	return &Converter{mode: mode}
}

// Decode 将一段原始输出字节转换为 UTF-8 字符串, 跨 chunk 保持转换状态
func (c *Converter) Decode(p []byte) string {
	if len(p) == 0 {
		return ""
	}
	data := append(c.leftover, p...)
	c.leftover = nil

	switch c.mode {
	case ModeGBK:
		return c.decodeGBK(data)
	case ModeUTF8:
		return c.emitUTF8(data)
	default:
		return c.decodeAuto(data)
	}
}

// decodeAuto 自动探测: 合法 UTF-8 (含纯 ASCII) 原样输出; 尾部仅是不完整 UTF-8
// 序列则保留到下一包; 否则判定为 GBK 并锁定整条流
func (c *Converter) decodeAuto(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	if n := c.holdUTF8Tail(data); n >= 0 {
		return string(data[:n])
	}
	c.mode = ModeGBK
	return c.decodeGBK(data)
}

// emitUTF8 强制 UTF-8: 原样输出, 尾部不完整序列保留到下一包
func (c *Converter) emitUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	if n := c.holdUTF8Tail(data); n >= 0 {
		return string(data[:n])
	}
	return string(data) // 无法恢复的非法字节原样 (JSON 序列化时替换为 U+FFFD)
}

// holdUTF8Tail 若 data 只有尾部 (长度 <4) 不完整, 保留尾部到 leftover 并返回
// 合法前缀长度; 若 data 整体合法或无法判定为"仅尾部不完整"则返回 -1
func (c *Converter) holdUTF8Tail(data []byte) int {
	i := len(data)
	for i > 0 && !utf8.Valid(data[:i]) {
		i--
	}
	tail := data[i:]
	if len(tail) > 0 && len(tail) < 4 && isUTF8Prefix(tail) {
		c.leftover = append(c.leftover, tail...)
		return i
	}
	return -1
}

// isUTF8Prefix 判断 b 是否为某个合法 UTF-8 字符的不完整前缀
// (首字节为引导字节, 其余均为续字节, 且总长小于该字符应有的编码长度)
func isUTF8Prefix(b []byte) bool {
	if len(b) == 0 || len(b) > 3 {
		return false
	}
	first := b[0]
	var want int
	switch {
	case first >= 0xC2 && first <= 0xDF:
		want = 2
	case first >= 0xE0 && first <= 0xEF:
		want = 3
	case first >= 0xF0 && first <= 0xF4:
		want = 4
	default:
		return false
	}
	if len(b) >= want {
		return false // 已完整, 不是前缀
	}
	for _, x := range b[1:] {
		if x < 0x80 || x > 0xBF {
			return false
		}
	}
	return true
}

// decodeGBK 将 GBK 字节流转为 UTF-8; 尾部孤立的 GBK 前导字节 (0x81-0xFE)
// 留到下一包拼接, 其余按完整序列一次性解码
func (c *Converter) decodeGBK(data []byte) string {
	complete, tail := splitGBKTail(data)
	if len(tail) > 0 {
		c.leftover = append(c.leftover, tail...)
	}
	out, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), complete)
	if err != nil {
		return string(complete) // 兜底: 解码失败原样返回
	}
	return string(out)
}

// splitGBKTail 扫描 GBK 字节流, 返回 (完整前缀, 尾部孤立前导字节)
// GBK 结构: 单字节 ASCII (0x00-0x7F) 或 双字节 (前导 0x81-0xFE + 尾随 0x40-0xFE)
func splitGBKTail(data []byte) (complete, tail []byte) {
	i, n := 0, len(data)
	for i < n {
		b := data[i]
		if b >= 0x81 && b <= 0xFE {
			if i+1 >= n {
				return data[:i], data[i:] // 最后一个字节是孤立前导
			}
			i += 2
		} else {
			i++ // ASCII 或非法单字节 (0x80/0xFF 由解码器替换)
		}
	}
	return data, nil
}
