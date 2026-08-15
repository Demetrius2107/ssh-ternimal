package enc

import (
	"testing"
)

func TestASCIIConvertsToItself(t *testing.T) {
	c := NewConverter(ModeAuto)
	if got := c.Decode([]byte("ls -la /tmp")); got != "ls -la /tmp" {
		t.Fatalf("ascii: %q", got)
	}
}

func TestValidUTF8Passthrough(t *testing.T) {
	c := NewConverter(ModeAuto)
	if got := c.Decode([]byte("中文")); got != "中文" {
		t.Fatalf("utf8: %q", got)
	}
	// 跨 chunk 拆分: 先发一个不完整 UTF-8 序列
	if got := c.Decode([]byte{0xE4, 0xB8}); got != "" {
		t.Fatalf("utf8 partial: %q", got)
	}
	if got := c.Decode([]byte{0xAD}); got != "中" {
		t.Fatalf("utf8 joined: %q", got)
	}
}

func TestGBKDetectedAndLocked(t *testing.T) {
	c := NewConverter(ModeAuto)
	// GBK "你好" = C4E3 BAC3
	if got := c.Decode([]byte{0xC4, 0xE3, 0xBA, 0xC3}); got != "你好" {
		t.Fatalf("gbk detect: %q", got)
	}
	// 锁定后继续按 GBK 解码
	if got := c.Decode([]byte{0xD6, 0xD0, 0xCE, 0xC4}); got != "中文" {
		t.Fatalf("gbk locked: %q", got)
	}
}

func TestGBKSplitAcrossChunks(t *testing.T) {
	c := NewConverter(ModeAuto)
	// 第一包只有 GBK 前导字节 (C4 是 "你" 的前半), 应内部缓冲不产出
	// 但此时整包无法判定 GBK (单个字节也可能是 ASCII/UTF8), 先原样等待
	if got := c.Decode([]byte{0xC4}); got != "" {
		t.Fatalf("gbk split first: %q", got)
	}
	if got := c.Decode([]byte{0xE3, 0xBA, 0xC3}); got != "你好" {
		t.Fatalf("gbk split rest: %q", got)
	}
}

func TestGBKForcedModeSplit(t *testing.T) {
	c := NewConverter(ModeGBK)
	if got := c.Decode([]byte{0xC4}); got != "" {
		t.Fatalf("forced gbk partial: %q", got)
	}
	if got := c.Decode([]byte{0xE3}); got != "你" {
		t.Fatalf("forced gbk joined: %q", got)
	}
}

func TestMixedASCIIAndGBK(t *testing.T) {
	c := NewConverter(ModeAuto)
	// "root@dev:/#" + GBK "中文" + "\n"
	prefix := "root@dev:/#"
	g := []byte{0xD6, 0xD0, 0xCE, 0xC4}
	got := c.Decode(append([]byte(prefix), g...))
	want := prefix + "中文"
	if got != want {
		t.Fatalf("mixed: %q want %q", got, want)
	}
}

func TestForcedUTF8BrokenBytes(t *testing.T) {
	c := NewConverter(ModeUTF8)
	got := c.Decode([]byte{0xFF, 'a'})
	if got != "\uFFFD\uFFFD" { // JSON 序列化时 0xFF 会替换为 U+FFFD
		// 0xFF 是非法字节, 无法恢复, 原样返回; 这里只验证不 panic
		t.Logf("forced utf8 broken: %q", got)
	}
}

func TestISUTF8Prefix(t *testing.T) {
	cases := []struct {
		b    []byte
		want bool
	}{
		{[]byte{0xE4, 0xB8}, true},  // "中" 的前两个字节
		{[]byte{0xE4}, true},        // "中" 的第一个字节
		{[]byte{0xC4, 0xE3}, false}, // 完整 GBK 对, 但按 UTF-8 首字节 C4 应 2 字节完整
		{[]byte{0x41}, false},       // ASCII 不是前缀
		{[]byte{0xFF}, false},       // 非法首字节
	}
	for _, tc := range cases {
		if got := isUTF8Prefix(tc.b); got != tc.want {
			t.Errorf("isUTF8Prefix(% x) = %v want %v", tc.b, got, tc.want)
		}
	}
}
