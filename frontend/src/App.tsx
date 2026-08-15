import { useEffect, useState } from 'react';
import { SshClose, SshSend, PickFile } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import ConnectForm from './ConnectForm';
import Workspace from './Workspace';
import HomeView from './HomeView';
import TunnelPanel from './TunnelPanel';
import MonitorPanel from './MonitorPanel';
import SnippetPanel from './SnippetPanel';
import LogPanel from './LogPanel';
import SettingsPanel from './SettingsPanel';
import { THEMES, THEME_LIST, type ThemeName } from './themes';
import './App.css';

interface OpenSession {
    id: number;
    label: string;
}

// ---------- 线性 SVG 图标 (Termius 风格, 替代 emoji) ----------
const Icon = {
    plus: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
            <path d="M12 5v14M5 12h14" />
        </svg>
    ),
    history: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="9" />
            <path d="M12 7v5l3 2" />
        </svg>
    ),
    tunnel: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="6" cy="6" r="2.5" />
            <circle cx="18" cy="6" r="2.5" />
            <circle cx="12" cy="18" r="2.5" />
            <path d="M6 8.5v2a3 3 0 0 0 3 3h6a3 3 0 0 0 3-3v-2" />
        </svg>
    ),
    monitor: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M3 12h4l2.5-7 5 14L17 12h4" />
        </svg>
    ),
    terminal: (
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="4" width="18" height="16" rx="3" />
            <path d="m7 9 3 3-3 3M13 15h4" />
        </svg>
    ),
    image: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="4" width="18" height="16" rx="3" />
            <circle cx="9" cy="10" r="1.8" />
            <path d="m5 18 5-5 3 3 3-3 3 3" />
        </svg>
    ),
    snippet: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M8 6 3 12l5 6M16 6l5 6-5 6" />
        </svg>
    ),
    gear: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="3.2" />
            <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1.11-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 1 1 0-4h.09a1.7 1.7 0 0 0 1.56-1.11 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34h.09a1.7 1.7 0 0 0 1.03-1.56V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1.03 1.56h.09a1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87v.09a1.7 1.7 0 0 0 1.56 1.03H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.56 1.03z" />
        </svg>
    ),
    squares: (
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect x="3" y="3" width="8" height="8" rx="1.5" />
            <rect x="13" y="3" width="8" height="8" rx="1.5" />
            <rect x="3" y="13" width="8" height="8" rx="1.5" />
            <rect x="13" y="13" width="8" height="8" rx="1.5" />
        </svg>
    ),
};

