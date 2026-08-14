import { useEffect, useState } from 'react';
import { SshConnect, SaveSession, ListSessions, DeleteSession, LoadSession, PickFile } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import TerminalView from './TerminalView';
import FilePanel from './FilePanel';
import './App.css';

function App() {
    const [host, setHost] = useState('');
    const [port, setPort] = useState(22);
    const [username, setUsername] = useState('root');
    const [password, setPassword] = useState('');
    const [authMethod, setAuthMethod] = useState<'password' | 'key'>('password');
    const [keyPath, setKeyPath] = useState('');
    const [passphrase, setPassphrase] = useState('');
    const [sessionId, setSessionId] = useState<number | null>(null);
    const [connecting, setConnecting] = useState(false);
    const [error, setError] = useState('');

    // 会话管理
    const [sessions, setSessions] = useState<model.StoredSession[]>([]);
    const [selectedSession, setSelectedSession] = useState('');
    const [saveSession, setSaveSession] = useState(false);
    const [sessionName, setSessionName] = useState('');
    const [activeTab, setActiveTab] = useState<'terminal' | 'files'>('terminal');

    async function refreshSessions() {
        try {
            setSessions((await ListSessions()) ?? []);
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
            setError('');
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
                host,
                port,
                username,
                password,
                privateKey: '',
                privateKeyPath: authMethod === 'key' ? keyPath : '',
                passphrase,
            });
            const id = await SshConnect(cfg);
            setSessionId(id); // 先切换终端视图, 保存会话后台执行不阻塞 UI
            if (saveSession) {
                const name = sessionName.trim() || `${username}@${host}`;
                SaveSession(name, host, port, username, password)
                    .then(() => refreshSessions())
                    .catch((e: any) => console.warn('保存会话失败', e));
            }
        } catch (e: any) {
            setError(e?.message ?? String(e));
        } finally {
            setConnecting(false);
        }
    }

    if (sessionId !== null) {
        return (
            <div className="workspace">
                <div className="tab-bar">
                    <button className={activeTab === 'terminal' ? 'tab active' : 'tab'} onClick={() => setActiveTab('terminal')}>
                        终端
                    </button>
                    <button className={activeTab === 'files' ? 'tab active' : 'tab'} onClick={() => setActiveTab('files')}>
                        文件
                    </button>
                </div>
                <div className={`tab-pane ${activeTab === 'terminal' ? '' : 'hidden'}`}>
                    <TerminalView sessionId={sessionId} onClose={() => setSessionId(null)} />
                </div>
                <div className={`tab-pane ${activeTab === 'files' ? '' : 'hidden'}`}>
                    <FilePanel sessionId={sessionId} />
                </div>
            </div>
        );
    }

    return (
        <div className="connect-panel">
            <h1>SSH 终端</h1>

            <div className="session-bar">
                <select value={selectedSession} onChange={(e) => setSelectedSession(e.target.value)}>
                    <option value="">— 已保存的会话 —</option>
                    {sessions.map((s) => (
                        <option key={s.id} value={s.id}>
                            {s.name} ({s.username}@{s.host}:{s.port})
                        </option>
                    ))}
                </select>
                <button type="button" onClick={loadSelected} disabled={!selectedSession}>
                    加载
                </button>
                <button type="button" onClick={deleteSelected} disabled={!selectedSession}>
                    删除
                </button>
            </div>

            <form
                onSubmit={(e) => {
                    e.preventDefault();
                    connect();
                }}
            >
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
                        <label>
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
                <label className="save-session">
                    <input type="checkbox" checked={saveSession} onChange={(e) => setSaveSession(e.target.checked)} />
                    连接成功后保存到会话库
                </label>
                {saveSession && (
                    <label>
                        会话名
                        <input
                            value={sessionName}
                            onChange={(e) => setSessionName(e.target.value)}
                            placeholder="留空则用 user@host"
                        />
                    </label>
                )}
                <button type="submit" disabled={connecting}>
                    {connecting ? '连接中...' : '连接'}
                </button>
            </form>
            {error && <div className="error-box">{error}</div>}
        </div>
    );
}

export default App;
