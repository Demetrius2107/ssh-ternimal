package vnccore

import (
	"bytes"
	"testing"
)

func Test位反转应逐位翻转(t *testing.T) {
	cases := []struct{ in, want byte }{
		{0x01, 0x80},
		{0x80, 0x01},
		{0xFF, 0xFF},
		{0x00, 0x00},
		{0x0F, 0xF0},
		{0x55, 0xAA}, // 0101 0101 → 1010 1010
	}
	for _, tc := range cases {
		if got := reverseBits(tc.in); got != tc.want {
			t.Errorf("reverseBits(0x%02x) = 0x%02x, want 0x%02x", tc.in, got, tc.want)
		}
	}
}

func TestVNC认证加密应输出16字节(t *testing.T) {
	challenge := make([]byte, 16)
	for i := range challenge {
		challenge[i] = byte(i)
	}
	enc, err := vncEncrypt("password", challenge)
	if err != nil {
		t.Fatalf("vncEncrypt 失败: %v", err)
	}
	if len(enc) != 16 {
		t.Fatalf("加密结果应 16 字节, got %d", len(enc))
	}
}

func TestVNC认证相同输入应稳定输出(t *testing.T) {
	challenge := []byte("0123456789abcdef")
	e1, _ := vncEncrypt("pass", challenge)
	e2, _ := vncEncrypt("pass", challenge)
	if !bytes.Equal(e1, e2) {
		t.Fatal("相同密码与 challenge 应输出相同密文")
	}
}

func TestVNC认证不同密码应输出不同密文(t *testing.T) {
	challenge := []byte("0123456789abcdef")
	e1, _ := vncEncrypt("pass1", challenge)
	e2, _ := vncEncrypt("pass2", challenge)
	if bytes.Equal(e1, e2) {
		t.Fatal("不同密码应输出不同密文")
	}
}

// ---------- 像素解析 (真彩色 mask/shift) ----------

// newTestSession 构造带像素格式的会话
func newTestSession(bpp uint8, bigEnd bool, rShift, gShift, bShift uint8) *Session {
	return &Session{pf: pixelFormat{
		bpp: bpp, depth: 24, bigEnd: bigEnd, trueCol: true,
		rMax: 255, gMax: 255, bMax: 255,
		rShift: rShift, gShift: gShift, bShift: bShift,
	}}
}

func Test32位小端像素应正确提取RGB(t *testing.T) {
	s := newTestSession(32, false, 16, 8, 0)
	// 小端 0x00RRGGBB: R=0x12, G=0x34, B=0x56 → 字节序 56 34 12 00
	raw := []byte{0x56, 0x34, 0x12, 0x00}
	if s.red(raw) != 0x12 {
		t.Errorf("red = %#x, want 0x12", s.red(raw))
	}
	if s.green(raw) != 0x34 {
		t.Errorf("green = %#x, want 0x34", s.green(raw))
	}
	if s.blue(raw) != 0x56 {
		t.Errorf("blue = %#x, want 0x56", s.blue(raw))
	}
}

func Test24位大端像素应正确提取RGB(t *testing.T) {
	s := newTestSession(24, true, 16, 8, 0)
	// 大端 24bit: 字节序 R G B
	raw := []byte{0xAB, 0xCD, 0xEF}
	if s.red(raw) != 0xAB {
		t.Errorf("red = %#x, want 0xAB", s.red(raw))
	}
	if s.green(raw) != 0xCD {
		t.Errorf("green = %#x, want 0xCD", s.green(raw))
	}
	if s.blue(raw) != 0xEF {
		t.Errorf("blue = %#x, want 0xEF", s.blue(raw))
	}
}

func Test16位565像素应正确提取RGB(t *testing.T) {
	s := &Session{pf: pixelFormat{
		bpp: 16, depth: 16, bigEnd: false, trueCol: true,
		rMax: 31, gMax: 63, bMax: 31, // 565 格式
		rShift: 11, gShift: 5, bShift: 0,
	}}
	// 小端 0b RRRRRGGGGGGBBBBB: R=31 G=63 B=31 → 0xFFFF 小端字节 0xFF 0xFF
	raw := []byte{0xFF, 0xFF}
	if s.red(raw) != 255 {
		t.Errorf("red = %d, want 255 (31/31 缩放)", s.red(raw))
	}
	if s.green(raw) != 255 {
		t.Errorf("green = %d, want 255 (63/63 缩放)", s.green(raw))
	}
	if s.blue(raw) != 255 {
		t.Errorf("blue = %d, want 255 (31/31 缩放)", s.blue(raw))
	}
	// R=16 → 16/31*255 ≈ 131
	s2 := &Session{pf: s.pf}
	raw16 := []byte{0x00, 0x80} // 0x8000: R=16, G=0, B=0
	_ = s2
	if got := s.red(raw16); got != 131 {
		t.Errorf("red(16/31) = %d, want 131", got)
	}
}
