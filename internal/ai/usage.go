// usage AI 用量估算与限额常量 (成本控制)
package ai

import (
	"strings"
	"unicode/utf8"
)

// 月度 token 限额 (默认, 用户可在设置中调整)
const DefaultMonthlyTokenLimit int64 = 5_000_000

// EstimateTokens 粗略估算文本 token 数 (用于流式接口拿不到 usage 时的成本核算)
// 中文约 1 token/字, 英文约 4 字符/token, 按字符加权估算
func EstimateTokens(text string) int64 {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	// 简单加权: 非 ASCII 字符按 1 token, ASCII 按 4 字符 1 token
	ascii := 0
	nonAscii := 0
	for _, r := range text {
		if r < 128 {
			ascii++
		} else {
			nonAscii++
		}
	}
	return int64(nonAscii) + int64((ascii+3)/4)
}

// TrimContext 截断上下文到最大字符数, 防止超长日志刷爆请求
func TrimContext(text string, maxChars int) string {
	if utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	// 优先保留末尾 (最新输出)
	r := []rune(text)
	tail := string(r[len(r)-maxChars:])
	return "…[上下文已截断]…\n" + tail
}

// 是否空输入
func IsBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}
