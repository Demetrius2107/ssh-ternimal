package localfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 准备临时目录: dirA/ (目录), fileB.txt, filea.txt
func setupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.Mkdir(filepath.Join(dir, "dirA"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "fileB.txt"), []byte("b"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "filea.txt"), []byte("aaaa"), 0644)
	return dir
}

func Test列出本地目录应目录在前按名称排序(t *testing.T) {
	dir := setupDir(t)
	entries, err := ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir 失败: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("应有 3 个条目, got %d", len(entries))
	}
	// 目录在前, 文件按不区分大小写排序
	if !entries[0].IsDir || entries[0].Name != "dirA" {
		t.Fatalf("目录应排第一: %+v", entries[0])
	}
	if entries[1].Name != "filea.txt" || entries[2].Name != "fileB.txt" {
		t.Fatalf("文件排序不符: %+v", entries)
	}
	// 大小与路径
	if entries[1].Size != 4 {
		t.Fatalf("filea.txt 大小应为 4, got %d", entries[1].Size)
	}
	if entries[0].Path != filepath.Join(dir, "dirA") {
		t.Fatalf("Path 不符: %s", entries[0].Path)
	}
}

func Test空目录应返回空列表(t *testing.T) {
	entries, err := ListDir(t.TempDir())
	if err != nil {
		t.Fatalf("ListDir 失败: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("空目录应为 0 条, got %d", len(entries))
	}
}

func Test列出不存在的目录应报错(t *testing.T) {
	if _, err := ListDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("不存在的目录应报错")
	}
}

func Test上级目录解析应支持多级与根目录(t *testing.T) {
	// 本应用为 Windows 平台, 前端传入的均为 Windows 路径
	cases := []struct{ in, want string }{
		{"C:\\a\\b", "C:\\a"},
		{"C:\\a", "C:\\"},
		{"C:\\", ""},
	}
	for _, tc := range cases {
		if got := Parent(tc.in); got != tc.want {
			t.Errorf("Parent(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func Test新建删除重命名应正确操作文件(t *testing.T) {
	dir := t.TempDir()
	// Mkdir
	newDir := filepath.Join(dir, "新建")
	if err := Mkdir(newDir); err != nil {
		t.Fatalf("Mkdir 失败: %v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("目录未创建: %v", err)
	}
	// 写个文件再 Rename
	f := filepath.Join(newDir, "x.txt")
	_ = os.WriteFile(f, []byte("x"), 0644)
	f2 := filepath.Join(dir, "y.txt")
	if err := Rename(f, f2); err != nil {
		t.Fatalf("Rename 失败: %v", err)
	}
	if _, err := os.Stat(f2); err != nil {
		t.Fatalf("重命名后目标不存在: %v", err)
	}
	// Delete
	if err := Delete(f2); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := os.Stat(f2); err == nil {
		t.Fatal("删除后文件仍存在")
	}
}

// 确保 Windows 与 POSIX 分隔符兼容
func Test返回路径应使用系统分隔符(t *testing.T) {
	dir := setupDir(t)
	entries, _ := ListDir(dir)
	for _, e := range entries {
		if !strings.Contains(e.Path, string(filepath.Separator)) {
			t.Errorf("路径应使用系统分隔符: %s", e.Path)
		}
	}
}
