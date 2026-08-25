// 纯函数工具: 报错检测与主机密钥消息解析
// 抽离自 TerminalView/ConnectForm, 便于单元测试 (无 React/Wails 依赖)

// 高置信度报错检测正则: 匹配行首的标准错误格式, 避免误报
// (如 grep error 的输出、readme 含 "error" 字样不会触发)
// 要求: Error:/FATAL/Traceback 等在行首, 或 Permission denied/command not found 等完整短语
export const AI_ERROR_RE =
    /(^|\n)\s*(Error:|ERROR:|FATAL|Traceback|Exception|Permission denied|command not found|No such file or directory|Connection refused|Access denied|操作不允许|无法访问|连接超时|认证失败)/i;

export interface HostKeyPrompt {
    host: string;
    port: string;
    fingerprint: string;
}

// 解析后端在 strict 模式首次连接时返回的 HOST_KEY_UNVERIFIED|host|port|fingerprint 消息
// 非该格式返回 null
export function parseHostKeyMessage(msg: string): HostKeyPrompt | null {
    const prefix = 'HOST_KEY_UNVERIFIED|';
    if (!msg.startsWith(prefix)) return null;
    const parts = msg.split('|');
    // parts[0] 为前缀, [1] host, [2] port, [3] fingerprint
    if (parts.length < 4) return null;
    return {
        host: parts[1],
        port: parts[2],
        fingerprint: parts[3],
    };
}
