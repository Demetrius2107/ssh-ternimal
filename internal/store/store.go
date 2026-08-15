// Package store 会话持久化: bbolt 存配置, 系统凭据库 (Windows Credential Manager) 存密码
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"go.etcd.io/bbolt"

	"ssh-terminal/internal/model"
)

const (
	bucketName     = "sessions"
	snippetBucket  = "snippets"
	aiUsageBucket  = "ai_usage"
	keyringService = "ssh-terminal"
)

// Store 会话存储
type Store struct {
	db *bbolt.DB
}

// Open 打开 (或创建) 会话库, 数据目录: %AppData%/ssh-terminal
func Open() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取配置目录失败: %v", err)
	}
	dir = filepath.Join(dir, "ssh-terminal")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := bbolt.Open(filepath.Join(dir, "sessions.db"), 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(snippetBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(aiUsageBucket))
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭存储
func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

// Save 保存会话; 密码非空时写入系统凭据库
func (s *Store) Save(sess model.StoredSession, password string) (string, error) {
	if sess.ID == "" {
		sess.ID = newID()
	}
	data, err := json.Marshal(sess)
	if err != nil {
		return "", err
	}
	if err := s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Put([]byte(sess.ID), data)
	}); err != nil {
		return "", err
	}
	if password != "" {
		if err := keyring.Set(keyringService, sess.ID, password); err != nil {
			return "", fmt.Errorf("保存密码到系统凭据库失败: %v", err)
		}
	}
	return sess.ID, nil
}

// List 列出全部会话 (空库返回空切片而非 nil, 避免 wails 序列化成 JS null)
func (s *Store) List() ([]model.StoredSession, error) {
	out := []model.StoredSession{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).ForEach(func(k, v []byte) error {
			var sess model.StoredSession
			if err := json.Unmarshal(v, &sess); err == nil {
				out = append(out, sess)
			}
			return nil
		})
	})
	return out, err
}

// Delete 删除会话配置及其密码
func (s *Store) Delete(id string) error {
	_ = keyring.Delete(keyringService, id)
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(bucketName)).Delete([]byte(id))
	})
}

// Load 加载会话配置与密码
func (s *Store) Load(id string) (model.StoredSession, string, error) {
	var sess model.StoredSession
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(bucketName)).Get([]byte(id))
		if v == nil {
			return errors.New("会话不存在")
		}
		return json.Unmarshal(v, &sess)
	})
	if err != nil {
		return sess, "", err
	}
	pw, _ := keyring.Get(keyringService, id)
	return sess, pw, nil
}

// MoveGroup 修改会话所属分组
func (s *Store) MoveGroup(id, group string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("会话不存在")
		}
		var sess model.StoredSession
		if err := json.Unmarshal(v, &sess); err != nil {
			return err
		}
		sess.Group = group
		data, err := json.Marshal(sess)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// ---------- 命令片段 (Snippets) ----------

// SaveSnippet 保存或更新命令片段; ID 为空时自动生成, 返回 ID
func (s *Store) SaveSnippet(sn model.Snippet) (string, error) {
	if sn.ID == "" {
		sn.ID = newID()
	}
	data, err := json.Marshal(sn)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(snippetBucket)).Put([]byte(sn.ID), data)
	})
	return sn.ID, err
}

// ListSnippets 列出全部命令片段 (空库返回空切片而非 nil)
func (s *Store) ListSnippets() ([]model.Snippet, error) {
	out := []model.Snippet{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(snippetBucket)).ForEach(func(k, v []byte) error {
			var sn model.Snippet
			if err := json.Unmarshal(v, &sn); err == nil {
				out = append(out, sn)
			}
			return nil
		})
	})
	return out, err
}

// DeleteSnippet 删除命令片段
func (s *Store) DeleteSnippet(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(snippetBucket)).Delete([]byte(id))
	})
}

// ---------- AI 用量统计 (成本控制, 月度限额) ----------

// AddAIUsage 累计当月 AI token 用量; month 形如 "2026-08"
func (s *Store) AddAIUsage(month string, tokens int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(aiUsageBucket))
		var cur int64
		if v := b.Get([]byte(month)); v != nil {
			_ = json.Unmarshal(v, &cur)
		}
		cur += tokens
		data, err := json.Marshal(cur)
		if err != nil {
			return err
		}
		return b.Put([]byte(month), data)
	})
}

// GetAIUsage 读取当月已用 token 数 (无记录返回 0)
func (s *Store) GetAIUsage(month string) (int64, error) {
	var cur int64
	err := s.db.View(func(tx *bbolt.Tx) error {
		if v := tx.Bucket([]byte(aiUsageBucket)).Get([]byte(month)); v != nil {
			return json.Unmarshal(v, &cur)
		}
		return nil
	})
	return cur, err
}

// newID 生成 16 位十六进制会话 ID
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
