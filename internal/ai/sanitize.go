// sanitize 脱敏: 发送给 AI 之前过滤密码/令牌/密钥/用户敏感词
package ai

import (
	"regexp"
	"strings"
)

// 常见凭据/密钥正则 (值统一替换为 ***, 保留键名便于上下文理解)
var credentialPatterns = []*regexp.Regexp{
	// 命令行密码参数: --password=xxx / --token=xxx / -p xxx / PASSWORD=xxx
	regexp.MustCompile(`(?i)(--?password|--?passwd|--?token|--?secret|--?api[-_]?key|--?auth)([=:\s]+)([^\s"'&,;]+)`),
	regexp.MustCompile(`(?i)(password|passwd|token|secret|api[-_]?key|access[-_]?key|private[-_]?key)(\s*[=:]\s*)([^\s"'&,;]+)`),
	// 环境变量赋值: export PASSWORD=xxx / PASSWORD="xxx"
	regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[-_]?key|access[-_]?key)(\s*=\s*)("[^"]*"|'[^']*'|[^\s]+)`),
	// 内置 Bearer / Basic 认证头
	regexp.MustCompile(`(?i)(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`),
	// 私钥 PEM 块整体替换
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
	// AWS 样式密钥 AKIA...
	regexp.MustCompile(`(?i)\b(AKIA|ASIA)[A-Z0-9]{16}\b`),
	// 数据库连接串中的密码: mysql://user:pass@host
	regexp.MustCompile(`(?i)([a-z]+://[^:/@\s]+:)[^@\s]+(@)`),
	// 长 hex/base64 疑似哈希或令牌
	regexp.MustCompile(`(?i)\b(?:[a-f0-9]{32}|[a-f0-9]{40}|[a-f0-9]{64})\b`),
}

// Sanitizer 脱敏器 (每次请求前构建, 可携带用户自定义敏感词)
type Sanitizer struct {
	// 用户自定义敏感词: 原始词 → 显示为 ***
	keywords []string
}

// NewSanitizer 创建脱敏器, keywords 为用户自定义敏感词列表
func NewSanitizer(keywords []string) *Sanitizer {
	var kw []string
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k != "" {
			kw = append(kw, k)
		}
	}
	return &Sanitizer{keywords: kw}
}

// Sanitize 脱敏处理: 替换凭据值与用户敏感词
func (s *Sanitizer) Sanitize(text string) string {
	out := text
	for _, re := range credentialPatterns {
		out = re.ReplaceAllString(out, s.mask(out, re))
	}
	for _, kw := range s.keywords {
		out = strings.ReplaceAll(out, kw, "***")
	}
	return out
}

// mask 按模式生成替换串: 尽量保留键名与结构, 仅掩盖值
func (s *Sanitizer) mask(text string, re *regexp.Regexp) string {
	m := re.FindStringSubmatch(text)
	switch {
	case len(m) >= 4 && strings.Contains(re.String(), `-----BEGIN`):
		return "*** [PRIVATE KEY 已脱敏] ***"
	case len(m) >= 4: // 键 + 分隔符 + 值
		return m[1] + m[2] + "***"
	case len(m) >= 3 && strings.Contains(re.String(), `://`): // URL 密码
		return m[1] + "***" + m[2]
	case len(m) >= 2: // 键 + 值
		return m[1] + "***"
	default:
		return "***"
	}
}
