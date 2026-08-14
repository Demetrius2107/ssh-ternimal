import { useEffect, useRef, useState } from 'react';
import { GetSysMetrics } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

function fmtUptime(s: number): string {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    return h > 0 ? `${h}h${m}m` : `${m}m`;
}

function fmtBytes(n: number): string {
    if (n >= 1024 * 1024 * 1024) return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
    if (n >= 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
    if (n >= 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${n} B`;
}

// MonitorPanel 远程主机资源监控 (CPU/内存/网络/运行时长), 每 2s 轮询
export default function MonitorPanel({ sessionId }: { sessionId: number }) {
    const [m, setM] = useState<model.SysMetrics | null>(null);
    const [inRate, setInRate] = useState(0);
    const [outRate, setOutRate] = useState(0);
    const [err, setErr] = useState('');
    const prev = useRef({ in: 0, out: 0, at: Date.now() });

    useEffect(() => {
        const t = setInterval(async () => {
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
                setErr('');
            } catch (e: any) {
                setErr(e?.message ?? String(e));
            }
        }, 2000);
        return () => clearInterval(t);
    }, [sessionId]);

    const cpu = m ? Math.min(100, Math.round(m.cpuPercent)) : 0;
    const memPct = m && m.memTotal > 0 ? Math.min(100, Math.round((m.memUsed / m.memTotal) * 100)) : 0;

    return (
        <div className="monitor-panel">
            {err && <div className="error-box">{err}</div>}
            {!m && !err && <div className="hist-empty">监控数据加载中...</div>}
            {m && (
                <>
                    <div className="mon-row">
                        <span className="mon-label">CPU</span>
                        <div className="mon-bar">
                            <div className="mon-fill" style={{ width: `${cpu}%` }} />
                        </div>
                        <span className="mon-val">{cpu}%</span>
                    </div>
                    <div className="mon-row">
                        <span className="mon-label">内存</span>
                        <div className="mon-bar">
                            <div className="mon-fill mem" style={{ width: `${memPct}%` }} />
                        </div>
                        <span className="mon-val">
                            {fmtBytes(m.memUsed * 1024)} / {fmtBytes(m.memTotal * 1024)} ({memPct}%)
                        </span>
                    </div>
                    <div className="mon-row">
                        <span className="mon-label">网络</span>
                        <span className="mon-val">
                            ↓ {fmtBytes(inRate)}/s　↑ {fmtBytes(outRate)}/s　累计 ↓{fmtBytes(m.netIn)} ↑{fmtBytes(m.netOut)}
                        </span>
                    </div>
                    <div className="mon-row">
                        <span className="mon-label">运行时长</span>
                        <span className="mon-val">{fmtUptime(m.uptime)}</span>
                    </div>
                </>
            )}
        </div>
    );
}
