import { useEffect, useRef, useState } from 'react';
import { AiChat, AiCancel, AiStatus, AiConfigure } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';

interface AiMsg {
    role: 'user' | 'assistant';
    text: string;
}

// 常用模型选项 (DeepSeek 档位 / Ollama 常见模型)
const DEEPSEEK_MODELS = ['deepseek-chat', 'deepseek-reasoner'];
const OLLAMA_MODELS = ['qwen2.5', 'deepseek-r1', 'llama3.2', 'qwen2.5-coder'];

// AiPanel AI 辅助: 对话 + 流式渲染 + 模型选择 + 新会话 + 停止 (成本控制)
export default function AiPanel({ sessionId, onOpenSettings }: { sessionId: number; onOpenSettings?: () => void }) {
    const [msgs, setMsgs] = useState<AiMsg[]>([]);
    const [input, setInput] = useState('');
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState<model.AiStatus | null>(null);
    const [err, setErr] = useState('');
    const streamRef = useRef('');

    async function refreshStatus() {
        try {
            setStatus(await AiStatus());
        } catch {
            /* 忽略 */
        }
    }

    useEffect(() => {
        refreshStatus();
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
            refreshStatus(); // 用量变化后刷新
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

    // 新会话: 清空对话记录
    function newSession() {
        if (busy) stop();
        setMsgs([]);
        setErr('');
        streamRef.current = '';
    }

    // 切换模型: 更新后端配置并刷新状态
    async function changeModel(mdl: string) {
        if (!mdl || !status) return;
        try {
            await AiConfigure(status.provider || 'deepseek', mdl, status.monthlyLimit);
            await refreshStatus();
            setErr('');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    const provider = status?.provider || 'deepseek';
    const modelOptions = provider === 'ollama' ? OLLAMA_MODELS : DEEPSEEK_MODELS;

    return (
        <div className="ai-panel">
            <div className="ai-status">
                <span className="ai-dot" />
                <select
                    className="ai-model-select"
                    value={status?.model || (provider === 'ollama' ? 'qwen2.5' : 'deepseek-chat')}
                    onChange={(e) => changeModel(e.target.value)}
                    title="切换模型"
                >
                    {modelOptions.map((mdl) => (
                        <option key={mdl} value={mdl}>
                            {provider === 'ollama' ? mdl : mdl === 'deepseek-reasoner' ? 'DeepSeek R1 (深度思考)' : 'DeepSeek Chat (快捷)'}
                        </option>
                    ))}
                    {status?.model && !modelOptions.includes(status.model) && (
                        <option value={status.model}>{status.model} (当前)</option>
                    )}
                </select>
                <span className="ai-usage">
                    本月 {status ? `${status.monthUsage.toLocaleString()}/${status.monthlyLimit.toLocaleString()}` : '-'}
                </span>
                {onOpenSettings && (
                    <button className="ai-gear" onClick={onOpenSettings} title="AI 配置 (API Key/限额)">
                        ⚙
                    </button>
                )}
                <button className="ai-new" onClick={newSession} title="新会话 (清空对话)">
                    ✚ 新会话
                </button>
            </div>
            {err && <div className="error-box">{err}</div>}
            {!status?.keyConfigured && provider === 'deepseek' && !err && (
                <div className="ai-hint">
                    DeepSeek API Key 未配置，请点 ⚙ 在设置中填写
                </div>
            )}
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
