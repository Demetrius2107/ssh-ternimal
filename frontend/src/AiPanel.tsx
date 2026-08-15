import { useEffect, useRef, useState } from 'react';
import { AiChat, AiCancel, AiStatus } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';

interface AiMsg {
    role: 'user' | 'assistant';
    text: string;
}

// AiPanel AI 辅助侧栏: 对话 + 流式渲染 (ai-delta 事件) + 停止 (成本控制)
export default function AiPanel({ sessionId }: { sessionId: number }) {
    const [msgs, setMsgs] = useState<AiMsg[]>([]);
    const [input, setInput] = useState('');
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState<model.AiStatus | null>(null);
    const [err, setErr] = useState('');
    const streamRef = useRef('');

    useEffect(() => {
        AiStatus()
            .then((s) => setStatus(s))
            .catch(() => undefined);
        // 流式输出 / 完成 / 错误事件
        const offDelta = EventsOn('ai-delta', (e: { text: string }) => {
            streamRef.current += e.text;
            setMsgs((prev) => {
                const next = [...prev];
                if (next.length > 0 && next[next.length - 1].role === 'assistant') {
                    next[next.length - 1] = { role: 'assistant', text: streamRef.current };
                } else {
                    next.push({ role: 'assistant', text: streamRef.current });
                }
                return next;
            });
        });
        const offDone = EventsOn('ai-done', () => {
            setBusy(false);
            streamRef.current = '';
        });
        const offErr = EventsOn('ai-error', (e: { text: string }) => {
            setBusy(false);
            streamRef.current = '';
            setErr(e.text);
        });
        return () => {
            offDelta();
            offDone();
            offErr();
        };
    }, []);

    async function send() {
        const q = input.trim();
        if (!q || busy) return;
        setErr('');
        setMsgs((prev) => [...prev, { role: 'user', text: q }]);
        setInput('');
        setBusy(true);
        streamRef.current = '';
        setMsgs((prev) => [...prev, { role: 'assistant', text: '' }]);
        try {
            await AiChat(sessionId, q);
        } catch (e: any) {
            setBusy(false);
            setErr(e?.message ?? String(e));
        }
    }

    function stop() {
        AiCancel();
        setBusy(false);
    }

    return (
        <div className="ai-panel">
            <div className="ai-status">
                <span className="ai-dot" />
                {status
                    ? `${status.provider === 'ollama' ? 'Ollama' : 'DeepSeek'} · ${status.model} · 本月 ${status.monthUsage}/${status.monthlyLimit}`
                    : 'AI 未配置'}
            </div>
            {err && <div className="error-box">{err}</div>}
            <div className="ai-msgs">
                {msgs.length === 0 && <div className="hist-empty">描述你想做的事，AI 会结合当前会话上下文给出命令或解释</div>}
                {msgs.map((m, i) => (
                    <div key={i} className={`ai-msg ${m.role}`}>
                        <div className="ai-role">{m.role === 'user' ? '你' : 'AI'}</div>
                        <div className="ai-text">
                            {m.text || (busy && i === msgs.length - 1 ? '思考中...' : '')}
                        </div>
                    </div>
                ))}
            </div>
            <div className="ai-input">
                <textarea
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' && !e.shiftKey) {
                            e.preventDefault();
                            send();
                        }
                    }}
                    placeholder="问 AI 命令 / 解释报错 (Enter 发送, Shift+Enter 换行)"
                    rows={2}
                />
                <div className="ai-actions">
                    {busy ? (
                        <button onClick={stop}>■ 停止</button>
                    ) : (
                        <button onClick={send} disabled={!input.trim()}>
                            发送
                        </button>
                    )}
                </div>
            </div>
        </div>
    );
}
