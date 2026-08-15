// Package model 定义跨包共享的数据结构 (DTO)
package model

// SshConfig 连接配置
type SshConfig struct {
	Protocol       string `json:"protocol"` // ssh / telnet
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKey     string `json:"privateKey"`     // 私钥 PEM 内容
	PrivateKeyPath string `json:"privateKeyPath"` // 私钥文件路径 (UI 选择后由后端读取)
	Passphrase     string `json:"passphrase"`
	OTP            string `json:"otp"`         // 双因素验证码 (可选, keyboard-interactive 应答)
	Encoding       string `json:"encoding"`    // 输出编码: auto / utf-8 / gbk (空=auto)
	HostKeyMode    string `json:"hostKeyMode"` // 主机密钥校验: off / accept-new / strict (空=accept-new)

	// 跳板机 (ProxyJump, 可选)
	JumpHost           string `json:"jumpHost"`
	JumpPort           int    `json:"jumpPort"`
	JumpUser           string `json:"jumpUser"`
	JumpPassword       string `json:"jumpPassword"`
	JumpPrivateKeyPath string `json:"jumpPrivateKeyPath"`
	JumpPassphrase     string `json:"jumpPassphrase"`
}

// FileEntry 文件/目录条目
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Mode    string `json:"mode"`
}

// TransferTask 传输任务
type TransferTask struct {
	TaskID      uint64 `json:"taskId"`
	SessionID   uint64 `json:"sessionId"`
	Direction   string `json:"direction"` // upload / download
	LocalPath   string `json:"localPath"`
	RemotePath  string `json:"remotePath"`
	CurrentFile string `json:"currentFile"` // 当前正在传输的文件
	Size        int64  `json:"size"`
	Transferred int64  `json:"transferred"`
	Status      string `json:"status"` // running / done / error
	Error       string `json:"error"`
	Conflict    string `json:"conflict"` // overwrite / skip / rename
	IsDir       bool   `json:"isDir"`
}

// StoredSession 保存的会话配置 (不含密码, 密码存系统凭据库)
type StoredSession struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Encoding    string `json:"encoding"`    // 输出编码: auto / utf-8 / gbk (空=auto)
	HostKeyMode string `json:"hostKeyMode"` // 主机密钥校验: off / accept-new / strict (空=accept-new)
	Group       string `json:"group"`       // 分组名 (空=未分组)
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// HistoryMatch 历史检索命中结果
type HistoryMatch struct {
	Name    string `json:"name"`    // 文件名 (含时间戳前缀)
	Path    string `json:"path"`    // 完整路径 (回放/查看用)
	Count   int    `json:"count"`   // 命中行数
	Preview string `json:"preview"` // 首个命中行内容
}

// AiStatus AI 配置与用量状态 (设置面板展示)
type AiStatus struct {
	Provider      string `json:"provider"`      // deepseek / ollama
	Model         string `json:"model"`         // 当前模型档位
	MonthlyLimit  int64  `json:"monthlyLimit"`  // 月度 token 限额
	MonthUsage    int64  `json:"monthUsage"`    // 当月已用 token
	KeyConfigured bool   `json:"keyConfigured"` // DeepSeek API Key 是否已配置
}

// AiDelta AI 流式输出事件负载
type AiDelta struct {
	Text string `json:"text"`
}

// Snippet 命令片段 (快捷命令)
type Snippet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

// Tunnel SSH 端口转发 (隧道)
type Tunnel struct {
	ID         uint64 `json:"id"`
	SessionID  uint64 `json:"sessionId"`
	Type       string `json:"type"` // local / dynamic / remote
	ListenAddr string `json:"listenAddr"`
	TargetAddr string `json:"targetAddr"`
	Status     string `json:"status"` // running / stopped
}

// SysMetrics 远程主机资源指标
type SysMetrics struct {
	CPUPercent float64 `json:"cpuPercent"`
	MemUsed    uint64  `json:"memUsed"`  // KB
	MemTotal   uint64  `json:"memTotal"` // KB
	NetIn      uint64  `json:"netIn"`    // 累计接收字节
	NetOut     uint64  `json:"netOut"`   // 累计发送字节
	Uptime     float64 `json:"uptime"`   // 秒
}
