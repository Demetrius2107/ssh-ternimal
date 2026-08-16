import { useEffect, useState } from 'react';
import { ListAudit, ClearAudit, ReadHistory, ReadCommandLog } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

function fmtDur(s: number): string {
    if (s <= 0) return '-';
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    if (h > 0) return `${h}h${m}m${sec}s`;
    if (m > 0) return `${m}m${sec}s`;
    return `${sec}s`;
}

function fmtBytes(n: number): string {
    if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
    if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${n} B`;
}

// AuditPanel 会话审计: 连接记录留痕 (时间/主机/用户/协议/时长/流量) + 操作记录 (命令留痕) + 输出回放
export default function AuditPanel() {
    const [audits, setAudits] = useState<model.AuditEntry[]>([]);
    const [err, setErr] = useState('');
    const [playing, setPlaying] = useState<model.AuditEntry | null>(null);
    const [content, setContent] = useState('');
    const [loading, setLoading] = useState(false);
    const [view, setView] = useState<'replay' | 'cmd'>('replay'); // 查看模式: 输出回放 / 操作记录

    async function refresh() {
        try {
            setAudits((await ListAudit()) ?? []);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    useEffect(() => {
        refresh();
    }, []);

    async function openReplay(a: model.AuditEntry) {
        if (!a.history) {
            setErr('该会话无历史记录 (可能未启用历史落盘)');
            return;
        }
        setErr('');
        setLoading(true);
        setView('replay');
        try {
            setContent(await ReadHistory(a.history));
            setPlaying(a);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setLoading(false);
        }
    }

    // 打开操作记录 (命令留痕)
    async function openCmdLog(a: model.AuditEntry) {
        if (!a.commandLog) {
            setErr('该会话无命令记录');
            return;
        }
        setErr('');
        setLoading(true);
        setView('cmd');
        try {
            setContent(await ReadCommandLog(a.commandLog));
            setPlaying(a);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setLoading(false);
        }
    }

    async function clearAll() {
        if (!window.confirm('确认清空全部审计记录？此操作不可恢复。')) return;
        try {
            await ClearAudit();
            setAudits([]);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="audit-panel">
            <div className="audit-toolbar">
                <button onClick={refresh}>刷新</button>
                <button className="audit-clear" onClick={clearAll} disabled={audits.length === 0}>
                    清空
                </button>
                <span className="audit-count">共 {audits.length} 条连接记录</span>
            </div>
            {err && <div className="error-box">{err}</div>}
            {playing ? (
                <div className="audit-replay">
                    <div className="log-toolbar">
                        <button onClick={() => setPlaying(null)}>← 返回审计列表</button>
                        <span className="lt-info">
                            {view === 'cmd' ? '操作记录' : '输出回放'} · {playing.label} · {playing.startTime} ·{' '}
                            {playing.duration ? fmtDur(playing.duration) : '进行中'}
                        </span>
                    </div>
                    <pre className="log-content">
                        {content.split('\n').map((l, i) => (
                            <div key={i} className="log-line">
                                {l || ' '}
                            </div>
                        ))}
                    </pre>
                </div>
            ) : (
                <div className="audit-list">
                    {audits.length === 0 && <div className="hist-empty">暂无审计记录，建立连接后自动记录</div>}
                    <table className="audit-table">
                        <thead>
                            <tr>
                                <th>开始时间</th>
                                <th>主机</th>
                                <th>用户</th>
                                <th>协议</th>
                                <th>时长</th>
                                <th>流量 ↓ / ↑</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            {audits.map((a, i) => (
                                <tr key={a.id ?? i} className="audit-row">
                                    <td className="au-time">{a.startTime}</td>
                                    <td className="au-host">
                                        {a.host}:{a.port}
                                    </td>
                                    <td>{a.user}</td>
                                    <td>
                                        <span className={`au-proto au-${a.protocol}`}>{a.protocol}</span>
                                    </td>
                                    <td className="au-dur">{fmtDur(a.duration)}</td>
                                    <td className="au-bytes">
                                        ↓{fmtBytes(a.bytesIn)} ↑{fmtBytes(a.bytesOut)}
                                    </td>
                                    <td>
                                        <button
                                            className="au-replay-btn"
                                            onClick={() => openReplay(a)}
                                            disabled={!a.history || loading}
                                            title={a.history ? '回放该会话历史输出' : '无历史记录'}
                                        >
                                            回放
                                        </button>
                                        <button
                                            className="au-replay-btn au-cmd-btn"
                                            onClick={() => openCmdLog(a)}
                                            disabled={!a.commandLog || loading}
                                            title={a.commandLog ? '查看该会话操作记录 (命令留痕)' : '无命令记录'}
                                        >
                                            操作记录
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
