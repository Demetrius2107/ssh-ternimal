// Package transfer 传输引擎: 上传/下载任务、进度回调、断点续传
package transfer

import (
	"errors"
	"io"
	"os"
	"sync"

	"ssh-terminal/internal/model"
	"ssh-terminal/internal/sshcore"
)

const chunkSize = 64 * 1024

// Engine 传输任务管理
type Engine struct {
	mu         sync.Mutex
	nextID     uint64
	tasks      map[uint64]*model.TransferTask
	onProgress func(model.TransferTask) // 进度回调, 由 app 层接 wails 事件
}

// NewEngine 创建传输引擎
func NewEngine() *Engine {
	return &Engine{tasks: make(map[uint64]*model.TransferTask)}
}

// SetProgressHandler 设置进度回调 (启动时调用一次)
func (e *Engine) SetProgressHandler(h func(model.TransferTask)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onProgress = h
}

// Upload 异步上传本地文件到远程
func (e *Engine) Upload(sessionID uint64, sess *sshcore.Session, localPath, remotePath string) (uint64, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, errors.New("暂不支持上传目录")
	}
	task := e.newTask(sessionID, "upload", localPath, remotePath, info.Size())
	go e.runUpload(sess, task)
	return task.TaskID, nil
}

// Download 异步下载远程文件到本地
func (e *Engine) Download(sessionID uint64, sess *sshcore.Session, remotePath, localPath string) (uint64, error) {
	c, err := sess.SFTP()
	if err != nil {
		return 0, err
	}
	info, err := c.Stat(remotePath)
	if err != nil {
		return 0, err
	}
	if info.IsDir() {
		return 0, errors.New("暂不支持下载目录")
	}
	task := e.newTask(sessionID, "download", localPath, remotePath, info.Size())
	go e.runDownload(sess, task)
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

func (e *Engine) newTask(sessionID uint64, direction, local, remote string, size int64) *model.TransferTask {
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
	}
	e.tasks[task.TaskID] = task
	return task
}

// runUpload 上传执行: 远程已存在且小于本地时断点续传
func (e *Engine) runUpload(sess *sshcore.Session, task *model.TransferTask) {
	c, err := sess.SFTP()
	if err != nil {
		e.fail(task, err)
		return
	}
	var offset int64
	if fi, err := c.Stat(task.RemotePath); err == nil && fi.Size() < task.Size {
		offset = fi.Size() // 断点续传
	}
	local, err := os.Open(task.LocalPath)
	if err != nil {
		e.fail(task, err)
		return
	}
	defer local.Close()
	if _, err := local.Seek(offset, io.SeekStart); err != nil {
		e.fail(task, err)
		return
	}
	flags := os.O_WRONLY | os.O_CREATE
	if offset == 0 {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	remote, err := c.OpenFile(task.RemotePath, flags)
	if err != nil {
		e.fail(task, err)
		return
	}
	buf := make([]byte, chunkSize)
	for {
		n, rerr := local.Read(buf)
		if n > 0 {
			if _, werr := remote.Write(buf[:n]); werr != nil {
				remote.Close()
				e.fail(task, werr)
				return
			}
			task.Transferred += int64(n)
			e.emit(task)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			remote.Close()
			e.fail(task, rerr)
			return
		}
	}
	if err := remote.Close(); err != nil {
		e.fail(task, err)
		return
	}
	e.done(task)
}

// runDownload 下载执行: 本地已存在且小于远程时断点续传
func (e *Engine) runDownload(sess *sshcore.Session, task *model.TransferTask) {
	c, err := sess.SFTP()
	if err != nil {
		e.fail(task, err)
		return
	}
	var offset int64
	if fi, err := os.Stat(task.LocalPath); err == nil && fi.Size() < task.Size {
		offset = fi.Size() // 断点续传
	}
	remote, err := c.Open(task.RemotePath)
	if err != nil {
		e.fail(task, err)
		return
	}
	defer remote.Close()
	if _, err := remote.Seek(offset, io.SeekStart); err != nil {
		e.fail(task, err)
		return
	}
	var local *os.File
	if offset > 0 {
		local, err = os.OpenFile(task.LocalPath, os.O_WRONLY|os.O_APPEND, 0600)
	} else {
		local, err = os.OpenFile(task.LocalPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	}
	if err != nil {
		e.fail(task, err)
		return
	}
	defer local.Close()
	buf := make([]byte, chunkSize)
	for {
		n, rerr := remote.Read(buf)
		if n > 0 {
			if _, werr := local.Write(buf[:n]); werr != nil {
				e.fail(task, werr)
				return
			}
			task.Transferred += int64(n)
			e.emit(task)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			e.fail(task, rerr)
			return
		}
	}
	e.done(task)
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
	task.Status = "done"
	e.emit(task)
}
