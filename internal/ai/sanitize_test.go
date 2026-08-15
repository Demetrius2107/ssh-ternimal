package ai

import (
	"strings"
	"testing"
)

func TestSanitizeCredentials(t *testing.T) {
	s := NewSanitizer([]string{"internal-prod-secret"})
	cases := []struct{ in, wantSub string }{
		{"ssh root@1.2.3.4 --password=SuperSecret123", "***"},
		{"export PASSWORD=hunter2; ls", "***"},
		{"curl -H \"Authorization: Bearer abcDEF1234567890xyz\" api", "***"},
		{"mysql://app:dbpass123@10.0.0.1:3306/mydb", "***"},
		{"-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----", "***"},
		{"the internal-prod-secret is used", "***"},
	}
	for _, tc := range cases {
		got := s.Sanitize(tc.in)
		if !strings.Contains(got, tc.wantSub) {
			t.Errorf("Sanitize(%q) = %q, 应包含 %q", tc.in, got, tc.wantSub)
		}
		// 原始密钥值不应残留
		leaks := []string{"SuperSecret123", "hunter2", "abcDEF1234567890xyz", "dbpass123", "MIIEow", "internal-prod-secret"}
		for _, leak := range leaks {
			if strings.Contains(got, leak) {
				t.Errorf("脱敏失败: %q 泄漏了 %q", got, leak)
			}
		}
	}
}

func TestSanitizeKeepsNormalText(t *testing.T) {
	s := NewSanitizer(nil)
	in := "检查磁盘空间 df -h, 以及 uptime"
	got := s.Sanitize(in)
	if got != in {
		t.Errorf("普通文本被误改: %q", got)
	}
}
