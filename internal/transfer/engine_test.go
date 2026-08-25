package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConflictOverwrite 覆盖策略: 返回原目标, 不跳过
func Test冲突策略为覆盖时应直接使用目标路径(t *testing.T) {
	target, skip, err := resolveConflict("/tmp/x", "overwrite", func(string) (bool, error) { return true, nil })
	if err != nil || skip || target != "/tmp/x" {
		t.Fatalf("overwrite 应直接返回目标: target=%q skip=%v err=%v", target, skip, err)
	}
}

// TestResolveConflictSkip 跳过策略: 标记跳过
func Test冲突策略为跳过时应标记跳过(t *testing.T) {
	_, skip, err := resolveConflict("/tmp/x", "skip", func(string) (bool, error) { return true, nil })
	if err != nil || !skip {
		t.Fatalf("skip 应返回 skip=true, got %v %v", skip, err)
	}
}

// TestResolveConflictRename 改名策略: 生成 "base (n).ext" 直到不冲突
func Test冲突策略为改名时应生成不冲突的新名称(t *testing.T) {
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
func Test无扩展名文件改名时也应追加序号(t *testing.T) {
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
func Test冲突探测出错时应返回错误(t *testing.T) {
	_, _, err := resolveConflict("/tmp/x", "rename", func(string) (bool, error) { return false, os.ErrPermission })
	if err == nil {
		t.Fatal("探测出错应返回错误")
	}
}

// TestCollectLocalFiles 递归收集文件与总大小
func Test递归收集文件应包含子目录并统计总大小(t *testing.T) {
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
func Test空目录收集文件应无文件(t *testing.T) {
	files, total, err := collectLocalFiles(t.TempDir())
	if err != nil {
		t.Fatalf("collectLocalFiles 失败: %v", err)
	}
	if len(files) != 0 || total != 0 {
		t.Fatalf("空目录应无文件: %d files, %d bytes", len(files), total)
	}
}

// TestResolveLocalTargetNotExists 目标不存在直接可用
func Test本地目标不存在时应直接可用(t *testing.T) {
	p := filepath.Join(t.TempDir(), "new.txt")
	target, skip, err := resolveLocalTarget(p, "overwrite")
	if err != nil || skip || target != p {
		t.Fatalf("不存在目标应直接可用: %q %v %v", target, skip, err)
	}
}

// ---------- 取消 / 移除 路径 ----------

// TestCancelMarksCancelled Cancel 后任务标记为已取消
func Test取消任务后应标记为已取消(t *testing.T) {
	e := NewEngine()
	task := e.newTask(1, "upload", "/l", "/r", 100, "overwrite", false)
	e.Cancel(task.TaskID)
	if !e.isCancelled(task.TaskID) {
		t.Fatal("Cancel 后 isCancelled 应为 true")
	}
}

// TestCancelTaskStatus 运行中循环检测到取消后状态置为 cancelled
func Test运行中检测到取消时状态置为已取消(t *testing.T) {
	e := NewEngine()
	task := e.newTask(1, "upload", "/l", "/r", 100, "overwrite", false)
	e.Cancel(task.TaskID)
	e.cancelTask(task)
	if task.Status != "cancelled" {
		t.Fatalf("状态应为 cancelled, got %q", task.Status)
	}
	if task.Error != "传输已取消" {
		t.Fatalf("取消任务的错误信息不符: %q", task.Error)
	}
}

// TestRemoveRunningRefused 运行中任务不可移除
func Test运行中任务不可移除(t *testing.T) {
	e := NewEngine()
	task := e.newTask(1, "upload", "/l", "/r", 100, "overwrite", false)
	if e.Remove(task.TaskID) {
		t.Fatal("运行中任务不应被移除")
	}
	if _, ok := e.tasks[task.TaskID]; !ok {
		t.Fatal("移除被拒后任务应仍在表中")
	}
}

// TestRemoveFinished 已结束任务可移除
func Test已结束任务可移除(t *testing.T) {
	e := NewEngine()
	task := e.newTask(1, "upload", "/l", "/r", 100, "overwrite", false)
	e.done(task)
	if !e.Remove(task.TaskID) {
		t.Fatal("已结束任务应可移除")
	}
	if _, ok := e.tasks[task.TaskID]; ok {
		t.Fatal("移除后任务不应仍在表中")
	}
	// 移除会连带清掉取消标记
	if e.isCancelled(task.TaskID) {
		t.Fatal("移除后取消标记应一并清除")
	}
}

// TestRemoveFinishedOnlyFinished RemoveFinished 只清理非运行中任务
func Test清理已完成仅删非运行中任务(t *testing.T) {
	e := NewEngine()
	running := e.newTask(1, "upload", "/l1", "/r", 100, "overwrite", false)
	done := e.newTask(1, "download", "/l2", "/r", 100, "overwrite", false)
	errored := e.newTask(1, "upload", "/l3", "/r", 100, "overwrite", false)
	e.done(done)
	e.fail(errored, errors.New("boom"))

	n := e.RemoveFinished()
	if n != 2 {
		t.Fatalf("应清理 2 个非运行中任务, got %d", n)
	}
	if _, ok := e.tasks[running.TaskID]; !ok {
		t.Fatal("运行中任务不应被清理")
	}
	if len(e.tasks) != 1 {
		t.Fatalf("清理后应只剩运行中任务, got %d", len(e.tasks))
	}
}
