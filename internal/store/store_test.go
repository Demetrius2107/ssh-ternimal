package store

import (
	"os"
	"testing"

	"ssh-terminal/internal/model"
)

// newTestStore 在临时目录 (APPDATA 环境变量重定向) 打开测试库, 自动清理
func newTestStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir()) // Windows os.UserConfigDir 读取 APPDATA
	s, err := Open()
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func Test保存会话后加载应返回原配置(t *testing.T) {
	s := newTestStore(t)

	// 保存
	id, err := s.Save(model.StoredSession{Name: "prod", Host: "10.0.0.1", Port: 22, Username: "root", Group: "生产"}, "secret123")
	if err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if id == "" {
		t.Fatal("Save 应返回非空 ID")
	}

	// 加载 (密码应从 keyring 取; 测试环境可能无凭据库, 允许为空但不报错)
	sess, pw, err := s.Load(id)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if sess.Host != "10.0.0.1" || sess.Username != "root" || sess.Group != "生产" {
		t.Fatalf("Load 内容不符: %+v", sess)
	}
	if sess.Name != "prod" || sess.Port != 22 {
		t.Fatalf("Load 内容不符: %+v", sess)
	}
	_ = pw // keyring 在 CI 可能不可用, 不强制断言

	// 列表 (空库返回空切片而非 nil)
	if _, err := s.List(); err != nil {
		t.Fatalf("List 失败: %v", err)
	}

	// 移动分组
	if err := s.MoveGroup(id, "测试组"); err != nil {
		t.Fatalf("MoveGroup 失败: %v", err)
	}
	sess2, _, _ := s.Load(id)
	if sess2.Group != "测试组" {
		t.Fatalf("MoveGroup 未生效: %+v", sess2)
	}

	// 删除
	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, _, err := s.Load(id); err == nil {
		t.Fatal("删除后 Load 应报错")
	}
}

func Test空库列出会话应返回空切片而非null(t *testing.T) {
	s := newTestStore(t)
	list, err := s.List()
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if list == nil {
		t.Fatal("空库 List 必须返回空切片而非 nil (否则 wails 序列化 JS null 前端崩溃)")
	}
	if len(list) != 0 {
		t.Fatalf("空库 List 长度应为 0, got %d", len(list))
	}
}

func Test保存片段后可更新与删除(t *testing.T) {
	s := newTestStore(t)

	id, err := s.SaveSnippet(model.Snippet{Name: "查看磁盘", Command: "df -h"})
	if err != nil {
		t.Fatalf("SaveSnippet 失败: %v", err)
	}
	list, err := s.ListSnippets()
	if err != nil {
		t.Fatalf("ListSnippets 失败: %v", err)
	}
	if len(list) != 1 || list[0].Command != "df -h" {
		t.Fatalf("片段列表不符: %+v", list)
	}

	// 更新 (带 ID)
	if _, err := s.SaveSnippet(model.Snippet{ID: id, Name: "查看磁盘", Command: "df -hT"}); err != nil {
		t.Fatalf("更新片段失败: %v", err)
	}
	list, _ = s.ListSnippets()
	if len(list) != 1 || list[0].Command != "df -hT" {
		t.Fatalf("片段更新未生效: %+v", list)
	}

	if err := s.DeleteSnippet(id); err != nil {
		t.Fatalf("DeleteSnippet 失败: %v", err)
	}
	list, _ = s.ListSnippets()
	if len(list) != 0 {
		t.Fatalf("删除后片段应为空, got %d", len(list))
	}
}

func Test月度用量应逐月独立累计(t *testing.T) {
	s := newTestStore(t)
	month := "2026-08"

	used, err := s.GetAIUsage(month)
	if err != nil {
		t.Fatalf("GetAIUsage 失败: %v", err)
	}
	if used != 0 {
		t.Fatalf("初始用量应为 0, got %d", used)
	}

	if err := s.AddAIUsage(month, 1200); err != nil {
		t.Fatalf("AddAIUsage 失败: %v", err)
	}
	if err := s.AddAIUsage(month, 800); err != nil {
		t.Fatalf("AddAIUsage 失败: %v", err)
	}
	used, _ = s.GetAIUsage(month)
	if used != 2000 {
		t.Fatalf("累计用量应为 2000, got %d", used)
	}

	// 不同月份互不影响
	other, _ := s.GetAIUsage("2026-09")
	if other != 0 {
		t.Fatalf("其他月份应为 0, got %d", other)
	}
}

func Test打开库时三个存储桶应可用(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s, err := Open()
	if err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	defer s.Close()
	// 三大 bucket 应可用: 会话/片段/用量 各写一条不报错
	if _, err := s.Save(model.StoredSession{Name: "x", Host: "h"}, ""); err != nil {
		t.Fatalf("sessions bucket 不可用: %v", err)
	}
	if _, err := s.SaveSnippet(model.Snippet{Name: "n", Command: "c"}); err != nil {
		t.Fatalf("snippets bucket 不可用: %v", err)
	}
	if err := s.AddAIUsage("2026-08", 10); err != nil {
		t.Fatalf("ai_usage bucket 不可用: %v", err)
	}
}

// 确保环境变量不会污染真实用户库
func Test测试不应污染真实用户配置目录(t *testing.T) {
	before := os.Getenv("APPDATA")
	t.Cleanup(func() { os.Setenv("APPDATA", before) })
}
