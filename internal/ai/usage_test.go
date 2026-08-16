package ai

import (
	"strings"
	"testing"
)

func Test估算token数应按中英文字符加权(t *testing.T) {
	// 纯 ASCII: 每 4 字符约 1 token
	ascii := EstimateTokens("ls -la /tmp")
	if ascii <= 0 {
		t.Fatalf("ASCII 估算应 > 0, got %d", ascii)
	}
	// 中文: 每字约 1 token, 应显著大于同长度 ASCII
	chinese := EstimateTokens("查看磁盘空间和内存使用情况")
	if chinese < 10 {
		t.Fatalf("中文估算应 >= 字数, got %d", chinese)
	}
	// 空输入为 0
	if EstimateTokens("") != 0 {
		t.Fatal("空输入估算应为 0")
	}
}

func Test上下文截断应保留最新尾部(t *testing.T) {
	text := strings.Repeat("A", 5000) + "TAIL-MARKER"
	got := TrimContext(text, 1000)
	if len([]rune(got)) > 1100 {
		t.Fatalf("截断后长度应接近上限, got %d", len([]rune(got)))
	}
	if !strings.Contains(got, "TAIL-MARKER") {
		t.Fatal("截断应保留最新输出 (尾部)")
	}
	if !strings.Contains(got, "上下文已截断") {
		t.Fatal("截断应包含提示标记")
	}
}

func Test短内容不应被截断(t *testing.T) {
	text := "short content"
	if TrimContext(text, 1000) != text {
		t.Fatal("短内容不应被截断")
	}
}

func Test空白判断(t *testing.T) {
	if !IsBlank("   \n\t") {
		t.Fatal("纯空白应判为空白")
	}
	if IsBlank("x") {
		t.Fatal("非空白不应判为空白")
	}
}
