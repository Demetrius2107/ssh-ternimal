// Package transfer 传输引擎: 上传/下载任务 (支持文件与目录)、进度回调、断点续传、大小校验、冲突策略、并发限制
package transfer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/sftp"

	"ssh-terminal/internal/model"
	"ssh-terminal/internal/sshcore"
)

const (
	chunkSize     = 64 * 1024
	maxConcurrent = 3 // 全局并发传输任务上限
)

// fileRef 待传输的文件
type fileRef struct {
	absPath string // 本地绝对路径 (上传) 或远程绝对路径 (下载)
	relPath string // 相对路径, posix 分隔符; 单文件为空
	size    int64
}

// Engine 传输任务管理
type Engine struct {
	mu         sync.Mutex
	nextID     uint64
	tasks      map[uint64]*model.TransferTask
	onProgress func(model.TransferTask) // 进度回调, 由 app 层接 wails 事件
	sem        chan struct{}            // 并发限制
}

// NewEngine 创建传输引擎
func NewEngine() *Engine {
	return &Engine{
		tasks: make(map[uint64]*model.TransferTask),
		sem:   make(chan struct{}, maxConcurrent),
	}
}

// SetProgressHandler 设置进度回调 (启动时调用一次)
func (e *Engine) SetProgressHandler(h func(model.TransferTask)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onProgress = h
}

// Upload 异步上传本地文件/目录到远程
func (e *Engine) Upload(sessionID uint64, sess *sshcore.Session, localPath, remotePath, conflict string) (uint64, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return 0, err
	}
	var files []fileRef
	var total int64
	isDir := info.IsDir()
	if isDir {
		files, total, err = collectLocalFiles(localPath)
		if err != nil {
			return 0, fmt.Errorf("遍历本地目录失败: %v", err)
		}
	} else {
		files = []fileRef{{absPath: localPath, size: info.Size()}}
		total = info.Size()
	}
	task := e.newTask(sessionID, "upload", localPath, remotePath, total, conflict, isDir)
	go e.runUpload(sess, task, files)
	return task.TaskID, nil
}

// Download 异步下载远程文件/目录到本地
func (e *Engine) Download(sessionID uint64, sess *sshcore.Session, remotePath, localPath, conflict string) (uint64, error) {
	c, err := sess.SFTP()
	if err != nil {
		return 0, err
	}
	info, err := c.Stat(remotePath)
	if err != nil {
		return 0, err
	}
	var files []fileRef
	var total int64
	isDir := info.IsDir()
	if isDir {
		files, total, err = collectRemoteFiles(c, remotePath)
		if err != nil {
			return 0, fmt.Errorf("遍历远程目录失败: %v", err)
		}
	} else {
		files = []fileRef{{absPath: remotePath, size: info.Size()}}
		total = info.Size()
	}
	task := e.newTask(sessionID, "download", localPath, remotePath, total, conflict, isDir)
	go e.runDownload(sess, task, files)
	return task.TaskID, nil
}

// Tasks 返回全部任务快照
func (e *Engine) Tasks() []model.TransferTask {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]model.TransferTask, 0, len(e.tasks))
	for _, t := range e.tasks {
		out = append(out, *t)
	}
	return out
}

// ---------- 任务执行 ----------

func (e *Engine) runUpload(sess *sshcore.Session, task *model.TransferTask, files []fileRef) {
	e.sem <- struct{}{} // 获取并发槽
	defer func() { <-e.sem }()
	c, err := sess.SFTP()
	if err != nil {
		e.fail(task, err)
		return
	}
	for _, f := range files {
		var target string
		if task.IsDir {
			target = path.Join(task.RemotePath, f.relPath)
			if err := c.MkdirAll(path.Dir(target)); err != nil {
				e.fail(task, fmt.Errorf("创建远程目录失败: %v", err))
				return
			}
		} else {
			target = task.RemotePath
		}
		resolved, skip, err := resolveRemoteTarget(c, target, task.Conflict)
		if err != nil {
			e.fail(task, err)
			return
		}
		if skip {
			task.Transferred += f.size
			e.emit(task)
			continue
		}
		task.CurrentFile = f.absPath
		e.emit(task)
		if err := e.uploadFile(c, f.absPath, resolved, f.size, task); err != nil {
			e.fail(task, err)
			return
		}
	}
	e.done(task)
}

func (e *Engine) runDownload(sess *sshcore.Session, task *model.TransferTask, files []fileRef) {
	e.sem <- struct{}{}
	defer func() { <-e.sem }()
	c, err := sess.SFTP()
	if err != nil {
		e.fail(task, err)
		return
	}
	for _, f := range files {
		var target string
		if task.IsDir {
			target = filepath.Join(task.LocalPath, filepath.FromSlash(f.relPath))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				e.fail(task, err)
				return
			}
		} else {
			target = task.LocalPath
		}
		resolved, skip, err := resolveLocalTarget(target, task.Conflict)
		if err != nil {
			e.fail(task, err)
			return
		}
		if skip {
			task.Transferred += f.size
			e.emit(task)
			continue
		}
		task.CurrentFile = f.absPath
		e.emit(task)
		if err := e.downloadFile(c, f.absPath, resolved, f.size, task); err != nil {
			e.fail(task, err)
			return
		}
	}
	e.done(task)
}

