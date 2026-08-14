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
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

// HistoryEntry 历史记录条目
type HistoryEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}
