import { useEffect, useState } from 'react';
import { SshClose, ListHistory, ReadHistory } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import ConnectForm from './ConnectForm';
import Workspace from './Workspace';
import HomeView from './HomeView';
import TunnelPanel from './TunnelPanel';
import MonitorPanel from './MonitorPanel';
import { THEMES, THEME_LIST, type ThemeName } from './themes';
import './App.css';

interface OpenSession {
    id: number;
    label: string;
}

function formatSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// App 根组件: 会话标签栏 + 多会话工作区 + 新建连接模态
function App() {
    const [openSessions, setOpenSessions] = useState<OpenSession[]>([]);
    const [activeId, setActiveId] = useState<number | null>(null);
    const [showConnect, setShowConnect] = useState(false);
    const [showHistory, setShowHistory] = useState(false);
    const [showTunnel, setShowTunnel] = useState(false);
    const [showMonitor, setShowMonitor] = useState(false);
    const [historyList, setHistoryList] = useState<model.HistoryEntry[]>([]);
    const [historyContent, setHistoryContent] = useState<string | null>(null);
    const [historyErr, setHistoryErr] = useState('');
    const [theme, setTheme] = useState<ThemeName>(() => (localStorage.getItem('theme') as ThemeName) || 'dark');

    useEffect(() => {
        localStorage.setItem('theme', theme);
        const t = THEMES[theme];
        document.body.classList.toggle('theme-light', t.mode === 'light');
        document.documentElement.style.setProperty('--accent', t.accent);
        document.documentElement.style.setProperty('--bg', t.chromeBg);
        document.documentElement.style.setProperty('--bar', t.chromeBar);
    }, [theme]);

    // 会话意外结束 (ssh-exit) 时自动移除标签; EventsOn 返回注销函数
    useEffect(() => {
        const onExit = (e: { sessionId: number }) => {
            setOpenSessions((prev) => prev.filter((s) => s.id !== e.sessionId));
            setActiveId((prev) => (prev === e.sessionId ? null : prev));
        };
        return EventsOn('ssh-exit', onExit);
    }, []);

    function closeSession(id: number) {
        SshClose(id);
        setOpenSessions((prev) => prev.filter((s) => s.id !== id));
        setActiveId((prev) => (prev === id ? null : prev));
    }

    function onConnected(id: number, label: string) {
        setOpenSessions((prev) => [...prev, { id, label }]);
        setActiveId(id);
        setShowConnect(false);
    }

    async function openHistory() {
        setShowHistory(true);
        setHistoryContent(null);
        setHistoryErr('');
        try {
            setHistoryList((await ListHistory()) ?? []);
        } catch (e: any) {
            setHistoryErr(e?.message ?? String(e));
        }
    }

    async function viewHistory(entry: model.HistoryEntry) {
        try {
            setHistoryContent(await ReadHistory(entry.path));
        } catch (e: any) {
            setHistoryErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="app-root">
            <div className="session-tabbar">
                <button className="btn-new" onClick={() => setShowConnect(true)}>
                    ＋ 新建连接
                </button>
                <button className="btn-hist" onClick={openHistory}>
                    🕘 历史
                </button>
                <button className="btn-hist" onClick={() => setShowTunnel(true)} disabled={openSessions.length === 0}>
                    🚇 隧道
                </button>
                <button className="btn-hist" onClick={() => setShowMonitor(true)} disabled={openSessions.length === 0}>
                    📊 监控
                </button>
                <select
                    className="theme-select"
                    value={theme}
                    onChange={(e) => setTheme(e.target.value as ThemeName)}
                    title="主题"
                >
                    {THEME_LIST.map((t) => (
                        <option key={t} value={t}>
                            {THEMES[t].label}
                        </option>
                    ))}
                </select>
                {openSessions.map((s) => (
                    <div
                        key={s.id}
                        className={`session-tab ${s.id === activeId ? 'active' : ''}`}
                        onClick={() => setActiveId(s.id)}
                        title={s.label}
                    >
                        <span className="st-label">{s.label}</span>
                        <button
                            className="tab-x"
                            onClick={(e) => {
                                e.stopPropagation();
                                closeSession(s.id);
                            }}
                        >
                            ×
                        </button>
                    </div>
                ))}
            </div>
            <div className="session-body">
                {openSessions.map((s) => (
                    <div key={s.id} className={`session-pane ${s.id === activeId ? '' : 'hidden'}`}>
                        <Workspace sessionId={s.id} active={s.id === activeId} theme={theme} onClose={() => closeSession(s.id)} />
                    </div>
                ))}
                {openSessions.length === 0 && (
                    <HomeView onConnected={onConnected} onNewConnect={() => setShowConnect(true)} />
                )}
            </div>
            {showConnect && (
                <div className="modal-mask" onClick={() => setShowConnect(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <ConnectForm onConnected={onConnected} onCancel={() => setShowConnect(false)} />
                    </div>
                </div>
            )}
            {showHistory && (
                <div className="modal-mask" onClick={() => setShowHistory(false)}>
                    <div className="modal history-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">历史记录</h3>
                        {historyErr && <div className="error-box">{historyErr}</div>}
                        {historyContent === null ? (
                            <div className="hist-list">
                                {historyList.length === 0 && <div className="hist-empty">暂无历史记录</div>}
                                {historyList.map((h) => (
                                    <div key={h.path} className="hist-item" onClick={() => viewHistory(h)}>
                                        <span className="hi-name">{h.name}</span>
                                        <span className="hi-meta">
                                            {h.modTime} {formatSize(h.size)}
                                        </span>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="hist-viewer">
                                <pre className="hist-content">{historyContent}</pre>
                            </div>
                        )}
                        <div className="modal-actions">
                            {historyContent !== null && (
                                <button onClick={() => setHistoryContent(null)}>返回列表</button>
                            )}
                            <button onClick={() => setShowHistory(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
            {showTunnel && (
                <div className="modal-mask" onClick={() => setShowTunnel(false)}>
                    <div className="modal tunnel-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">SSH 隧道管理</h3>
                        <TunnelPanel sessionId={activeId ?? (openSessions[0]?.id ?? 0)} />
                        <div className="modal-actions">
                            <button onClick={() => setShowTunnel(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
            {showMonitor && (
                <div className="modal-mask" onClick={() => setShowMonitor(false)}>
                    <div className="modal monitor-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">远程主机监控</h3>
                        <MonitorPanel sessionId={activeId ?? (openSessions[0]?.id ?? 0)} />
                        <div className="modal-actions">
                            <button onClick={() => setShowMonitor(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export default App;
