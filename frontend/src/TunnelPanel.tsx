import { useEffect, useState } from 'react';
import { ListTunnels, StartTunnel, StopTunnel } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

// TunnelPanel 隧道管理: 本地 -L / 动态 -D (SOCKS5) / 远程 -R
export default function TunnelPanel({ sessionId }: { sessionId: number }) {
    const [tunnels, setTunnels] = useState<model.Tunnel[]>([]);
    const [type, setType] = useState<'local' | 'dynamic' | 'remote'>('local');
    const [listenPort, setListenPort] = useState(8080);
    const [target, setTarget] = useState('');
    const [err, setErr] = useState('');
    const [msg, setMsg] = useState('');

    async function refresh() {
        try {
            setTunnels((await ListTunnels()) ?? []);
        } catch {
            /* 忽略 */
        }
    }

    useEffect(() => {
        refresh();
    }, []);

    async function doStart() {
        setErr('');
        setMsg('');
        try {
            await StartTunnel(sessionId, type, listenPort, target);
            setMsg('隧道已启动');
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    async function doStop(id: number) {
        try {
            await StopTunnel(id);
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="tunnel-panel">
            <div className="tunnel-form">
                <select value={type} onChange={(e) => setType(e.target.value as 'local' | 'dynamic' | 'remote')}>
                    <option value="local">本地 -L</option>
                    <option value="dynamic">动态 -D (SOCKS5)</option>
                    <option value="remote">远程 -R</option>
                </select>
                <input
                    type="number"
                    value={listenPort}
                    min={1}
                    max={65535}
                    onChange={(e) => setListenPort(Number(e.target.value))}
                    placeholder="监听端口"
                />
                <input
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    placeholder="目标 如 db:3306（动态转发可空）"
                />
                <button onClick={doStart}>启动</button>
            </div>
            {err && <div className="error-box">{err}</div>}
            {msg && <div className="tunnel-msg">{msg}</div>}
            <div className="tunnel-list">
                {tunnels.length === 0 && <div className="hist-empty">暂无隧道，选择类型并启动</div>}
                {tunnels.map((t) => (
                    <div key={t.id} className="tunnel-item">
                        <span className="ti-info">
                            [{t.type}] {t.listenAddr}
                            {t.targetAddr ? ` → ${t.targetAddr}` : ''}（{t.status === 'running' ? '运行中' : t.status}）
                        </span>
                        <button onClick={() => doStop(t.id)} disabled={t.status !== 'running'}>
                            停止
                        </button>
                    </div>
                ))}
            </div>
        </div>
    );
}
