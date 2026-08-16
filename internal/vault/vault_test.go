package vault

import (
	"strings"
	"testing"
)

func Test加密后应能解密还原原文(t *testing.T) {
	plain := []byte(`{"version":1,"sessions":[{"name":"prod","host":"10.0.0.1"}]}`)
	enc, err := Encrypt(plain, "BackupPass#2026")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if enc == "" || strings.Contains(enc, string(plain)) {
		t.Fatalf("密文不应为空且不应包含明文: %q", enc)
	}
	dec, err := Decrypt(enc, "BackupPass#2026")
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if string(dec) != string(plain) {
		t.Fatalf("解密结果与原文不符: %s", dec)
	}
}

func Test密码错误时解密应失败(t *testing.T) {
	enc, err := Encrypt([]byte("secret"), "correct-password")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	if _, err := Decrypt(enc, "wrong-password"); err == nil {
		t.Fatal("错误密码应解密失败")
	}
}

func Test空密码加密应拒绝(t *testing.T) {
	if _, err := Encrypt([]byte("x"), ""); err == nil {
		t.Fatal("空密码应拒绝加密")
	}
}

func Test篡改密文后解密应失败(t *testing.T) {
	enc, err := Encrypt([]byte("secret-data"), "pass")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	raw := []byte(enc)
	// 翻转密文末尾一个字符模拟篡改 (GCM 认证应失败)
	if raw[len(raw)-1] == 'A' {
		raw[len(raw)-1] = 'B'
	} else {
		raw[len(raw)-1] = 'A'
	}
	if _, err := Decrypt(string(raw), "pass"); err == nil {
		t.Fatal("篡改后的密文应解密失败")
	}
}

func Test无效base64输入应报格式错误(t *testing.T) {
	if _, err := Decrypt("!!!not-base64!!!", "pass"); err == nil {
		t.Fatal("无效 base64 应报错")
	}
}

func Test同一密码多次加密应产生不同密文(t *testing.T) {
	enc1, _ := Encrypt([]byte("same"), "pass")
	enc2, _ := Encrypt([]byte("same"), "pass")
	if enc1 == enc2 {
		t.Fatal("随机盐应使两次加密结果不同")
	}
}
