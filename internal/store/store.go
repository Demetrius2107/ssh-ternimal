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
	"sort"

	"github.com/zalando/go-keyring"
	"go.etcd.io/bbolt"

	"ssh-terminal/internal/model"
)

const (
	bucketName     = "sessions"
	snippetBucket  = "snippets"
	aiUsageBucket  = "ai_usage"
	auditBucket    = "audit"
	credBucket     = "credentials"
	alertBucket    = "alerts"
	taskBucket     = "tasks"
	keyringService = "ssh-terminal"
)

// AlertConfig 监控告警配置 (CPU/内存/磁盘阈值 + 通知渠道)
type AlertConfig struct {
	Enabled       bool    `json:"enabled"`
	CpuThreshold  float64 `json:"cpuThreshold"`  // % (0=关闭该项)
	MemThreshold  float64 `json:"memThreshold"`  // %
	DiskThreshold float64 `json:"diskThreshold"` // %
	WebhookURL    string  `json:"webhookUrl"`    // 钉钉/自定义 webhook (可空)
}

// AlertRecord 告警历史记录
type AlertRecord struct {
	ID        string  `json:"id"`
	Time      string  `json:"time"`
	Session   string  `json:"session"` // 会话标签
	Metric    string  `json:"metric"`  // cpu / mem / disk
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Type      string  `json:"type"` // alert / recovery
}

