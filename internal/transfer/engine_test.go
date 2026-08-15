package transfer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConflictOverwrite 覆盖策略: 返回原目标, 不跳过
func TestResolveConflictOverwrite(t *testing.T) {
	target, skip, err := resolveConflict("/tmp/x", "overwrite", func(string) (bool, error) { return true, nil })
	if err != nil || skip || target != "/tmp/x" {
		t.Fatalf("overwrite 应直接返回目标: target=%q skip=%v err=%v", target, skip, err)
	}
}

// TestResolveConflictSkip 跳过策略: 标记跳过
func TestResolveConflictSkip(t *testing.T) {
	_, skip, err := resolveConflict("/tmp/x", "skip", func(string) (bool, error) { return true, nil })
	if err != nil || !skip {
		t.Fatalf("skip 应返回 skip=true, got %v %v", skip, err)
	}
}

// TestResolveConflictRename 改名策略: 生成 "base (n).ext" 直到不冲突
func TestResolveConflictRename(t *testing.T) {
	exists := map[string]bool{"/tmp/a.txt": true, "/tmp/a (1).txt": true}
	target, skip, err := resolveConflict("/tmp/a.txt", "rename", func(p string) (bool, error) { return exists[p], nil })
	if err != nil || skip {
		t.Fatalf("rename 不应跳过: %v %v", skip, err)
	}
	if target != "/tmp/a (2).txt" {
		t.Fatalf("改名结果不符: %q", target)
	}
}

// TestResolveConflictRenameNoExt 无扩展名文件改名
func TestResolveConflictRenameNoExt(t *testing.T) {
	exists := map[string]bool{"/tmp/backup": true}
	target, _, err := resolveConflict("/tmp/backup", "rename", func(p string) (bool, error) { return exists[p], nil })
	if err != nil {
		t.Fatalf("rename 失败: %v", err)
	}
	if target != "/tmp/backup (1)" {
		t.Fatalf("无扩展名改名结果不符: %q", target)
	}
}

// TestResolveConflictExistsError 探测出错应传播
func TestResolveConflictExistsError(t *testing.T) {
	_, _, err := resolveConflict("/tmp/x", "rename", func(string) (bool, error) { return false, os.ErrPermission })
	if err == nil {
		t.Fatal("探测出错应返回错误")
	}
}

// TestCollectLocalFiles 递归收集文件与总大小
func TestCollectLocalFiles(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.txt"), make([]byte, 10), 0644)
	sub := filepath.Join(root, "sub")
	_ = os.Mkdir(sub, 0755)
	_ = os.WriteFile(filepath.Join(sub, "b.log"), make([]byte, 20), 0644)

	files, total, err := collectLocalFiles(root)
	if err != nil {
		t.Fatalf("collectLocalFiles 失败: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("应收集 2 个文件, got %d", len(files))
	}
	if total != 30 {
		t.Fatalf("总大小应为 30, got %d", total)
	}
	// relPath 用斜杠, 且包含子目录
	got := map[string]bool{}
	for _, f := range files {
		got[f.relPath] = true
	}
	if !got["a.txt"] || !got["sub/b.log"] {
		t.Fatalf("relPath 不符: %+v", got)
	}
}

// TestCollectLocalFilesEmpty 空目录返回空
func TestCollectLocalFilesEmpty(t *testing.T) {
	files, total, err := collectLocalFiles(t.TempDir())
	if err != nil {
		t.Fatalf("collectLocalFiles 失败: %v", err)
	}
	if len(files) != 0 || total != 0 {
		t.Fatalf("空目录应无文件: %d files, %d bytes", len(files), total)
	}
}

// TestResolveLocalTargetNotExists 目标不存在直接可用
func TestResolveLocalTargetNotExists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "new.txt")
	target, skip, err := resolveLocalTarget(p, "overwrite")
	if err != nil || skip || target != p {
		t.Fatalf("不存在目标应直接可用: %q %v %v", target, skip, err)
	}
}
