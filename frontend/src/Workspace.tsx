import { useEffect, useRef, useState } from 'react';
import { GetSessionMetrics, SshKeepAlive } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import TerminalView, { ThemeName } from './TerminalView';
import FilePanel from './FilePanel';
import AiPanel from './AiPanel';

interface Props {
    sessionId: number;
    active: boolean;
    theme: ThemeName;
    fontFamily: string;
    fontSize: number;
    onClose: () => void;
    onOpenSettings?: () => void; // AI 配置入口 (⚙ 打开设置弹窗)
}

function fmtDur(s: number): string {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const sec = s % 60;
    return h > 0 ? `${h}h${m}m` : `${m}m${sec}s`;
}

function fmtRate(bps: number): string {
    if (bps >= 1024 * 1024) return `${(bps / 1024 / 1024).toFixed(1)} MB/s`;
    if (bps >= 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
    return `${Math.round(bps)} B/s`;
}

// Workspace 单个会话的工作区: 终端/文件 子标签页 + 底部网络状态栏
export default function Workspace({ sessionId, active, theme, fontFamily, fontSize, onClose, onOpenSettings }: Props) {
    const [activeTab, setActiveTab] = useState<'terminal' | 'files' | 'ai'>('terminal');
    const [metrics, setMetrics] = useState<model.Metrics | null>(null);
    const [rate, setRate] = useState({ in: 0, out: 0 });
    const [duration, setDuration] = useState(0);
    const [kaMsg, setKaMsg] = useState('');
    const startedAt = useRef(Date.now());
    const prev = useRef({ bytesIn: 0, bytesOut: 0, at: Date.now() });

    // 实时指标轮询 (激活会话每 2s)
    useEffect(() => {
        if (!active) return;
        const t = setInterval(async () => {
            try {
                const m = await GetSessionMetrics(sessionId);
                const now = Date.now();
                const dt = (now - prev.current.at) / 1000;
                if (dt > 0) {
                    setRate({
                        in: Math.max(0, (m.bytesIn - prev.current.bytesIn) / dt),
                        out: Math.max(0, (m.bytesOut - prev.current.bytesOut) / dt),
                    });
                }
                prev.current = { bytesIn: m.bytesIn, bytesOut: m.bytesOut, at: now };
                setMetrics(m);
                setDuration(Math.floor((now - startedAt.current) / 1000));
            } catch {
                /* 会话已关闭 */
            }
        }, 2000);
        return () => clearInterval(t);
    }, [active, sessionId]);

    async function doKeepAlive() {
        setKaMsg('保活中...');
        try {
            const ms = await SshKeepAlive(sessionId);
            setKaMsg(ms > 0 ? `RTT ${ms}ms` : '已发送');
        } catch (e: any) {
            setKaMsg(e?.message ?? '失败');
        }
    }

    return (
        <div className="workspace">
            <div className="tab-bar">
                <button className={activeTab === 'terminal' ? 'tab active' : 'tab'} onClick={() => setActiveTab('terminal')}>
                    终端
                </button>
                <button className={activeTab === 'files' ? 'tab active' : 'tab'} onClick={() => setActiveTab('files')}>
                    文件
                </button>
                <button className={activeTab === 'ai' ? 'tab active' : 'tab'} onClick={() => setActiveTab('ai')}>
                    AI
                </button>
                <span className="tab-spacer" />
                <button className="tab-disconnect" onClick={onClose}>
                    断开
                </button>
            </div>
            <div className={`tab-pane ${activeTab === 'terminal' ? '' : 'hidden'}`}>
                <TerminalView sessionId={sessionId} active={active} theme={theme} fontFamily={fontFamily} fontSize={fontSize} />
            </div>
            <div className={`tab-pane ${activeTab === 'files' ? '' : 'hidden'}`}>
                <FilePanel sessionId={sessionId} />
            </div>
            <div className={`tab-pane ${activeTab === 'ai' ? '' : 'hidden'}`}>
                <AiPanel sessionId={sessionId} onOpenSettings={onOpenSettings} />
            </div>
            <div className="status-bar">
                <span className="sb-dot" />
                <span className="sb-item">时长 {fmtDur(duration)}</span>
                <span className="sb-item">↓ {fmtRate(rate.in)}</span>
                <span className="sb-item">↑ {fmtRate(rate.out)}</span>
                <span className="sb-item">保活 {metrics?.keepAliveMs ? `${metrics.keepAliveMs}ms` : '-'}</span>
                <button className="sb-btn" onClick={doKeepAlive}>
                    保活
                </button>
                {kaMsg && <span className="sb-msg">{kaMsg}</span>}
            </div>
        </div>
    );
}
