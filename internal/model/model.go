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

	// 代理 (HTTP CONNECT / SOCKS5, 可选; 与跳板机互斥)
	ProxyType     string `json:"proxyType"` // ""(无) / http / socks5
	ProxyHost     string `json:"proxyHost"`
	ProxyPort     int    `json:"proxyPort"`
	ProxyUser     string `json:"proxyUser"`
	ProxyPassword string `json:"proxyPassword"`

	// Keychain 凭据引用 (可选): 指定后密码从集中凭据解析 (一处修改全局生效)
	CredentialID string `json:"credentialId"`
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

// StoredSession 保存的会话配置 (不含密码/私钥内容, 敏感值存系统凭据库)
type StoredSession struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Protocol     string   `json:"protocol"` // ssh / telnet (空=ssh)
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Username     string   `json:"username"`
	Encoding     string   `json:"encoding"`    // 输出编码: auto / utf-8 / gbk (空=auto)
	HostKeyMode  string   `json:"hostKeyMode"` // 主机密钥校验: off / accept-new / strict (空=accept-new)
	Group        string   `json:"group"`       // 分组名 (空=未分组)
	ProxyType    string   `json:"proxyType"`   // ""(无) / http / socks5
	ProxyHost    string   `json:"proxyHost"`
	ProxyPort    int      `json:"proxyPort"`
	ProxyUser    string   `json:"proxyUser"`
	CredentialID string   `json:"credentialId"` // 集中凭据引用 (空=使用会话自身密码)
	Tags         []string `json:"tags"`         // 标签列表 (可空, 用于筛选)

	// 私钥与跳板机 (保存后重新加载可完整还原, 不再丢失)
	PrivateKeyPath   string `json:"privateKeyPath"` // 私钥文件路径（私钥内容与口令存凭据库）
	OTP              string `json:"otp,omitempty"`  // 双因素验证码
	JumpHost         string `json:"jumpHost"`
	JumpPort         int    `json:"jumpPort"`
	JumpUser         string `json:"jumpUser"`
	JumpPrivateKeyPath string `json:"jumpPrivateKeyPath"`
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
// SessionID: 内联报错解释 (ai-explain-*) 标记所属会话, 供前端过滤; AI 面板 (ai-*) 为 0
type AiDelta struct {
	Text      string `json:"text"`
	SessionID uint64 `json:"sessionId"`
}

// UpdateInfo 客户端更新信息 (CheckUpdate 返回)
type UpdateInfo struct {
	LatestVersion string `json:"latestVersion"` // 最新版本号, 如 v0.9.1
	DownloadURL   string `json:"downloadUrl"`   // 安装包下载地址
	Notes         string `json:"notes"`         // 更新说明 (发布说明)
	HasUpdate     bool   `json:"hasUpdate"`     // 是否有新版本
}

// AuditEntry 会话审计条目 (操作留痕, 内网运维/审计场景)
type AuditEntry struct {
	ID         string `json:"id"`
	StartTime  string `json:"startTime"` // 连接开始 "2006-01-02 15:04:05"
	EndTime    string `json:"endTime"`   // 连接结束 (空=进行中)
	Duration   int64  `json:"duration"`  // 时长 (秒, 0=进行中)
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Protocol   string `json:"protocol"` // ssh / telnet
	BytesIn    int64  `json:"bytesIn"`
	BytesOut   int64  `json:"bytesOut"`
	History    string `json:"history"`    // 历史日志文件路径 (回放用, 空=无)
	CommandLog string `json:"commandLog"` // 命令录制文件路径 (.cmd.log, 操作留痕, 空=无)
	Label      string `json:"label"`      // 会话标签 user@host:port
}

// Credential 集中凭据 (Keychain): 多个会话可引用同一身份, 一处修改全局生效
type Credential struct {
	ID       string `json:"id"`
	Name     string `json:"name"`     // 凭据名, 如 "生产环境 root"
	Type     string `json:"type"`     // password / privateKey
	Username string `json:"username"` // 登录用户
	// Secret 密码内容或私钥内容 (不落库, 存系统凭据库; 列表接口不返回)
	CreatedAt string `json:"createdAt"`
}

// CredentialListEntry 凭据列表条目 (不含 secret, 供 UI 展示)
type CredentialListEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Username  string `json:"username"`
	CreatedAt string `json:"createdAt"`
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

// ProcEntry 进程信息 (top 实时状态)
type ProcEntry struct {
	PID     string  `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu"` // %
	Mem     float64 `json:"mem"` // %
	Command string  `json:"command"`
}

// DiskUsage 磁盘分区占比
type DiskUsage struct {
	Filesystem string  `json:"filesystem"`
	Size       string  `json:"size"`
	Used       string  `json:"used"`
	Avail      string  `json:"avail"`
	UsePct     float64 `json:"usePct"`  // 使用率 %
	Mounted    string  `json:"mounted"` // 挂载点
}

// PortInfo 监听端口
type PortInfo struct {
	Protocol string `json:"protocol"` // tcp / udp / tcp6
	Addr     string `json:"addr"`     // 监听地址
	Port     string `json:"port"`
	Process  string `json:"process"` // 进程名 (可空)
}
