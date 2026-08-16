// 云同步服务端入口: 账户 + Vault 零知识同步 + 设备管理
//
// 启动: cd server && go run ./cmd/server -db data.db -secret <随机密钥> -addr :8080
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"ssh-terminal/server/internal/auth"
	"ssh-terminal/server/internal/store"
	"ssh-terminal/server/internal/sync"
)

var (
	st     *store.Store
	svc    *sync.Service
	jwt    *auth.JWT
	addr   string
	dbPath string
)

func main() {
	secret := flag.String("secret", os.Getenv("SSH_TERMINAL_SECRET"), "JWT 签名密钥 (生产必填)")
	flag.StringVar(&addr, "addr", ":8080", "监听地址")
	flag.StringVar(&dbPath, "db", "data.db", "数据库文件路径")
	flag.Parse()

	if *secret == "" {
		// 开发模式: 随机密钥 (重启后令牌失效)
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		*secret = hex.EncodeToString(b)
		log.Println("警告: 未设置 -secret, 使用随机密钥 (重启后令牌全部失效)")
	}

	var err error
	st, err = store.Open(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()
	svc = sync.New(st)
	jwt = auth.NewJWT(*secret)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/vault", authRequired(handleVault))
	mux.HandleFunc("/api/devices", authRequired(handleDevices))
	mux.HandleFunc("/api/devices/", authRequired(handleDeviceRevoke))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	})

	log.Printf("云同步服务启动: %s (db=%s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ---------- 工具 ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// authRequired 中间件: 校验 JWT, 注入 email/deviceId 到 context (经请求头传递)
func authRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authz, "Bearer ")
		email, deviceID, err := jwt.VerifyToken(token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		// 校验设备仍有效
		d, derr := st.GetDevice(deviceID)
		if derr != nil || d.Status != "active" {
			writeErr(w, http.StatusUnauthorized, "设备已撤销或不存在")
			return
		}
		svc.TouchDevice(deviceID)
		_ = st.Audit(email, deviceID, r.Method+" "+r.URL.Path)
		r.Header.Set("X-User-Email", email)
		r.Header.Set("X-Device-ID", deviceID)
		next(w, r)
	}
}

func reqUser(r *http.Request) string { return r.Header.Get("X-User-Email") }
func reqDevice(r *http.Request) string { return r.Header.Get("X-Device-ID") }

func newDeviceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- 账户 ----------

type credReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Device   string `json:"device"` // 设备名, 注册必填, 登录可空
}

// handleRegister 注册: 创建用户 + 主设备, 返回令牌
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	var req credReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式无效")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || len(req.Password) < 6 || strings.TrimSpace(req.Device) == "" {
		writeErr(w, http.StatusBadRequest, "邮箱/密码(≥6位)/设备名不能为空")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "服务端错误")
		return
	}
	u := store.User{
		Email: req.Email, PwdHash: hash, Status: "active",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := st.CreateUser(u); err != nil {
		if errors.Is(err, store.ErrUserExists) {
			writeErr(w, http.StatusConflict, "用户已存在")
			return
		}
		writeErr(w, http.StatusInternalServerError, "服务端错误")
		return
	}
	// 主设备
	deviceID := newDeviceID()
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	secretHash, _ := auth.HashPassword(hex.EncodeToString(secret))
	_ = st.AddDevice(store.Device{
		ID: deviceID, UserID: req.Email, Name: req.Device,
		Secret: secretHash, Status: "active", LastSeen: time.Now().Format(time.RFC3339),
	})
	token, _ := jwt.GenerateToken(req.Email, deviceID)
	_ = st.Audit(req.Email, deviceID, "register")
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "deviceId": deviceID})
}

// handleLogin 登录: 校验口令, 返回令牌 (设备需已存在)
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}
	var req credReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "请求格式无效")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	u, err := st.GetUser(req.Email)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	if u.Status != "active" {
		writeErr(w, http.StatusForbidden, "账户已锁定")
		return
	}
	if !auth.VerifyPassword(u.PwdHash, req.Password) {
		writeErr(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	// 校验设备有效
	devices, _ := st.ListDevices(req.Email)
	deviceID := ""
	for _, d := range devices {
		if d.Status == "active" {
			deviceID = d.ID
			break
		}
	}
	if deviceID == "" {
		writeErr(w, http.StatusUnauthorized, "无有效设备, 请重新注册")
		return
	}
	token, _ := jwt.GenerateToken(req.Email, deviceID)
	_ = st.Audit(req.Email, deviceID, "login")
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "deviceId": deviceID})
}

// ---------- Vault 同步 (零知识) ----------

// handleVault GET 拉取 / PUT 上传
func handleVault(w http.ResponseWriter, r *http.Request) {
	user := reqUser(r)
	device := reqDevice(r)
	switch r.Method {
	case http.MethodGet:
		row, err := svc.Pull(user)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "读取失败")
			return
		}
		writeJSON(w, http.StatusOK, row)
	case http.MethodPut:
		var req struct {
			Version    int64  `json:"version"`
			Ciphertext string `json:"ciphertext"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "请求格式无效")
			return
		}
		if len(req.Ciphertext) > 4*1024*1024 {
			writeErr(w, http.StatusRequestEntityTooLarge, "密文超过 4MB 上限")
			return
		}
		newVersion, err := svc.Push(user, req.Version, req.Ciphertext)
		if err != nil {
			if errors.Is(err, sync.ErrVaultConflict) {
				// 返回最新版本供客户端合并
				writeErr(w, http.StatusConflict, fmt.Sprintf("版本冲突, 当前最新版本 %d", newVersion))
				return
			}
			writeErr(w, http.StatusInternalServerError, "写入失败")
			return
		}
		_ = st.Audit(user, device, "vault:push")
		writeJSON(w, http.StatusOK, map[string]int64{"version": newVersion})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 GET/PUT")
	}
}

// ---------- 设备管理 ----------

func handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 GET")
		return
	}
	devices, err := st.ListDevices(reqUser(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "查询失败")
		return
	}
	// 不返回设备密钥
	out := make([]map[string]string, 0, len(devices))
	for _, d := range devices {
		out = append(out, map[string]string{
			"id": d.ID, "name": d.Name, "status": d.Status, "lastSeen": d.LastSeen,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleDeviceRevoke DELETE /api/devices/{id}
func handleDeviceRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErr(w, http.StatusMethodNotAllowed, "仅支持 DELETE")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/devices/")
	d, err := st.GetDevice(id)
	if err != nil || d.UserID != reqUser(r) {
		writeErr(w, http.StatusNotFound, "设备不存在")
		return
	}
	d.Status = "revoked"
	_ = st.UpdateDevice(d)
	_ = st.Audit(reqUser(r), reqDevice(r), "device:revoke:"+id)
	w.WriteHeader(http.StatusNoContent)
}
