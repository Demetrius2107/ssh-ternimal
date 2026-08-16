// Package vault 端到端加密备份: 会话与凭据导出/导入 (Vault 模式, 服务端不可解密)
//
// 加密: AES-256-GCM; 密钥由用户密码经 PBKDF2-HMAC-SHA256 派生 (随机盐, 迭代 60 万次)。
// 输出格式: base64( salt || nonce || ciphertext )
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltLen   = 16
	nonceLen  = 12
	iterCount = 600_000
	keyLen    = 32
)

// Encrypt 用用户密码加密数据, 返回 base64 字符串
func Encrypt(plain []byte, password string) (string, error) {
	if password == "" {
		return "", errors.New("备份密码不能为空")
	}
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("生成盐失败: %v", err)
	}
	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	out := append(salt, nonce...)
	out = append(out, sealed...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt 解密 vault 备份字符串, 密码错误返回明确错误
func Decrypt(encoded, password string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("备份数据格式无效")
	}
	if len(raw) < saltLen+nonceLen+16 {
		return nil, errors.New("备份数据不完整")
	}
	salt := raw[:saltLen]
	nonce := raw[saltLen : saltLen+nonceLen]
	sealed := raw[saltLen+nonceLen:]
	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, errors.New("密码错误或数据被篡改")
	}
	return plain, nil
}

// deriveKey PBKDF2 派生 AES-256 密钥
func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, iterCount, keyLen, sha256.New)
}
