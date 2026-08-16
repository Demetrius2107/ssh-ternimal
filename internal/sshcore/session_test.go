package sshcore

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"ssh-terminal/internal/model"
)

// testPublicKey 生成一个真实 ed25519 公钥 (known_hosts 写入测试用)
func testPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		t.Fatalf("转 ssh.PublicKey 失败: %v", err)
	}
	return pub
}

// ---------- knownhosts ----------

func Test主机端口解析应支持IPv4IPv6与默认端口回退(t *testing.T) {
	cases := []struct {
		in       string
		wantHost string
		wantPort int
	}{
		{"10.0.0.1:22", "10.0.0.1", 22},
		{"[2001:db8::1]:2201", "2001:db8::1", 2201},
		{"host.example.com:2222", "host.example.com", 2222},
		{"noport.example.com", "noport.example.com", 22}, // 无端口回退默认
		{"bad:port:extra", "bad:port:extra", 22},         // 解析失败回退
	}
	for _, tc := range cases {
		host, port := splitHostPort(tc.in)
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("splitHostPort(%q) = (%q,%d), want (%q,%d)", tc.in, host, port, tc.wantHost, tc.wantPort)
		}
	}
}

func Test接受主机密钥后应写入known_hosts(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	// 生成一个真实 ssh 公钥做写入测试
	key := testPublicKey(t)
	if err := AcceptHostKey("10.0.0.1", 22, key); err != nil {
		t.Fatalf("AcceptHostKey 失败: %v", err)
	}
	path, err := KnownHostsPath()
	if err != nil {
		t.Fatalf("KnownHostsPath 失败: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 known_hosts 失败: %v", err)
	}
	line := string(data)
	// knownhosts.Line 规范: 默认端口 22 省略, 写成 "host ssh-ed25519 ..."
	if !strings.Contains(line, "10.0.0.1 ssh-ed25519") {
		t.Fatalf("known_hosts 应包含 host 条目: %q", line)
	}
	if !strings.Contains(line, "ssh-ed25519") {
		t.Fatalf("known_hosts 应包含公钥类型: %q", line)
	}
}

func Test多个主机密钥应各自追加互不覆盖(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	key := testPublicKey(t)
	_ = AcceptHostKey("a.example.com", 22, key)
	_ = AcceptHostKey("b.example.com", 2201, key)
	path, _ := KnownHostsPath()
	data, _ := os.ReadFile(path)
	text := string(data)
	// knownhosts.Line 规范: 非默认端口写成 [host]:port
	if !strings.Contains(text, "a.example.com ssh-ed25519") || !strings.Contains(text, "[b.example.com]:2201 ssh-ed25519") {
		t.Fatalf("多主机追加失败: %q", text)
	}
}

// TestKnownHostsPath 路径位于 APPDATA/ssh-terminal/known_hosts
func TestKnown_hosts路径应位于配置目录下(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	p, err := KnownHostsPath()
	if err != nil {
		t.Fatalf("KnownHostsPath 失败: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "ssh-terminal/known_hosts") {
		t.Fatalf("路径不符: %s", p)
	}
}

// ---------- session.go ----------

func Test文件列表排序应目录优先且名称不区分大小写(t *testing.T) {
	entries := []model.FileEntry{
		{Name: "fileB.txt", IsDir: false},
		{Name: "dirA", IsDir: true},
		{Name: "filea.txt", IsDir: false},
		{Name: "dirb", IsDir: true},
	}
	sortEntries(entries)
	// 目录在前, 名称不区分大小写字典序
	if !entries[0].IsDir || entries[0].Name != "dirA" {
		t.Fatalf("目录应排前且有序: %+v", entries)
	}
	if !entries[1].IsDir || entries[1].Name != "dirb" {
		t.Fatalf("目录应排前且有序: %+v", entries)
	}
	if entries[2].Name != "filea.txt" || entries[3].Name != "fileB.txt" {
		t.Fatalf("文件应按不区分大小写排序: %+v", entries)
	}
}
