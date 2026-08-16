// Package auth 账户认证: PBKDF2 口令哈希 + JWT 签发/校验
//
// 口令哈希参数与客户端 vault 加密派生保持一致 (迭代 60 万, 随机盐)。
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const (
	iterCount = 600_000
	keyLen    = 32
	tokenTTL  = 24 * time.Hour // 访问令牌有效期
)

// HashPassword PBKDF2 派生口令哈希, 返回 "iter:盐:哈希"
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	hash := pbkdf2.Key([]byte(password), salt, iterCount, keyLen, sha256.New)
	return fmt.Sprintf("%d:%s:%s", iterCount, hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// VerifyPassword 校验口令与存储哈希是否匹配 (恒定时间比较)
func VerifyPassword(stored, password string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 3 {
		return false
	}
	iter, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := pbkdf2.Key([]byte(password), salt, iter, len(want), sha256.New)
	return subtle.ConstantTimeCompare(got, want) == 1
}

// JWT 手写 HS256 (无第三方依赖)
type JWT struct {
	secret []byte
}

// NewJWT 创建 JWT 签发器
func NewJWT(secret string) *JWT {
	return &JWT{secret: []byte(secret)}
}

// GenerateToken 签发访问令牌: payload = base64url(header).base64url(claims).sig
// claims: {sub: email, dev: deviceId, exp: unix}
func (j *JWT) GenerateToken(email, deviceID string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"sub":%q,"dev":%q,"exp":%d}`, email, deviceID, time.Now().Add(tokenTTL).Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	sig := j.sign(header + "." + payload)
	return header + "." + payload + "." + sig, nil
}

// VerifyToken 校验并解析令牌, 返回 (email, deviceID, error)
func (j *JWT) VerifyToken(token string) (string, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", "", errors.New("令牌格式无效")
	}
	want := j.sign(parts[0] + "." + parts[1])
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(want)) != 1 {
		return "", "", errors.New("令牌签名无效")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", errors.New("令牌载荷无效")
	}
	claims := parseClaims(string(raw))
	if claims["exp"] == "" {
		return "", "", errors.New("令牌缺少过期时间")
	}
	exp, err := strconv.ParseInt(claims["exp"], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", "", errors.New("令牌已过期")
	}
	return claims["sub"], claims["dev"], nil
}

func (j *JWT) sign(data string) string {
	mac := hmac.New(sha256.New, j.secret)
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// parseClaims 极简 JSON claims 解析 (sub/dev/exp 均为字符串/数字)
func parseClaims(s string) map[string]string {
	out := map[string]string{}
	// 逐字段解析 {"sub":"a","dev":"b","exp":123}
	for _, seg := range strings.Split(strings.Trim(s, "{}"), ",") {
		kv := strings.SplitN(seg, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.Trim(kv[0], `"`)
		val := strings.Trim(kv[1], `"`)
		out[key] = val
	}
	return out
}
