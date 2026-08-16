// 监控告警引擎: CPU/内存/磁盘阈值轮询 + 去重 (firing→recovered 状态机) + 通知
//
// 业务规则 (docs/04 3.9):
//   - 超阈值触发一次告警, 持续超限不重复 (去重)
//   - 恢复 (低于阈值) 发送恢复通知
//   - 通知渠道: Windows 系统通知 (go-toast) + 可选钉钉/自定义 webhook
package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-terminal/internal/store"
)

const alertPollInterval = 10 * time.Second

// alertState 各会话各指标的告警状态 (去重): key = "会话ID:指标"
type alertState struct {
	firing map[string]bool
}

// startAlertEngine 启动告警轮询引擎 (应用启动时调用, 常驻运行)
func (a *App) startAlertEngine() {
	as := &alertState{firing: map[string]bool{}}
	go func() {
		for {
			time.Sleep(alertPollInterval)
			a.checkAlerts(as)
		}
	}()
}

// checkAlerts 对全部活动会话执行一次阈值检查
func (a *App) checkAlerts(as *alertState) {
	if a.store == nil {
		return
	}
	cfg, err := a.store.GetAlertConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	// 快照活动会话 (避免遍历中 map 变更)
	a.mu.Lock()
	ids := make([]uint64, 0, len(a.sessions))
	for id := range a.sessions {
		ids = append(ids, id)
	}
	a.mu.Unlock()

	for _, id := range ids {
		label := a.sessionLabel(id)
		if label == "" {
			continue
		}
		cpu, memPct, diskPct := a.sampleUsage(id)
		// 逐指标判断 (阈值 >0 才启用该指标)
		a.evalMetric(as, id, label, "cpu", cpu, cfg.CpuThreshold)
		a.evalMetric(as, id, label, "mem", memPct, cfg.MemThreshold)
		a.evalMetric(as, id, label, "disk", diskPct, cfg.DiskThreshold)
	}
}

// sampleUsage 采集一次 CPU%/内存%/磁盘最高使用率
func (a *App) sampleUsage(id uint64) (cpu, memPct, diskPct float64) {
	if m, err := a.GetSysMetrics(id); err == nil {
		cpu = m.CPUPercent
		if m.MemTotal > 0 {
			memPct = float64(m.MemUsed) / float64(m.MemTotal) * 100
		}
	}
	if disks, err := a.GetDiskUsage(id); err == nil {
		for _, d := range disks {
			if d.UsePct > diskPct {
				diskPct = d.UsePct
			}
		}
	}
	return
}

// evalMetric 单指标评估: 超阈值触发 (去重), 恢复发送恢复通知
func (a *App) evalMetric(as *alertState, id uint64, label, metric string, value, threshold float64) {
	if threshold <= 0 {
		return // 该指标未启用
	}
	key := fmt.Sprintf("%d:%s", id, metric)
	if value >= threshold {
		if !as.firing[key] {
			as.firing[key] = true
			a.fireAlert(label, metric, value, threshold, "alert")
		}
		return
	}
	if as.firing[key] {
		as.firing[key] = false
		a.fireAlert(label, metric, value, threshold, "recovery")
	}
}

// fireAlert 触发通知 + 记录历史
func (a *App) fireAlert(label, metric string, value, threshold float64, typ string) {
	metricName := map[string]string{"cpu": "CPU", "mem": "内存", "disk": "磁盘"}[metric]
	msg := fmt.Sprintf("%s %s: %.1f%% (阈值 %.0f%%)", label, metricName, value, threshold)
	if typ == "recovery" {
		msg = fmt.Sprintf("%s %s 已恢复: %.1f%%", label, metricName, value)
	}
	// 系统通知
	a.notifySystem(metricName, msg)
	// webhook 通知
	if cfg, err := a.store.GetAlertConfig(); err == nil && cfg.WebhookURL != "" {
		a.notifyWebhook(cfg.WebhookURL, msg)
	}
	// 历史记录
	_ = a.store.AddAlert(store.AlertRecord{
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		Session:   label,
		Metric:    metric,
		Value:     value,
		Threshold: threshold,
		Type:      typ,
	})
}

// notifySystem Windows 系统通知 (go-toast v2)
func (a *App) notifySystem(title, msg string) {
	n := toast.Notification{
		AppID: "ssh-terminal",
		Title: "ssh-terminal 告警 · " + title,
		Body:  msg,
	}
	_ = n.Push()
}

// notifyWebhook 钉钉/自定义 webhook (POST JSON, 钉钉 text 格式)
func (a *App) notifyWebhook(url, msg string) {
	body := fmt.Sprintf(`{"msgtype":"text","text":{"content":"%s"}}`, msg)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	_, _ = client.Do(req)
}

// sessionLabel 会话标签 (user@host:port)
func (a *App) sessionLabel(id uint64) string {
	a.mu.Lock()
	cfg, ok := a.connConfigs[id]
	a.mu.Unlock()
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s@%s:%d", cfg.Username, cfg.Host, cfg.Port)
}

// ---------- 告警绑定 ----------

// SaveAlertConfig 保存告警配置
func (a *App) SaveAlertConfig(enabled bool, cpu, mem, disk float64, webhookURL string) error {
	if a.store == nil {
		return errStoreUninit()
	}
	return a.store.SaveAlertConfig(store.AlertConfig{
		Enabled: enabled, CpuThreshold: cpu, MemThreshold: mem,
		DiskThreshold: disk, WebhookURL: webhookURL,
	})
}

// GetAlertConfig 读取告警配置
func (a *App) GetAlertConfig() (store.AlertConfig, error) {
	if a.store == nil {
		return store.AlertConfig{}, errStoreUninit()
	}
	return a.store.GetAlertConfig()
}

// ListAlerts 列出告警历史
func (a *App) ListAlerts() ([]store.AlertRecord, error) {
	if a.store == nil {
		return []store.AlertRecord{}, errStoreUninit()
	}
	return a.store.ListAlerts()
}

// ClearAlerts 清空告警历史
func (a *App) ClearAlerts() error {
	if a.store == nil {
		return errStoreUninit()
	}
	return a.store.ClearAlerts()
}

// TestAlert 发送一条测试通知 (验证通知渠道)
func (a *App) TestAlert(msg string) error {
	if msg == "" {
		msg = "测试通知: 告警渠道正常 ✅"
	}
	a.notifySystem("测试", msg)
	runtime.EventsEmit(a.ctx, "alert-test", msg)
	return nil
}

func errStoreUninit() error { return errors.New("会话存储未初始化") }