// Task 定时任务: 在指定会话按固定间隔执行命令
type Task struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	SessionID       uint64 `json:"sessionId"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Command         string `json:"command"`
	Enabled         bool   `json:"enabled"`
	LastRun         string `json:"lastRun"`   // 最近执行时间
	LastError       string `json:"lastError"` // 最近失败原因 (会话断开等)
}

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
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(auditBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(credBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(alertBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(taskBucket))
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

// ---------- 会话审计 (Audit) ----------

// SaveAudit 保存或更新审计条目 (ID 相同时覆盖, 用于连接结束后补记时长/字节), 返回 ID
func (s *Store) SaveAudit(e model.AuditEntry) (string, error) {
	if e.ID == "" {
		e.ID = newID()
	}
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(auditBucket)).Put([]byte(e.ID), data)
	})
	return e.ID, err
}

// ListAudit 列出全部审计条目, 按开始时间倒序 (最新在前)
func (s *Store) ListAudit() ([]model.AuditEntry, error) {
	out := []model.AuditEntry{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(auditBucket)).ForEach(func(k, v []byte) error {
			var e model.AuditEntry
			if err := json.Unmarshal(v, &e); err == nil {
				out = append(out, e)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	// 倒序: 最新连接在前
	sort.Slice(out, func(i, j int) bool { return out[i].StartTime > out[j].StartTime })
	return out, nil
}

// ClearAudit 清空全部审计条目
func (s *Store) ClearAudit() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(auditBucket)).ForEach(func(k, v []byte) error {
			return tx.Bucket([]byte(auditBucket)).Delete(k)
		})
	})
}

// ---------- 集中凭据 (Keychain) ----------

// credentialKey 凭据在 keyring 中的键
func credentialKey(id string) string { return "cred:" + id }

// SaveCredential 保存凭据 (secret 存系统凭据库, bbolt 只存元数据), 返回 ID
func (s *Store) SaveCredential(c model.Credential, secret string) (string, error) {
	if c.ID == "" {
		c.ID = newID()
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(credBucket)).Put([]byte(c.ID), data)
	})
	if err != nil {
		return "", err
	}
	if secret != "" {
		if err := keyring.Set(keyringService, credentialKey(c.ID), secret); err != nil {
			return "", fmt.Errorf("保存凭据密钥到系统凭据库失败: %v", err)
		}
	}
	return c.ID, nil
}

// ListCredentials 列出全部凭据元数据 (不含 secret)
func (s *Store) ListCredentials() ([]model.CredentialListEntry, error) {
	out := []model.CredentialListEntry{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(credBucket)).ForEach(func(k, v []byte) error {
			var c model.Credential
			if err := json.Unmarshal(v, &c); err == nil {
				out = append(out, model.CredentialListEntry{
					ID: c.ID, Name: c.Name, Type: c.Type, Username: c.Username, CreatedAt: c.CreatedAt,
				})
			}
			return nil
		})
	})
	return out, err
}

// GetCredentialSecret 读取凭据 secret (系统凭据库)
func (s *Store) GetCredentialSecret(id string) (string, error) {
	return keyring.Get(keyringService, credentialKey(id))
}

// DeleteCredential 删除凭据 (含系统凭据库中的 secret)
func (s *Store) DeleteCredential(id string) error {
	_ = keyring.Delete(keyringService, credentialKey(id))
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(credBucket)).Delete([]byte(id))
	})
}

// ---------- 监控告警 ----------

const alertConfigKey = "config"

// SaveAlertConfig 保存告警配置
func (s *Store) SaveAlertConfig(c AlertConfig) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(alertBucket)).Put([]byte(alertConfigKey), data)
	})
}

// GetAlertConfig 读取告警配置 (无配置返回零值)
func (s *Store) GetAlertConfig() (AlertConfig, error) {
	var c AlertConfig
	err := s.db.View(func(tx *bbolt.Tx) error {
		if v := tx.Bucket([]byte(alertBucket)).Get([]byte(alertConfigKey)); v != nil {
			return json.Unmarshal(v, &c)
		}
		return nil
	})
	return c, err
}

// AddAlert 追加一条告警历史 (最多保留 200 条)
func (s *Store) AddAlert(r AlertRecord) error {
	if r.ID == "" {
		r.ID = newID()
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(alertBucket))
		if err := b.Put([]byte(r.ID), data); err != nil {
			return err
		}
		// 超限裁剪: 保留最新 200 条
		var ids []string
		_ = b.ForEach(func(k, v []byte) error {
			ids = append(ids, string(k))
			return nil
		})
		if len(ids) > 200 {
			sort.Strings(ids)
			for _, k := range ids[:len(ids)-200] {
				_ = b.Delete([]byte(k))
			}
		}
		return nil
	})
}

// ListAlerts 列出告警历史 (最新在前)
func (s *Store) ListAlerts() ([]AlertRecord, error) {
	out := []AlertRecord{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(alertBucket)).ForEach(func(k, v []byte) error {
			if string(k) == alertConfigKey {
				return nil // 跳过配置条目
			}
			var r AlertRecord
			if err := json.Unmarshal(v, &r); err == nil {
				out = append(out, r)
			}
			return nil
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out, err
}

// ClearAlerts 清空告警历史 (保留配置)
func (s *Store) ClearAlerts() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(alertBucket))
		var keys []string
		_ = b.ForEach(func(k, v []byte) error {
			if string(k) != alertConfigKey {
				keys = append(keys, string(k))
			}
			return nil
		})
		for _, k := range keys {
			_ = b.Delete([]byte(k))
		}
		return nil
	})
}

// ---------- 定时任务 ----------

// SaveTask 保存或更新定时任务, 返回 ID
func (s *Store) SaveTask(t Task) (string, error) {
	if t.ID == "" {
		t.ID = newID()
	}
	data, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(taskBucket)).Put([]byte(t.ID), data)
	})
	return t.ID, err
}

// ListTasks 列出全部定时任务
func (s *Store) ListTasks() ([]Task, error) {
	out := []Task{}
	err := s.db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(taskBucket)).ForEach(func(k, v []byte) error {
			var t Task
			if err := json.Unmarshal(v, &t); err == nil {
				out = append(out, t)
			}
			return nil
		})
	})
	return out, err
}

// DeleteTask 删除定时任务
func (s *Store) DeleteTask(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(taskBucket)).Delete([]byte(id))
	})
}

// newID 生成 16 位十六进制会话 ID
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
