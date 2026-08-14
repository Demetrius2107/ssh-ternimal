// Package localfs 本地文件系统操作
package localfs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ssh-terminal/internal/model"
)

// ListDir 列出本地目录, 目录在前
func ListDir(dir string) ([]model.FileEntry, error) {
	if dir == "" {
		dir = "C:\\"
	}
	infos, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]model.FileEntry, 0, len(infos))
	for _, de := range infos {
		e := model.FileEntry{
			Name:  de.Name(),
			Path:  filepath.Join(dir, de.Name()),
			IsDir: de.IsDir(),
		}
		if info, err := de.Info(); err == nil {
			e.Size = info.Size()
			e.ModTime = info.ModTime().Format("2006-01-02 15:04")
			e.Mode = info.Mode().String()
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// Parent 返回上级目录; 已是根目录则返回空串
func Parent(dir string) string {
	p := filepath.Dir(dir)
	if p == dir {
		return ""
	}
	return p
}

// Mkdir 新建目录 (含父级)
func Mkdir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// Delete 删除文件或目录 (递归)
func Delete(p string) error {
	return os.RemoveAll(p)
}

// Rename 重命名/移动
func Rename(oldP, newP string) error {
	return os.Rename(oldP, newP)
}
