package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ssh-terminal/internal/model"
)

const historyMaxRead = 4 * 1024 * 1024 // 单条历史最多读取 4MB

// historyFile 会话历史日志文件
type historyFile struct {
	file *os.File
	path string
	mu   sync.Mutex
}

// historyDir 历史记录目录: %AppData%/ssh-terminal/history
func historyDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "ssh-terminal", "history")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// openHistory 为会话创建历史日志文件 (输出实时落盘)
func openHistory(label string) (*historyFile, error) {
	dir, err := historyDir()
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s.log", time.Now().Format("20060102-150405"), sanitizeLabel(label))
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &historyFile{file: f, path: filepath.Join(dir, name)}, nil
}

// sanitizeLabel 过滤文件名非法字符
func sanitizeLabel(s string) string {
	var out []rune
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ', '\n', '\r':
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}

func (h *historyFile) write(data string) {
	if h == nil || h.file == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, _ = h.file.WriteString(data)
}

func (h *historyFile) close() {
	if h == nil || h.file == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.file.Close()
	h.file = nil
}

// ListHistory 列出历史记录 (按时间倒序)
func (a *App) ListHistory() ([]model.HistoryEntry, error) {
	dir, err := historyDir()
	if err != nil {
		return nil, err
	}
	entries := []model.HistoryEntry{}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, de := range files {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".log") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		entries = append(entries, model.HistoryEntry{
			Name:    de.Name(),
			Path:    filepath.Join(dir, de.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name > entries[j].Name })
	return entries, nil
}

// ReadHistory 读取历史记录内容 (限 4MB, 防目录穿越)
func (a *App) ReadHistory(path string) (string, error) {
	dir, err := historyDir()
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, filepath.Clean(dir)+string(filepath.Separator)) {
		return "", errors.New("非法路径")
	}
	f, err := os.Open(clean)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > historyMaxRead {
		buf := make([]byte, historyMaxRead)
		n, _ := f.Read(buf)
		return string(buf[:n]) + "\n\n...(内容较大, 已截断至 4MB)...", nil
	}
	buf := make([]byte, info.Size())
	n, _ := f.Read(buf)
	return string(buf[:n]), nil
}