// Windows 本地路径 → file:/// URL (CSS background 用)
function fileURL(p: string): string {
    const norm = p.replace(/\\/g, '/');
    return 'file:///' + norm.split('/').map((seg) => encodeURIComponent(seg)).join('/');
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
    const [showSnippets, setShowSnippets] = useState(false);
    const [showSettings, setShowSettings] = useState(false);
    const [splitMode, setSplitMode] = useState(false);
    const [broadcastCmd, setBroadcastCmd] = useState('');
    const [theme, setTheme] = useState<ThemeName>(() => (localStorage.getItem('theme') as ThemeName) || 'dark');
    const [bgImage, setBgImage] = useState<string>(() => localStorage.getItem('bgImage') || '');
    const [fontFamily, setFontFamily] = useState<string>(() => localStorage.getItem('fontFamily') || 'Consolas, "Courier New", monospace');
    const [fontSize, setFontSize] = useState<number>(() => Number(localStorage.getItem('fontSize')) || 14);

    useEffect(() => {
        localStorage.setItem('theme', theme);
        const t = THEMES[theme];
        document.body.classList.toggle('theme-light', t.mode === 'light');
        document.documentElement.style.setProperty('--accent', t.accent);
        document.documentElement.style.setProperty('--bg', t.chromeBg);
        document.documentElement.style.setProperty('--bar', t.chromeBar);
    }, [theme]);

    // 背景图片: 持久化 + 应用到 CSS 变量 (图片在 body 层, 内容面板之上保持半透明毛玻璃)
    useEffect(() => {
        localStorage.setItem('bgImage', bgImage);
        document.documentElement.style.setProperty('--bg-image', bgImage ? `url("${fileURL(bgImage)}")` : 'none');
        document.body.classList.toggle('has-bg', !!bgImage);
    }, [bgImage]);

    // 终端字体/字号: 持久化
    useEffect(() => {
        localStorage.setItem('fontFamily', fontFamily);
    }, [fontFamily]);
    useEffect(() => {
        localStorage.setItem('fontSize', String(fontSize));
    }, [fontSize]);

    // 选择背景图片
    async function pickBackground() {
        try {
            const p = await PickFile();
            if (p) setBgImage(p);
        } catch (e: any) {
            console.warn('选择背景图片失败', e);
        }
    }

    // 会话意外结束 (ssh-exit) 时自动移除标签; EventsOn 返回注销函数
    useEffect(() => {
        const onExit = (e: { sessionId: number }) => {
            setOpenSessions((prev) => prev.filter((s) => s.id !== e.sessionId));
            setActiveId((prev) => (prev === e.sessionId ? null : prev));
        };
        return EventsOn('ssh-exit', onExit);
    }, []);

    // ESC 关闭任意打开的功能模态框 (连接/历史/隧道/监控/片段/设置)
    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== 'Escape') return;
            if (showConnect || showHistory || showTunnel || showMonitor || showSnippets || showSettings) {
                e.preventDefault();
                setShowConnect(false);
                setShowHistory(false);
                setShowTunnel(false);
                setShowMonitor(false);
                setShowSnippets(false);
                setShowSettings(false);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [showConnect, showHistory, showTunnel, showMonitor, showSnippets, showSettings]);

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

    function openHistory() {
        setShowHistory(true);
    }

    return (
        <div className="app-root">
            <div className="session-tabbar">
                <div className="app-brand" title="ssh-terminal">
                    <span className="brand-icon">{Icon.terminal}</span>
                    <span className="brand-name">SSH 终端</span>
                </div>
                <button className="btn-new" onClick={() => setShowConnect(true)}>
                    {Icon.plus} 新建连接
                </button>
                <button className="btn-hist" onClick={openHistory}>
                    {Icon.history} 历史
                </button>
                <button className="btn-hist" onClick={() => setShowTunnel(true)}>
                    {Icon.tunnel} 隧道
                </button>
                <button className="btn-hist" onClick={() => setShowMonitor(true)}>
                    {Icon.monitor} 监控
                </button>
                <button className="btn-hist" onClick={() => setShowSnippets(true)} title="命令片段">
                    {Icon.snippet} 片段
                </button>
                <span className="tabbar-spacer" />
                {bgImage && (
                    <button
                        className="btn-hist btn-bg"
                        onClick={() => setBgImage('')}
                        title={`清除背景图片\n${bgImage}`}
                    >
                        {Icon.image} 清除背景
                    </button>
                )}
                <button className="btn-hist" onClick={pickBackground} title="选择自定义背景图片">
                    {Icon.image} 背景
                </button>
                <button className="btn-hist btn-settings" onClick={() => setShowSettings(true)} title="设置">
                    {Icon.gear}
                </button>
                {openSessions.length > 1 && (
                    <button
                        className={`btn-hist ${splitMode ? 'btn-active' : ''}`}
                        onClick={() => setSplitMode((v) => !v)}
                        title={splitMode ? '退出分屏' : '分屏显示所有会话'}
                    >
                        {Icon.squares} {splitMode ? '退出分屏' : '分屏'}
                    </button>
                )}
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
            <div className={`session-body ${splitMode ? 'split' : ''}`}>
                {splitMode && (
                    <div className="broadcast-bar">
                        <span className="bb-label">广播命令</span>
                        <input
                            value={broadcastCmd}
                            onChange={(e) => setBroadcastCmd(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter' && broadcastCmd.trim()) {
                                    openSessions.forEach((s) => SshSend(s.id, broadcastCmd + '\r'));
                                    setBroadcastCmd('');
                                }
                            }}
                            placeholder="输入命令, 回车发送到所有会话..."
                        />
                        <button
                            onClick={() => {
                                if (broadcastCmd.trim()) {
                                    openSessions.forEach((s) => SshSend(s.id, broadcastCmd + '\r'));
                                    setBroadcastCmd('');
                                }
                            }}
                        >
                            发送到全部
                        </button>
                    </div>
                )}
                {openSessions.map((s) => (
                    <div
                        key={s.id}
                        className={`session-pane ${!splitMode && s.id !== activeId ? 'hidden' : ''}`}
                    >
                        <Workspace
                            sessionId={s.id}
                            active={s.id === activeId}
                            theme={theme}
                            fontFamily={fontFamily}
                            fontSize={fontSize}
                            onClose={() => closeSession(s.id)}
                        />
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
            {showSettings && (
                <div className="modal-mask" onClick={() => setShowSettings(false)}>
                    <div className="modal settings-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">设置</h3>
                        <SettingsPanel
                            theme={theme}
                            onTheme={setTheme}
                            fontFamily={fontFamily}
                            onFontFamily={setFontFamily}
                            fontSize={fontSize}
                            onFontSize={setFontSize}
                        />
                        <div className="modal-actions">
                            <button onClick={() => setShowSettings(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
            {showHistory && (
                <div className="modal-mask" onClick={() => setShowHistory(false)}>
                    <div className="modal history-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">历史日志</h3>
                        <LogPanel />
                        <div className="modal-actions">
                            <button onClick={() => setShowHistory(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
            {showSnippets && (
                <div className="modal-mask" onClick={() => setShowSnippets(false)}>
                    <div className="modal snippet-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">命令片段</h3>
                        <SnippetPanel />
                        <div className="modal-actions">
                            <button onClick={() => setShowSnippets(false)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
            {showTunnel && (
                <div className="modal-mask" onClick={() => setShowTunnel(false)}>
                    <div className="modal tunnel-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">SSH 隧道管理</h3>
                        {openSessions.length === 0 ? (
                            <div className="hist-empty">请先建立连接，再管理 SSH 隧道</div>
                        ) : (
                            <TunnelPanel sessionId={activeId ?? (openSessions[0]?.id ?? 0)} />
                        )}
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
                        {openSessions.length === 0 ? (
                            <div className="hist-empty">请先建立连接，再查看主机监控</div>
                        ) : (
                            <MonitorPanel sessionId={activeId ?? (openSessions[0]?.id ?? 0)} />
                        )}
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
