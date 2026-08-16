import { useEffect, useRef, useState } from 'react';
import { GetSysMetrics, GetSysMetricsHistory, GetProcessList, GetDiskUsage, GetOpenPorts } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

function fmtUptime(s: number): string {
    const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (d > 0) return `${d}天${h}小时`;
    if (h > 0) return `${h}h${m}m`;
    return `${m}m`;
}

function fmtBytes(n: number): string {
    if (n >= 1024 * 1024 * 1024) return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
    if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
    if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${n} B`;
}

// ---------- SVG 折线图 (手绘, 无第三方依赖) ----------
interface ChartProps {
    data: number[];
    color: string;
    height?: number;
    unit?: string; // 数值后缀, 如 % / MB/s
    max?: number; // 纵轴上限 (默认自动)
}

function SparkChart({ data, color, height = 90, unit = '', max }: ChartProps) {
    if (data.length < 2) {
        return <div className="chart-empty" style={{ height }}>采样中...</div>;
    }
    const W = 100;
    const pad = 4;
    const maxV = max ?? (Math.max(...data) * 1.1 || 100);
    const pts = data.map((v, i) => {
        const x = pad + (i / (data.length - 1)) * (W - pad * 2);
        const y = height - pad - (Math.min(v, maxV) / maxV) * (height - pad * 2);
        return `${x.toFixed(1)},${y.toFixed(1)}`;
    });
    const last = data[data.length - 1];
    const cur = `${((W - pad) / (W - pad * 2) * (W - pad * 2) + pad).toFixed(1)},${(height - pad - (Math.min(last, maxV) / maxV) * (height - pad * 2)).toFixed(1)}`;
    return (
        <div className="chart-wrap" style={{ height }}>
            <svg viewBox={`0 0 ${W} ${height}`} preserveAspectRatio="none" className="chart-svg">
                <polyline points={pts.join(' ')} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
                <circle cx={cur.split(',')[0]} cy={cur.split(',')[1]} r="1.6" fill={color} />
            </svg>
            <span className="chart-cur" style={{ color }}>
                {last.toFixed(1)}
                {unit}
            </span>
        </div>
    );
}

// ---------- 标签页 ----------
type Tab = 'res' | 'proc' | 'disk' | 'port';

// MonitorPanel 远程主机详细监控: 资源(折线图) / 进程 / 磁盘 / 端口
// 弹窗与独立页面共用 (props: compact 控制是否显示标题栏与自动刷新间隔)
export default function MonitorPanel({ sessionId, compact }: { sessionId: number; compact?: boolean }) {
    const [m, setM] = useState<model.SysMetrics | null>(null);
    const [hist, setHist] = useState<model.SysMetrics[]>([]);
    const [inRate, setInRate] = useState(0);
    const [outRate, setOutRate] = useState(0);
    const [err, setErr] = useState('');
    const [tab, setTab] = useState<Tab>('res');
    const [procs, setProcs] = useState<model.ProcEntry[]>([]);
    const [disks, setDisks] = useState<model.DiskUsage[]>([]);
    const [ports, setPorts] = useState<model.PortInfo[]>([]);
    const prev = useRef({ in: 0, out: 0, at: Date.now() });
    const histRef = useRef<model.SysMetrics[]>([]);
    const loading = useRef(false);

    const interval = compact ? 3000 : 2000;

    // 资源轮询 + 历史折线数据
    useEffect(() => {
        const tick = async () => {
            try {
                const s = await GetSysMetrics(sessionId);
                const now = Date.now();
                const dt = (now - prev.current.at) / 1000;
                if (dt > 0) {
                    setInRate(Math.max(0, (s.netIn - prev.current.in) / dt));
                    setOutRate(Math.max(0, (s.netOut - prev.current.out) / dt));
                }
                prev.current = { in: s.netIn, out: s.netOut, at: now };
                setM(s);
                const h = await GetSysMetricsHistory(sessionId);
                histRef.current = h;
                setHist(h);
                setErr('');
            } catch (e: any) {
                setErr(e?.message ?? String(e));
            }
        };
        tick();
        const t = setInterval(tick, interval);
        return () => clearInterval(t);
    }, [sessionId, interval]);

    // 标签页懒加载
    useEffect(() => {
        if (tab === 'proc' && procs.length === 0 && !loading.current) {
            loading.current = true;
            GetProcessList(sessionId)
                .then((p) => setProcs(p ?? []))
                .catch((e: any) => setErr(e?.message ?? String(e)))
                .finally(() => (loading.current = false));
        } else if (tab === 'disk' && disks.length === 0 && !loading.current) {
            loading.current = true;
            GetDiskUsage(sessionId)
                .then((d) => setDisks(d ?? []))
                .catch((e: any) => setErr(e?.message ?? String(e)))
                .finally(() => (loading.current = false));
        } else if (tab === 'port' && ports.length === 0 && !loading.current) {
            loading.current = true;
            GetOpenPorts(sessionId)
                .then((p) => setPorts(p ?? []))
                .catch((e: any) => setErr(e?.message ?? String(e)))
                .finally(() => (loading.current = false));
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [tab]);

    const cpuPct = m ? Math.min(100, m.cpuPercent) : 0;
    const memPct = m && m.memTotal > 0 ? Math.min(100, (m.memUsed / m.memTotal) * 100) : 0;

    const cpuHist = hist.map((h) => h.cpuPercent);
    const memHist = hist.map((h) => (h.memTotal > 0 ? (h.memUsed / h.memTotal) * 100 : 0));
    const netInHist = hist.map((_, i) => (i === 0 ? 0 : Math.max(0, hist[i].netIn - hist[i - 1].netIn) / interval));
    const netOutHist = hist.map((_, i) => (i === 0 ? 0 : Math.max(0, hist[i].netOut - hist[i - 1].netOut) / interval));
    const netMax = Math.max(...netInHist, ...netOutHist, 1024);

    return (
        <div className={`monitor-panel ${compact ? 'mp-compact' : ''}`}>
            {err && <div className="error-box">{err}</div>}
            {!m && !err && <div className="hist-empty">监控数据加载中...</div>}
            {m && (
                <>
                    <div className="mon-tabs">
                        <button className={tab === 'res' ? 'mt active' : 'mt'} onClick={() => setTab('res')}>资源</button>
                        <button className={tab === 'proc' ? 'mt active' : 'mt'} onClick={() => setTab('proc')}>进程</button>
                        <button className={tab === 'disk' ? 'mt active' : 'mt'} onClick={() => setTab('disk')}>磁盘</button>
                        <button className={tab === 'port' ? 'mt active' : 'mt'} onClick={() => setTab('port')}>端口</button>
                        {!compact && (
                            <span className="mt-meta">
                                {fmtUptime(m.uptime)} · 刷新 {interval / 1000}s
                            </span>
                        )}
                    </div>

                    {tab === 'res' && (
                        <div className="mon-res">
                            <div className="mon-grid">
                                <div className="mon-card">
                                    <div className="mc-label">CPU</div>
                                    <div className="mon-bar"><div className="mon-fill" style={{ width: `${cpuPct}%` }} /></div>
                                    <div className="mc-val">{cpuPct.toFixed(1)}%</div>
                                    <SparkChart data={cpuHist} color="#0a84ff" unit="%" max={100} />
                                </div>
                                <div className="mon-card">
                                    <div className="mc-label">内存</div>
                                    <div className="mon-bar"><div className="mon-fill mem" style={{ width: `${memPct}%` }} /></div>
                                    <div className="mc-val">{fmtBytes(m.memUsed * 1024)} / {fmtBytes(m.memTotal * 1024)} ({memPct.toFixed(0)}%)</div>
                                    <SparkChart data={memHist} color="#30d158" unit="%" max={100} />
                                </div>
                                <div className="mon-card">
                                    <div className="mc-label">网络 ↓</div>
                                    <div className="mc-val">{fmtBytes(inRate)}/s · 累计 {fmtBytes(m.netIn)}</div>
                                    <SparkChart data={netInHist} color="#5e5ce6" unit="B/s" max={netMax} />
                                </div>
                                <div className="mon-card">
                                    <div className="mc-label">网络 ↑</div>
                                    <div className="mc-val">{fmtBytes(outRate)}/s · 累计 {fmtBytes(m.netOut)}</div>
                                    <SparkChart data={netOutHist} color="#ff9f0a" unit="B/s" max={netMax} />
                                </div>
                            </div>
                            {compact && (
                                <div className="mon-compact-row">
                                    <span className="mon-label">运行时长</span>
                                    <span className="mon-val">{fmtUptime(m.uptime)}</span>
                                </div>
                            )}
                        </div>
                    )}

                    {tab === 'proc' && (
                        <div className="mon-table-wrap">
                            {procs.length === 0 && <div className="hist-empty">暂无进程数据</div>}
                            <table className="mon-table">
                                <thead>
                                    <tr><th>PID</th><th>用户</th><th>CPU%</th><th>内存%</th><th>命令</th></tr>
                                </thead>
                                <tbody>
                                    {procs.map((p, i) => (
                                        <tr key={i}>
                                            <td>{p.pid}</td><td>{p.user}</td>
                                            <td className="mt-num">{p.cpu.toFixed(1)}</td>
                                            <td className="mt-num">{p.mem.toFixed(1)}</td>
                                            <td className="mt-cmd" title={p.command}>{p.command}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}

                    {tab === 'disk' && (
                        <div className="mon-table-wrap">
                            {disks.length === 0 && <div className="hist-empty">暂无磁盘数据</div>}
                            {/* 使用率总览: 按使用率降序横向条形 (图表化) */}
                            {disks.length > 0 && (
                                <div className="disk-overview">
                                    {[...disks]
                                        .sort((a, b) => b.usePct - a.usePct)
                                        .map((d, i) => (
                                            <div key={i} className="do-row" title={`${d.filesystem} ${d.used}/${d.size} · 挂载 ${d.mounted}`}>
                                                <span className="do-label">{d.mounted || d.filesystem}</span>
                                                <div className="do-bar">
                                                    <div
                                                        className={`do-fill ${d.usePct >= 90 ? 'crit' : d.usePct >= 75 ? 'warn' : ''}`}
                                                        style={{ width: `${Math.min(100, d.usePct)}%` }}
                                                    />
                                                </div>
                                                <span className="do-val">{d.usePct.toFixed(0)}%</span>
                                            </div>
                                        ))}
                                </div>
                            )}
                            <table className="mon-table">
                                <thead>
                                    <tr><th>文件系统</th><th>容量</th><th>已用</th><th>可用</th><th>使用率</th><th>挂载点</th></tr>
                                </thead>
                                <tbody>
                                    {disks.map((d, i) => (
                                        <tr key={i}>
                                            <td>{d.filesystem}</td><td>{d.size}</td><td>{d.used}</td><td>{d.avail}</td>
                                            <td>
                                                <div className="disk-cell">
                                                    <div className="disk-bar"><div className="disk-fill" style={{ width: `${Math.min(100, d.usePct)}%` }} /></div>
                                                    <span>{d.usePct.toFixed(0)}%</span>
                                                </div>
                                            </td>
                                            <td>{d.mounted}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}

                    {tab === 'port' && (
                        <div className="mon-table-wrap">
                            {ports.length === 0 && <div className="hist-empty">暂无端口数据 (可能需要权限)</div>}
                            <table className="mon-table">
                                <thead>
                                    <tr><th>协议</th><th>监听地址</th><th>端口</th><th>进程</th></tr>
                                </thead>
                                <tbody>
                                    {ports.map((p, i) => (
                                        <tr key={i}>
                                            <td>{p.protocol}</td>
                                            <td>{p.addr}</td>
                                            <td className="mt-port">{p.port}</td>
                                            <td>{p.process || '-'}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </>
            )}
        </div>
    );
}