// uploadFile 单文件上传: 断点续传 (目标已存在且小于源) + 大小校验
func (e *Engine) uploadFile(c *sftp.Client, localPath, remotePath string, size int64, task *model.TransferTask) error {
	var offset int64
	if fi, err := c.Stat(remotePath); err == nil && fi.Size() < size {
		offset = fi.Size() // 断点续传
	}
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()
	if _, err := local.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	remote, err := c.OpenFile(remotePath, flags)
	if err != nil {
		return err
	}
	defer remote.Close()
	buf := make([]byte, chunkSize)
	for {
		n, rerr := local.Read(buf)
		if n > 0 {
			if _, werr := remote.Write(buf[:n]); werr != nil {
				return werr
			}
			task.Transferred += int64(n)
			e.emit(task)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	// 大小校验
	if fi, err := c.Stat(remotePath); err == nil && fi.Size() != size {
		return fmt.Errorf("校验失败: %s 大小不一致 (%d != %d)", remotePath, fi.Size(), size)
	}
	return nil
}

// downloadFile 单文件下载: 断点续传 (本地已存在且小于远程) + 大小校验
func (e *Engine) downloadFile(c *sftp.Client, remotePath, localPath string, size int64, task *model.TransferTask) error {
	var offset int64
	if fi, err := os.Stat(localPath); err == nil && fi.Size() < size {
		offset = fi.Size() // 断点续传
	}
	remote, err := c.Open(remotePath)
	if err != nil {
		return err
	}
	defer remote.Close()
	if _, err := remote.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	var local *os.File
	if offset > 0 {
		local, err = os.OpenFile(localPath, os.O_WRONLY|os.O_APPEND, 0600)
	} else {
		local, err = os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	}
	if err != nil {
		return err
	}
	defer local.Close()
	buf := make([]byte, chunkSize)
	for {
		n, rerr := remote.Read(buf)
		if n > 0 {
			if _, werr := local.Write(buf[:n]); werr != nil {
				return werr
			}
			task.Transferred += int64(n)
			e.emit(task)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	// 大小校验
	if fi, err := os.Stat(localPath); err == nil && fi.Size() != size {
		return fmt.Errorf("校验失败: %s 大小不一致 (%d != %d)", localPath, fi.Size(), size)
	}
	return nil
}

// ---------- 收集文件清单 ----------

func collectLocalFiles(root string) ([]fileRef, int64, error) {
	var files []fileRef
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, fileRef{absPath: p, relPath: filepath.ToSlash(rel), size: info.Size()})
		total += info.Size()
		return nil
	})
	return files, total, err
}

func collectRemoteFiles(c *sftp.Client, root string) ([]fileRef, int64, error) {
	var files []fileRef
	var total int64
	w := c.Walk(root)
	for w.Step() {
		if w.Err() != nil {
			return nil, 0, w.Err()
		}
		if w.Stat().IsDir() {
			continue
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(w.Path()))
		if err != nil {
			return nil, 0, err
		}
		files = append(files, fileRef{absPath: w.Path(), relPath: filepath.ToSlash(rel), size: w.Stat().Size()})
		total += w.Stat().Size()
	}
	return files, total, nil
}

// ---------- 冲突策略 ----------

func resolveRemoteTarget(c *sftp.Client, target, conflict string) (string, bool, error) {
	_, err := c.Stat(target)
	if err != nil && !os.IsNotExist(err) {
		return target, false, nil // stat 失败交给传输报真实错误
	}
	if err == nil {
		return resolveConflict(target, conflict, func(p string) (bool, error) {
			_, err := c.Stat(p)
			if err == nil {
				return true, nil
			}
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		})
	}
	return target, false, nil
}

func resolveLocalTarget(target, conflict string) (string, bool, error) {
	_, err := os.Stat(target)
	if err != nil && !os.IsNotExist(err) {
		return target, false, nil
	}
	if err == nil {
		return resolveConflict(target, conflict, func(p string) (bool, error) {
			_, err := os.Stat(p)
			if err == nil {
				return true, nil
			}
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		})
	}
	return target, false, nil
}

// resolveConflict 目标已存在时按策略处理; skip 返回 skip=true
func resolveConflict(target, conflict string, exists func(string) (bool, error)) (string, bool, error) {
	switch conflict {
	case "skip":
		return "", true, nil
	case "rename":
		dir := path.Dir(target)
		ext := path.Ext(target)
		base := strings.TrimSuffix(path.Base(target), ext)
		for i := 1; ; i++ {
			candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
			full := path.Join(dir, candidate)
			ok, err := exists(full)
			if err != nil {
				return "", false, err
			}
			if !ok {
				return full, false, nil
			}
		}
	default: // overwrite
		return target, false, nil
	}
}

// ---------- 任务状态 ----------

func (e *Engine) newTask(sessionID uint64, direction, local, remote string, size int64, conflict string, isDir bool) *model.TransferTask {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextID++
	task := &model.TransferTask{
		TaskID:     e.nextID,
		SessionID:  sessionID,
		Direction:  direction,
		LocalPath:  local,
		RemotePath: remote,
		Size:       size,
		Status:     "running",
		Conflict:   conflict,
		IsDir:      isDir,
	}
	e.tasks[task.TaskID] = task
	return task
}

func (e *Engine) emit(task *model.TransferTask) {
	e.mu.Lock()
	h := e.onProgress
	e.mu.Unlock()
	if h != nil {
		h(*task)
	}
}

func (e *Engine) fail(task *model.TransferTask, err error) {
	task.Status = "error"
	task.Error = err.Error()
	e.emit(task)
}

func (e *Engine) done(task *model.TransferTask) {
	task.CurrentFile = ""
	task.Status = "done"
	e.emit(task)
}
