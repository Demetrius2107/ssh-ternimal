import { useEffect, useState } from 'react';
import {
    ListSessions,
    ListHistory,
    LoadSession,
    Connect,
    ReadHistory,
} from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

interface Props {
    onConnected: (id: number, label: string) => void;
    onNewConnect: () => void;
}

function formatSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// HomeView 首页: 快速连接 (已保存会话) + 最近历史 + 新建连接入口
export default function HomeView({ onConnected, onNewConnect }: Props) {
    const [sessions, setSessions] = useState<model.StoredSession[]>([]);
    const [history, setHistory] = useState<model.HistoryEntry[]>([]);
    const [connecting, setConnecting] = useState<string | null>(null);
    const [error, setError] = useState('');
    const [viewing, setViewing] = useState<model.HistoryEntry | null>(null);
    const [content, setContent] = useState('');

    useEffect(() => {
        Promise.all([ListSessions().catch(() => []), ListHistory().catch(() => [])]).then(([s, h]) => {
            setSessions(s ?? []);
            setHistory((h ?? []).slice(0, 10));
        });
    }, []);

    // ESC 关闭历史内容查看弹窗
    useEffect(() => {
        if (!viewing) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                setViewing(null);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [viewing]);

    async function quickConnect(id: string) {
        setConnecting(id);
        setError('');
        try {
            const cfg = await LoadSession(id);
            const newId = await Connect(cfg);
            const label = cfg.username ? `${cfg.username}@${cfg.host}:${cfg.port}` : `${cfg.host}:${cfg.port}`;
            onConnected(newId, label);
        } catch (e: any) {
            setError(e?.message ?? String(e));
            setConnecting(null);
        }
    }

    async function viewHistory(h: model.HistoryEntry) {
        try {
            setContent(await ReadHistory(h.path));
            setViewing(h);
        } catch (e: any) {
            setError(e?.message ?? String(e));
        }
    }

    return (
        <div className="home">
            <div className="home-hero">
                <h1>SSH 终端</h1>
                <p className="home-sub">多协议终端 · SFTP 文件管理</p>
                <button className="home-new" onClick={onNewConnect}>
                    ＋ 新建连接
                </button>
            </div>

            {error && <div className="error-box home-error">{error}</div>}

            <div className="home-section">
                <h2>快速连接</h2>
                {sessions.length === 0 ? (
                    <div className="home-empty">
                        暂无保存的会话。点击「新建连接」连接成功后勾选"保存到会话库"，即可在这里一键直连
                    </div>
                ) : (
                    <div className="home-grid">
                        {sessions.map((s) => (
                            <div key={s.id} className="home-card" onClick={() => quickConnect(s.id)}>
                                <div className="hc-top">
                                    <span className="hc-name">{s.name}</span>
                                    {connecting === s.id && <span className="hc-loading">连接中...</span>}
                                </div>
                                <div className="hc-meta">
                                    {s.username}@{s.host}:{s.port}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            <div className="home-section">
                <h2>最近历史</h2>
                {history.length === 0 ? (
                    <div className="home-empty">暂无历史记录，会话输出会自动保存</div>
                ) : (
                    <div className="home-hist">
                        {history.map((h) => (
                            <div key={h.path} className="home-hist-item" onClick={() => viewHistory(h)}>
                                <span className="hh-name">{h.name}</span>
                                <span className="hh-meta">
                                    {h.modTime} · {formatSize(h.size)}
                                </span>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {viewing && (
                <div className="modal-mask" onClick={() => setViewing(null)}>
                    <div className="modal history-modal" onClick={(e) => e.stopPropagation()}>
                        <h3 className="modal-title">{viewing.name}</h3>
                        <div className="hist-viewer">
                            <pre className="hist-content">{content}</pre>
                        </div>
                        <div className="modal-actions">
                            <button onClick={() => setViewing(null)}>关闭</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
