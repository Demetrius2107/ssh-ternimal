// 定时任务引擎: 在指定会话按固定间隔执行命令
//
// 业务规则 (docs/04 3.10):
//   - 按秒间隔轮询启用任务, 到期执行命令 (复用 SshSend 发送, 输出经历史落盘)
//   - 会话断开时任务标记失败 (LastError), 会话重连后自动恢复执行
//   - 结果无需单独收集 (命令输出已随会话历史落盘, 供审计回放)
package main

import (
	"fmt"
	"time"

	"ssh-terminal/internal/store"
)

const taskTickInterval = time.Second

// startTaskEngine 启动定时任务引擎 (应用启动时调用, stopCh 关闭时退出)
func (a *App) startTaskEngine() {
	go func() {
		ticker := time.NewTicker(taskTickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.checkTasks()
			}
		}
	}()
}

// checkTasks 每秒检查全部启用任务
func (a *App) checkTasks() {
	if a.store == nil {
		return
	}
	tasks, err := a.store.ListTasks()
	if err != nil {
		return
	}
	now := time.Now()
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		// 判断是否到期
		last, _ := time.Parse("2006-01-02 15:04:05", t.LastRun)
		due := last.IsZero() || now.Sub(last) >= time.Duration(t.IntervalSeconds)*time.Second
		if !due {
			continue
		}
		a.runTask(t, now)
	}
}

// runTask 执行单个定时任务
func (a *App) runTask(t store.Task, now time.Time) {
	// 会话断开处理: 会话不存在时标记失败 (但不删除任务, 重连后自动恢复)
	if _, err := a.getSession(t.SessionID); err != nil {
		t.LastRun = now.Format("2006-01-02 15:04:05")
		t.LastError = "会话不存在或已断开"
		_, _ = a.store.SaveTask(t)
		return
	}
	// 执行命令: 走 SshSend (含命令录制, 输出随会话历史落盘, 与用户手动输入一致)
	if err := a.SshSend(t.SessionID, t.Command+"\r"); err != nil {
		t.LastRun = now.Format("2006-01-02 15:04:05")
		t.LastError = fmt.Sprintf("发送失败: %v", err)
		_, _ = a.store.SaveTask(t)
		return
	}
	t.LastRun = now.Format("2006-01-02 15:04:05")
	t.LastError = ""
	_, _ = a.store.SaveTask(t)
}

// ---------- 定时任务绑定 ----------

// SaveTask 保存定时任务 (ID 为空时新建), 返回 ID
func (a *App) SaveTask(name string, sessionID uint64, intervalSeconds int, command string, enabled bool, id string) (string, error) {
	if a.store == nil {
		return "", errStoreUninit()
	}
	if name == "" || command == "" || intervalSeconds < 5 {
		return "", fmt.Errorf("任务名、命令不能为空, 且间隔至少 5 秒")
	}
	return a.store.SaveTask(store.Task{
		ID: id, Name: name, SessionID: sessionID,
		IntervalSeconds: intervalSeconds, Command: command, Enabled: enabled,
	})
}

// ListTasks 列出全部定时任务
func (a *App) ListTasks() ([]store.Task, error) {
	if a.store == nil {
		return []store.Task{}, errStoreUninit()
	}
	return a.store.ListTasks()
}

// DeleteTask 删除定时任务
func (a *App) DeleteTask(id string) error {
	if a.store == nil {
		return errStoreUninit()
	}
	return a.store.DeleteTask(id)
}
