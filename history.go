package main

import (
	"bufio"
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

const (
	historyMaxRead  = 4 * 1024 * 1024   // 单条历史最多读取 4MB
	historyMaxFiles = 200               // 历史文件数量上限
	historyMaxTotal = 200 * 1024 * 1024 // 历史目录总大小上限 (200MB)
)

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

// openHistory 为会话创建历史日志文件 (输出实时落盘), 并触发滚动清理
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
	cleanupHistory(dir)
	return &historyFile{file: f, path: filepath.Join(dir, name)}, nil
}

// cleanupHistory 滚动清理: 文件数或总大小超限时, 按时间(文件名倒序=最新在前)删除最旧的
// 文件名格式 "20060102-150405-label.log", 字典序即时间序, 排序后从末尾(最旧)删除
func cleanupHistory(dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var logs []os.DirEntry
	for _, de := range files {
		if !de.IsDir() && strings.HasSuffix(de.Name(), ".log") {
			logs = append(logs, de)
		}
	}
	if len(logs) == 0 {
		return
	}
	// 最新在前 (文件名带时间戳, 字典序=时间序)
	sort.Slice(logs, func(i, j int) bool { return logs[i].Name() > logs[j].Name() })

	var total int64
	for _, de := range logs {
		if info, err := de.Info(); err == nil {
			total += info.Size()
		}
	}
	// 需要删掉的数量: 超出数量上限 或 超出总大小 (从最旧的末尾开始)
	excess := len(logs) - historyMaxFiles
	for i := len(logs) - 1; i >= 0 && (excess > 0 || total > historyMaxTotal); i-- {
		de := logs[i]
		if info, err := de.Info(); err == nil {
			total -= info.Size()
		}
		_ = os.Remove(filepath.Join(dir, de.Name()))
		excess--
	}
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

// SearchHistory 在全部历史文件中检索关键字, 返回命中文件列表 (按时间倒序)
// 命中行数超过 200 即停止统计 (防超大日志卡死)
func (a *App) SearchHistory(keyword string) ([]model.HistoryMatch, error) {
	if strings.TrimSpace(keyword) == "" {
		return []model.HistoryMatch{}, nil
	}
	dir, err := historyDir()
	if err != nil {
		return nil, err
	}
	kw := strings.ToLower(keyword)
	out := []model.HistoryMatch{}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, de := range files {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".log") {
			continue
		}
		full := filepath.Join(dir, de.Name())
		m, err := scanHistory(full, kw, 200)
		if err != nil || m.Count == 0 {
			continue
		}
		m.Name = de.Name()
		m.Path = full
		out = append(out, m)
	}
	// 时间倒序 (文件名带时间戳, 字典序即时间序)
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// scanHistory 逐行扫描日志文件, 统计含关键字的行数并记录首个命中行
func scanHistory(path, kw string, max int) (model.HistoryMatch, error) {
	var m model.HistoryMatch
	f, err := os.Open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(strings.ToLower(line), kw) {
			continue
		}
		m.Count++
		if m.Preview == "" {
			m.Preview = strings.TrimSpace(line)
			if len(m.Preview) > 200 {
				m.Preview = m.Preview[:200] + "..."
			}
		}
		if m.Count >= max {
			break
		}
	}
	return m, sc.Err()
}
