// Package store 云同步服务端存储: bbolt (users/devices/vault/audit 四个桶)
//
// 零知识: 只存 Vault 密文 + 版本元数据, 永不解密。版本号单调递增由事务保证。
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

var (
	ErrUserExists   = errors.New("用户已存在")
	ErrUserNotFound = errors.New("用户不存在")
	ErrDeviceNotFound = errors.New("设备不存在")
)

const (
	bucketUsers    = "users"
	bucketDevices  = "devices"
	bucketVault    = "vault"
	bucketAudit    = "audit"
	keyVaultRow    = "row" // vault 桶中唯一键 (每用户一个 Vault)
)

// Store 服务端存储
type Store struct {
	db *bbolt.DB
}

// User 用户账户 (口令哈希 + 盐, 不存明文)
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	PwdHash   string `json:"pwdHash"`
	Salt      string `json:"salt"`
	Status    string `json:"status"` // active / locked
	CreatedAt string `json:"createdAt"`
}

// Device 设备 (独立于 JWT 的撤销凭据)
type Device struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Secret   string `json:"secret"` // 设备密钥哈希
	Status   string `json:"status"` // active / revoked
	LastSeen string `json:"lastSeen"`
}

// VaultRow Vault 数据 (零知识: ciphertext 永不解密)
type VaultRow struct {
	Version    int64  `json:"version"`
	Ciphertext string `json:"ciphertext"`
	UpdatedAt  string `json:"updatedAt"`
}

// Open 打开 (或创建) 数据库
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		for _, b := range []string{bucketUsers, bucketDevices, bucketVault, bucketAudit} {
			if _, err := tx.CreateBucketIfNotExists([]byte(b)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库
func (s *Store) Close() error { return s.db.Close() }

// ---------- 用户 ----------

// CreateUser 创建用户 (已存在返回 ErrUserExists)
func (s *Store) CreateUser(u User) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		if b.Get([]byte(u.Email)) != nil {
			return ErrUserExists
		}
		data, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return b.Put([]byte(u.Email), data)
	})
}

// GetUser 按邮箱取用户
func (s *Store) GetUser(email string) (User, error) {
	var u User
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(bucketUsers)).Get([]byte(email))
		if v == nil {
			return ErrUserNotFound
		}
		return json.Unmarshal(v, &u)
	})
	return u, err
}

// UpdateUser 更新用户状态等
func (s *Store) UpdateUser(u User) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketUsers)).Put([]byte(u.Email), data)
	})
}

// ---------- 设备 ----------

// AddDevice 添加设备
func (s *Store) AddDevice(d Device) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketDevices)).Put([]byte(d.ID), data)
	})
}

// GetDevice 按 ID 取设备
func (s *Store) GetDevice(id string) (Device, error) {
	var d Device
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(bucketDevices)).Get([]byte(id))
		if v == nil {
			return ErrDeviceNotFound
		}
		return json.Unmarshal(v, &d)
	})
	return d, err
}

// ListDevices 列出用户全部设备
func (s *Store) ListDevices(userID string) ([]Device, error) {
	out := []Device{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketDevices)).ForEach(func(k, v []byte) error {
			var d Device
			if err := json.Unmarshal(v, &d); err == nil && d.UserID == userID {
				out = append(out, d)
			}
			return nil
		})
	})
	return out, err
}

// UpdateDevice 更新设备状态/最近活跃
func (s *Store) UpdateDevice(d Device) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketDevices)).Put([]byte(d.ID), data)
	})
}

// DeleteDevice 删除设备
func (s *Store) DeleteDevice(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketDevices)).Delete([]byte(id))
	})
}

// ---------- Vault (零知识) ----------

// GetVault 取用户 Vault (无记录返回空行 Version=0)
func (s *Store) GetVault(userID string) (VaultRow, error) {
	var row VaultRow
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(bucketVault)).Get([]byte(userID))
		if v == nil {
			return nil // 无记录: 空行
		}
		return json.Unmarshal(v, &row)
	})
	return row, err
}

// PutVault 原子提交新版本: 仅当 clientVersion == 当前版本时写入并 +1
// 返回 (新版本号, 是否成功); 版本不符时返回当前最新版本供客户端合并
func (s *Store) PutVault(userID string, clientVersion int64, ciphertext string) (int64, bool, error) {
	var newVersion int64
	ok := false
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketVault))
		var cur VaultRow
		if v := b.Get([]byte(userID)); v != nil {
			_ = json.Unmarshal(v, &cur)
		}
		if clientVersion != cur.Version {
			return nil // 冲突: 不改写 (ok 保持 false)
		}
		row := VaultRow{
			Version:    cur.Version + 1,
			Ciphertext: ciphertext,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		}
		data, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(userID), data); err != nil {
			return err
		}
		newVersion = row.Version
		ok = true
		return nil
	})
	return newVersion, ok, err
}

// ---------- 审计日志 ----------

// Audit 记录服务端操作日志 (谁/何时/哪台设备/动作)
func (s *Store) Audit(userID, deviceID, action string) error {
	entry := map[string]string{
		"user":   userID,
		"device": deviceID,
		"action": action,
		"time":   time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketAudit))
		return b.Put([]byte(time.Now().Format("20060102-150405.000")+deviceID), data)
	})
}
