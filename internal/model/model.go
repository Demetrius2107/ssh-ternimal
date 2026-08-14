// Package model 定义跨包共享的数据结构 (DTO)
package model

// SshConfig SSH 连接配置
type SshConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase"`
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
	Size        int64  `json:"size"`
	Transferred int64  `json:"transferred"`
	Status      string `json:"status"` // running / done / error
	Error       string `json:"error"`
}

// StoredSession 保存的会话配置 (不含密码, 密码存系统凭据库)
type StoredSession struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}
