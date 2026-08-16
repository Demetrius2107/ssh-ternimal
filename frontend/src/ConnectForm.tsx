import { useEffect, useState } from 'react';
import {
    Connect,
    SaveSession,
    ListSessions,
    ListGroups,
    DeleteSession,
    LoadSession,
    PickFile,
    LaunchRdp,
    LaunchVnc,
    AcceptHostKey,
    MoveSession,
} from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

interface Props {
    onConnected: (id: number, label: string) => void;
    onCancel: () => void;
}

// ConnectForm 新建连接表单 (模态窗口内使用)
export default function ConnectForm({ onConnected, onCancel }: Props) {
    const [host, setHost] = useState('');
    const [port, setPort] = useState(22);
    const [username, setUsername] = useState('root');
    const [password, setPassword] = useState('');
    const [authMethod, setAuthMethod] = useState<'password' | 'key'>('password');
    const [keyPath, setKeyPath] = useState('');
    const [passphrase, setPassphrase] = useState('');
    const [protocol, setProtocol] = useState<'ssh' | 'telnet'>('ssh');
    const [otp, setOtp] = useState('');
    const [encoding, setEncoding] = useState('auto');
    const [hostKeyMode, setHostKeyMode] = useState('accept-new');
    const [useJump, setUseJump] = useState(false);
    const [jumpHost, setJumpHost] = useState('');
    const [jumpPort, setJumpPort] = useState(22);
    const [jumpUser, setJumpUser] = useState('');
    const [jumpPassword, setJumpPassword] = useState('');
    const [useProxy, setUseProxy] = useState(false);
    const [proxyType, setProxyType] = useState<'http' | 'socks5'>('http');
    const [proxyHost, setProxyHost] = useState('');
    const [proxyPort, setProxyPort] = useState(1080);
    const [proxyUser, setProxyUser] = useState('');
    const [proxyPassword, setProxyPassword] = useState('');
    const [connecting, setConnecting] = useState(false);
    const [error, setError] = useState('');

    // 已保存会话
    const [sessions, setSessions] = useState<model.StoredSession[]>([]);
    const [groups, setGroups] = useState<string[]>([]);
    const [selectedSession, setSelectedSession] = useState('');
    const [saveSession, setSaveSession] = useState(false);
    const [sessionName, setSessionName] = useState('');
    const [sessionGroup, setSessionGroup] = useState(''); // 保存会话时的分组

    async function refreshSessions() {
        try {
            setSessions((await ListSessions()) ?? []);
            setGroups((await ListGroups()) ?? []);
        } catch (e) {
            /* 存储不可用时不打扰 */
        }
    }

    useEffect(() => {
        refreshSessions();
    }, []);

    async function loadSelected() {
        if (!selectedSession) return;
        try {
            const cfg = await LoadSession(selectedSession);
            setHost(cfg.host);
            setPort(cfg.port);
            setUsername(cfg.username);
            setPassword(cfg.password);
            setEncoding(cfg.encoding ?? 'auto');
            setHostKeyMode(cfg.hostKeyMode ?? 'accept-new');
            setUseProxy(!!cfg.proxyType && !!cfg.proxyHost);
            setProxyType((cfg.proxyType as 'http' | 'socks5') || 'http');
            setProxyHost(cfg.proxyHost ?? '');
            setProxyPort(cfg.proxyPort || 1080);
            setProxyUser(cfg.proxyUser ?? '');
            setProxyPassword(cfg.proxyPassword ?? '');
            // 回填该会话所在分组 (从会话列表取, 便于继续修改)
            const s = sessions.find((x) => x.id === selectedSession);
            setSessionGroup(s?.group ?? '');
            setError('');
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    // 将选中的会话移动到指定分组 (空=未分组)
    async function moveSelected(group: string) {
        if (!selectedSession) return;
        try {
            await MoveSession(selectedSession, group);
            setSessionGroup(group);
            refreshSessions();
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    async function deleteSelected() {
        if (!selectedSession) return;
        try {
            await DeleteSession(selectedSession);
            setSelectedSession('');
            refreshSessions();
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    async function connect() {
        setConnecting(true);
        setError('');
        try {
            const cfg = new model.SshConfig({
                protocol,
                host,
                port,
                username,
                password,
                privateKey: '',
                privateKeyPath: authMethod === 'key' ? keyPath : '',
                passphrase,
                otp,
                encoding,
                hostKeyMode,
                jumpHost: useJump ? jumpHost : '',
                jumpPort,
                jumpUser: useJump ? jumpUser : '',
                jumpPassword: useJump ? jumpPassword : '',
                jumpPrivateKeyPath: '',
                jumpPassphrase: '',
                proxyType: useProxy ? proxyType : '',
                proxyHost: useProxy ? proxyHost : '',
                proxyPort,
                proxyUser: useProxy ? proxyUser : '',
                proxyPassword: useProxy ? proxyPassword : '',
            });
            const id = await Connect(cfg);
            const label = protocol === 'telnet' ? `${host}:${port}` : `${username}@${host}:${port}`;
            onConnected(id, label);
            if (saveSession) {
                const name = sessionName.trim() || `${username}@${host}`;
                SaveSession(name, host, port, username, password, encoding, hostKeyMode, useProxy ? proxyType : '', useProxy ? proxyHost : '', proxyPort, useProxy ? proxyUser : '', useProxy ? proxyPassword : '')
                    .then(async (sid) => {
                        if (sessionGroup) await MoveSession(sid, sessionGroup); // 新会话落入分组
                        refreshSessions();
                    })
                    .catch((e: any) => console.warn('保存会话失败', e));
            }
        } catch (e: any) {
            const msg = e?.message ?? String(e);
            // strict 模式首次连接: 后端返回 HOST_KEY_UNVERIFIED|host|port|fingerprint
            if (msg.startsWith('HOST_KEY_UNVERIFIED|')) {
                const parts = msg.split('|');
                const fp = parts[3] ?? '';
                const ok = window.confirm(
                    `⚠️ 主机密钥未验证\n\n${host}:${port} 是首次连接的主机。\n\nSHA256 指纹:\n${fp}\n\n是否信任该主机并继续连接?`,
                );
                if (ok) {
                    try {
                        await AcceptHostKey(host, port);
                        setError('');
                        await connect(); // 记录成功后重连
                        return;
                    } catch (e2: any) {
                        setError(e2?.message ?? String(e2));
                    }
                } else {
                    setError('已取消连接: 未信任该主机密钥');
                }
            } else {
                setError(msg);
            }
        } finally {
            setConnecting(false);
        }
    }

    async function launchRdp() {
        if (!host) return;
        try {
            await LaunchRdp(host, port, username);
            setError('');
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    async function launchVnc() {
        if (!host) return;
        try {
            await LaunchVnc(host, port);
            setError('');
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    return (
        <div className="connect-panel">
            <h1>新建连接</h1>

            <div className="session-bar">
                <select value={selectedSession} onChange={(e) => setSelectedSession(e.target.value)}>
                    <option value="">— 已保存的会话 —</option>
                    {groups.length === 0
                        ? sessions.map((s) => (
                              <option key={s.id} value={s.id}>
                                  {s.name} ({s.username}@{s.host}:{s.port})
                              </option>
                          ))
                        : groups.map((g) => (
                              <optgroup key={g} label={g}>
                                  {sessions
                                      .filter((s) => s.group === g)
                                      .map((s) => (
                                          <option key={s.id} value={s.id}>
                                              {s.name} ({s.username}@{s.host}:{s.port})
                                          </option>
                                      ))}
                              </optgroup>
                          ))}
                    {groups.length > 0 && (
                        <optgroup label="未分组">
                            {sessions
                                .filter((s) => !s.group)
                                .map((s) => (
                                    <option key={s.id} value={s.id}>
                                        {s.name} ({s.username}@{s.host}:{s.port})
                                    </option>
                                ))}
                        </optgroup>
                    )}
                </select>
                <button type="button" onClick={loadSelected} disabled={!selectedSession}>
                    加载
                </button>
                <button type="button" onClick={deleteSelected} disabled={!selectedSession}>
                    删除
                </button>
                {selectedSession && (
                    <>
                        <select value={sessionGroup} onChange={(e) => moveSelected(e.target.value)} title="移动分组">
                            <option value="">未分组</option>
                            {groups.map((g) => (
                                <option key={g} value={g}>
                                    {g}
                                </option>
                            ))}
                        </select>
                    </>
                )}
            </div>

            <form
                onSubmit={(e) => {
                    e.preventDefault();
                    connect();
                }}
            >
                <label>
                    连接类型
                    <select
                        value={protocol}
                        onChange={(e) => {
                            const p = e.target.value as 'ssh' | 'telnet';
                            setProtocol(p);
                            if (p === 'telnet' && port === 22) setPort(23);
                            if (p === 'ssh' && port === 23) setPort(22);
                        }}
                    >
                        <option value="ssh">SSH</option>
                        <option value="telnet">Telnet</option>
                    </select>
                </label>
                <label>
                    主机
                    <input
                        value={host}
                        onChange={(e) => setHost(e.target.value)}
                        placeholder="192.168.1.100"
                        required
                        autoFocus
                    />
                </label>
                <label>
                    端口
                    <input
                        type="number"
                        value={port}
                        min={1}
                        max={65535}
                        onChange={(e) => setPort(Number(e.target.value))}
                    />
                </label>
                {protocol === 'ssh' && (
                    <>
                        <label>
                            用户名
                            <input value={username} onChange={(e) => setUsername(e.target.value)} required />
                        </label>
                        <label>
                            认证方式
                            <select value={authMethod} onChange={(e) => setAuthMethod(e.target.value as 'password' | 'key')}>
                                <option value="password">密码</option>
                                <option value="key">私钥</option>
                            </select>
                        </label>
                        {authMethod === 'password' ? (
                            <label>
                                密码
                                <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
                            </label>
                        ) : (
                            <>
                                <label className="field-full">
                                    私钥文件
                                    <div className="key-row">
                                        <input
                                            value={keyPath}
                                            onChange={(e) => setKeyPath(e.target.value)}
                                            placeholder="C:\Users\xxx\.ssh\id_rsa"
                                        />
                                        <button
                                            type="button"
                                            onClick={async () => {
                                                const p = await PickFile();
                                                if (p) setKeyPath(p);
                                            }}
                                        >
                                            选择
                                        </button>
                                    </div>
                                </label>
                                <label>
                                    私钥口令（可选）
                                    <input
                                        type="password"
                                        value={passphrase}
                                        onChange={(e) => setPassphrase(e.target.value)}
                                    />
                                </label>
                            </>
                        )}
                        <label>
                            双因素验证码（可选）
                            <input value={otp} onChange={(e) => setOtp(e.target.value)} placeholder="OTP / 验证码" />
                        </label>
                        <label>
                            输出编码
                            <select value={encoding} onChange={(e) => setEncoding(e.target.value)}>
                                <option value="auto">自动识别 (UTF-8/GBK)</option>
                                <option value="utf-8">UTF-8</option>
                                <option value="gbk">GBK (Windows 服务器)</option>
                            </select>
                        </label>
                        <label>
                            主机密钥校验
                            <select value={hostKeyMode} onChange={(e) => setHostKeyMode(e.target.value)}>
                                <option value="accept-new">自动接受新主机 (默认)</option>
                                <option value="strict">首次连接需确认</option>
                                <option value="off">不校验 (不安全)</option>
                            </select>
                        </label>
                        <label className="save-session">
                            <input type="checkbox" checked={useJump} onChange={(e) => setUseJump(e.target.checked)} />
                            使用跳板机
                        </label>
                        {useJump && (
                            <>
                                <label>
                                    跳板主机
                                    <input
                                        value={jumpHost}
                                        onChange={(e) => setJumpHost(e.target.value)}
                                        placeholder="跳板机 IP"
                                    />
                                </label>
                                <label>
                                    跳板端口
                                    <input
                                        type="number"
                                        value={jumpPort}
                                        min={1}
                                        max={65535}
                                        onChange={(e) => setJumpPort(Number(e.target.value))}
                                    />
                                </label>
                                <label>
                                    跳板用户名
                                    <input
                                        value={jumpUser}
                                        onChange={(e) => setJumpUser(e.target.value)}
                                        placeholder="跳板账号"
                                    />
                                </label>
                                <label>
                                    跳板密码
                                    <input
                                        type="password"
                                        value={jumpPassword}
                                        onChange={(e) => setJumpPassword(e.target.value)}
                                    />
                                </label>
                            </>
                        )}
                        <label className="save-session">
                            <input type="checkbox" checked={useProxy} onChange={(e) => setUseProxy(e.target.checked)} />
                            使用代理 (HTTP/SOCKS5)
                        </label>
                        {useProxy && (
                            <>
                                <label>
                                    代理类型
                                    <select value={proxyType} onChange={(e) => setProxyType(e.target.value as 'http' | 'socks5')}>
                                        <option value="http">HTTP CONNECT</option>
                                        <option value="socks5">SOCKS5</option>
                                    </select>
                                </label>
                                <label>
                                    代理主机
                                    <input
                                        value={proxyHost}
                                        onChange={(e) => setProxyHost(e.target.value)}
                                        placeholder="127.0.0.1"
                                    />
                                </label>
                                <label>
                                    代理端口
                                    <input
                                        type="number"
                                        value={proxyPort}
                                        min={1}
                                        max={65535}
                                        onChange={(e) => setProxyPort(Number(e.target.value))}
                                    />
                                </label>
                                <label>
                                    代理用户名 (可选)
                                    <input
                                        value={proxyUser}
                                        onChange={(e) => setProxyUser(e.target.value)}
                                        placeholder="代理认证账号"
                                    />
                                </label>
                                <label>
                                    代理密码 (可选)
                                    <input
                                        type="password"
                                        value={proxyPassword}
                                        onChange={(e) => setProxyPassword(e.target.value)}
                                        placeholder="代理认证密码"
                                    />
                                </label>
                            </>
                        )}
                        <label className="save-session">
                            <input type="checkbox" checked={saveSession} onChange={(e) => setSaveSession(e.target.checked)} />
                            连接成功后保存到会话库
                        </label>
                        {saveSession && (
                            <>
                                <label>
                                    会话名
                                    <input
                                        value={sessionName}
                                        onChange={(e) => setSessionName(e.target.value)}
                                        placeholder="留空则用 user@host"
                                    />
                                </label>
                                <label>
                                    分组 (可选)
                                    <input
                                        value={sessionGroup}
                                        onChange={(e) => setSessionGroup(e.target.value)}
                                        placeholder="留空=未分组, 支持新建分组"
                                        list="group-options"
                                    />
                                    <datalist id="group-options">
                                        {groups.map((g) => (
                                            <option key={g} value={g} />
                                        ))}
                                    </datalist>
                                </label>
                            </>
                        )}
                    </>
                )}
                <div className="form-actions">
                    <button type="submit" disabled={connecting}>
                        {connecting ? '连接中...' : '连接'}
                    </button>
                    <button type="button" onClick={launchRdp} disabled={!host} title="用系统 mstsc 打开远程桌面">
                        🖥 远程桌面
                    </button>
                    <button type="button" onClick={launchVnc} disabled={!host} title="用系统 VNC 查看器打开 (需安装 TigerVNC/RealVNC/UltraVNC)">
                        🖥 VNC 桌面
                    </button>
                </div>
            </form>
            {error && <div className="error-box">{error}</div>}
            <div className="modal-actions">
                <button type="button" onClick={onCancel}>
                    取消
                </button>
            </div>
        </div>
    );
}
