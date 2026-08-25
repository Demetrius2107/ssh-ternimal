import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { SshSend, SshResize, ListSnippets, SessionMeta, AiExplain } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { THEMES, type ThemeName } from './themes';
import { loadShortcuts, matchShortcut } from './shortcuts';
import { AI_ERROR_RE } from './lib/detect';
import '@xterm/xterm/css/xterm.css';

export type { ThemeName } from './themes';

interface SshOutput {
    sessionId: number;
    data: string;
}

interface SshExit {
    sessionId: number;
    error: string;
}

interface SshReconnect {
    sessionId: number;
    attempt: number;
    max: number;
}

// ---------- 日志关键字着色 (纯文本日志无 ANSI 色码时生效) ----------
// 已含 ANSI 转义码的内容跳过, 避免双重着色破坏转义序列
const LOG_KEYWORDS: Array<[RegExp, string]> = [
    [/\b(ERROR|FATAL|FAILED|FAIL|CRITICAL|PANIC|Exception|异常|错误|失败|致命|超时)\b/g, '\x1b[31m$1\x1b[0m'], // 红
    [/\b(WARN|WARNING|警告|注意|小心|DENIED|拒绝)\b/g, '\x1b[33m$1\x1b[0m'], // 黄
    [/\b(SUCCESS|OK|DONE|成功|完成|通过)\b/g, '\x1b[32m$1\x1b[0m'], // 绿
    [/\b(INFO|信息|调试|DEBUG)\b/g, '\x1b[36m$1\x1b[0m'], // 青
];

function colorizeLog(data: string): string {
    if (data.indexOf('\x1b') >= 0) return data; // 已有 ANSI 色码, 原样透传
    let out = data;
    for (const [re, wrap] of LOG_KEYWORDS) {
        out = out.replace(re, wrap);
    }
    return out;
}

// ---------- 终端组件 ----------
// TerminalView 终端组件: 输出渲染/输入转发/尺寸同步/查找/右键菜单/主题实时切换
export default function TerminalView({ sessionId, active, theme, fontFamily, fontSize }: { sessionId: number; active: boolean; theme: ThemeName; fontFamily: string; fontSize: number }) {
    const containerRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const termRef = useRef<Terminal | null>(null);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const [exitMsg, setExitMsg] = useState('');
    const [showFind, setShowFind] = useState(false);
    const [ctx, setCtx] = useState<{ x: number; y: number } | null>(null);
    const [snippets, setSnippets] = useState<model.Snippet[]>([]);
    // 内联 AI 报错解释 (Warp 式): 检测到疑似报错 → 提示条 → 点击流式解释
    const [aiHint, setAiHint] = useState(false);
    const [explaining, setExplaining] = useState(false);
    const [explainText, setExplainText] = useState('');
    // 输出节流: 高频输出合并到一帧内一次写入, 避免大日志卡顿
    const pendingRef = useRef('');
    const rafRef = useRef<number | null>(null);
    // active 镜像: 非激活会话仍累积输出到 pendingRef (不丢数据), 但跳过 term.write 渲染
    // 避免重建终端 (useEffect 依赖 [sessionId]), 用 ref 持有最新值
    const activeRef = useRef(active);
    useEffect(() => {
        const wasInactive = !activeRef.current;
        activeRef.current = active;
        // 从非激活切回激活: 立即 flush 积压的输出
        if (active && wasInactive && pendingRef.current) {
            scheduleFlush();
        }
    }, [active]);

    // 合并并写入待处理输出 (rAF 单帧内只 flush 一次)
    // 非激活会话: 仍累积到 pendingRef (切回时不丢数据), 但跳过 term.write 避免后台渲染开销
    function scheduleFlush() {
        if (rafRef.current !== null) return; // 已有排队
        rafRef.current = requestAnimationFrame(() => {
            rafRef.current = null;
            const term = termRef.current;
            if (term && pendingRef.current) {
                if (activeRef.current) {
                    term.write(pendingRef.current);
                    pendingRef.current = '';
                }
                // 非激活时保留 pendingRef, 待切回时由 active useEffect 触发 flush
                // 上限保护: 积压超 512KB 丢弃前段, 避免隐藏会话内存无限增长
                if (!activeRef.current && pendingRef.current.length > 512 * 1024) {
                    pendingRef.current = pendingRef.current.slice(-256 * 1024);
                }
            }
        });
    }

    // 右键打开时加载命令片段 (供快速发送)
    useEffect(() => {
        if (!ctx) return;
        ListSnippets()
            .then((s) => setSnippets(s ?? []))
            .catch(() => setSnippets([]));
    }, [ctx]);

    useEffect(() => {
        const term = new Terminal({
            fontFamily,
            fontSize,
            cursorBlink: true,
            scrollback: 50000, // 增大滚动缓冲 (原10000, 大日志不丢前段)
            // 双击选中单词是 xterm 内建默认行为
            theme: THEMES[theme].xterm,
        });
        const fit = new FitAddon();
        const search = new SearchAddon();
        term.loadAddon(fit);
        term.loadAddon(search);
        term.loadAddon(new WebLinksAddon()); // URL 自动识别为可点击链接 (Ctrl+点击打开)
        term.open(containerRef.current!);
        fit.fit();
        SshResize(sessionId, term.rows, term.cols);
        termRef.current = term;
        searchAddonRef.current = search;

        // Ctrl+C 智能复制: 有选中文本时复制到剪贴板 (不发中断信号);
        // 无选中文本时发 \x03 中断远端进程 (与原生终端一致)
        term.attachCustomKeyEventHandler((e) => {
            if (e.type === 'keydown' && (e.ctrlKey || e.metaKey) && e.key === 'c') {
                const sel = term.getSelection();
                if (sel) {
                    navigator.clipboard.writeText(sel).catch(() => {});
                    term.clearSelection();
                    return false; // 阻止 \x03 发往远端
                }
            }
            return true;
        });

        const onOutput = (e: SshOutput) => {
            if (e.sessionId !== sessionId) return;
            // 报错检测: 仅匹配高置信度的错误模式 (行首 Error:/FATAL/Permission denied 等),
            // 避免误报 (如 grep error 输出、readme 含 "error" 字样)
            // 新输出到来时复位提示条 (让黄条随新正常输出消失)
            if (!explaining) {
                if (AI_ERROR_RE.test(e.data)) {
                    setAiHint(true);
                } else if (aiHint && !AI_ERROR_RE.test(e.data) && e.data.includes('\n')) {
                    setAiHint(false); // 新的非错误行到来, 自动复位
                }
            }
            pendingRef.current += colorizeLog(e.data);
            scheduleFlush();
        };
        const onExit = (e: SshExit) => {
            if (e.sessionId === sessionId) setExitMsg(e.error ? `会话已退出: ${e.error}` : '会话已结束');
        };
        const onReconnect = (e: SshReconnect) => {
            if (e.sessionId === sessionId) setExitMsg(`连接断开，正在重连 (${e.attempt}/${e.max})...`);
        };
        const offOutput = EventsOn('ssh-output', onOutput);
        const offExit = EventsOn('ssh-exit', onExit);
        const offReconnect = EventsOn('ssh-reconnect', onReconnect);
        // 内联 AI 报错解释流式事件 (独立事件名, 不与 AI 面板冲突)
        // payload 带 sessionId, 只处理本会话的流 (多会话时防止串扰)
        const offExplainDelta = EventsOn('ai-explain-delta', (e: { sessionId?: number; text: string }) => {
            if (e.sessionId !== sessionId) return;
            setExplainText((prev) => prev + e.text);
        });
        const offExplainDone = EventsOn('ai-explain-done', (e: { sessionId?: number }) => {
            if (e.sessionId !== sessionId) return;
            setExplaining(false);
        });
        const offExplainErr = EventsOn('ai-explain-error', (e: { sessionId?: number; text: string }) => {
            if (e.sessionId !== sessionId) return;
            setExplaining(false);
            setExplainText((prev) => prev + '\n[错误] ' + e.text);
        });

        const dataDispose = term.onData((data) => {
            SshSend(sessionId, data);
        });

        const ro = new ResizeObserver(() => {
            // rAF 防抖: 避免 fit() 在 RO 回调里改尺寸触发 loop 通知
            requestAnimationFrame(() => {
                fit.fit();
                SshResize(sessionId, term.rows, term.cols);
            });
        });
        ro.observe(containerRef.current!);

        return () => {
            ro.disconnect();
            dataDispose.dispose();
            offOutput();
            offExit();
            offReconnect();
            offExplainDelta();
            offExplainDone();
            offExplainErr();
            term.dispose();
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sessionId]);

    // 主题实时切换 (无需重启)
    useEffect(() => {
        if (termRef.current) termRef.current.options.theme = THEMES[theme].xterm;
    }, [theme]);

    // 字体/字号实时切换 (无需重启)
    useEffect(() => {
        const term = termRef.current;
        if (!term) return;
        term.options.fontFamily = fontFamily;
        term.options.fontSize = fontSize;
    }, [fontFamily, fontSize]);

    // 终端查找快捷键 (跟随用户配置, 默认 Ctrl+F), 只对激活会话生效
    useEffect(() => {
        if (!active) return;
        const onKey = (e: KeyboardEvent) => {
            if (matchShortcut(e, 'terminalFind', loadShortcuts())) {
                e.preventDefault();
                setShowFind(true);
                setTimeout(() => inputRef.current?.focus(), 0);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [active]);

    // 右键菜单: 点击别处/失焦/ESC 关闭
    useEffect(() => {
        if (!ctx) return;
        const close = () => setCtx(null);
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') close();
        };
        window.addEventListener('click', close);
        window.addEventListener('blur', close);
        window.addEventListener('keydown', onKey);
        return () => {
            window.removeEventListener('click', close);
            window.removeEventListener('blur', close);
            window.removeEventListener('keydown', onKey);
        };
    }, [ctx]);

    function doFind(next: boolean) {
        const q = inputRef.current?.value;
        const s = searchAddonRef.current;
        if (!q || !s) return;
        if (next) {
            s.findNext(q);
        } else {
            s.findPrevious(q);
        }
    }

    async function copySelection() {
        const sel = termRef.current?.getSelection();
        if (sel) {
            try {
                await navigator.clipboard.writeText(sel);
            } catch {
                /* 剪贴板不可用时忽略 */
            }
        }
        setCtx(null);
    }

    async function paste() {
        try {
            const text = await navigator.clipboard.readText();
            if (text) SshSend(sessionId, text);
        } catch {
            /* 剪贴板不可用时忽略 */
        }
        setCtx(null);
    }

    function selectAll() {
        termRef.current?.selectAll();
        setCtx(null);
    }

    // 发送命令片段到当前会话: 变量求值 ({host}/{user}/{port}/{date}) + 多行顺序执行 (宏)
    async function sendSnippet(cmd: string) {
        // 变量求值: 从会话元信息与当前日期填充占位符
        let resolved = cmd;
        try {
            const meta = await SessionMeta(sessionId);
            const map: Record<string, string> = {
                host: meta[0] ?? '',
                user: meta[1] ?? '',
                port: String(meta[2] ?? ''),
                date: new Date().toISOString().slice(0, 10),
            };
            resolved = cmd.replace(/\{(\w+)\}/g, (_, k: string) => map[k] ?? `{${k}}`);
            // 未定义的占位符保留 {xxx}: 提示但不阻塞发送
            const unresolved = resolved.match(/\{\w+\}/g);
            if (unresolved) {
                console.warn('片段含未定义变量:', [...new Set(unresolved)].join(', '));
            }
        } catch {
            /* 会话元信息不可用: 使用原片段 */
        }
        // 顺序执行: 多行按行依次发送 (宏), 每行补回车
        const lines = resolved.split('\n').filter((l) => l.trim() !== '');
        for (const line of lines) {
            SshSend(sessionId, line + '\r');
            await new Promise((r) => setTimeout(r, 60)); // 宏执行间隔, 避免击穿远端
        }
        setCtx(null);
    }

    // 请求 AI 解释最近报错 (Warp 式内联)
    async function askExplain() {
        setExplaining(true);
        setExplainText('');
        try {
            await AiExplain(sessionId, '解释上面最近的报错，给出原因和修复命令');
        } catch (e: any) {
            setExplaining(false);
            setExplainText(e?.message ?? String(e));
        }
    }

    return (
        <div className="terminal-layout">
            <div className="terminal-toolbar">
                <span>
                    SSH 会话 #{sessionId}
                    {exitMsg && <span className="exit-msg"> — {exitMsg}</span>}
                </span>
                <button onClick={() => setShowFind((v) => !v)} title="查找 (Ctrl+F)">
                    🔍 查询
                </button>
            </div>
            {aiHint && (
                <div className="ai-hint-bar">
                    <span className="ahb-icon">⚠️</span>
                    <span className="ahb-text">检测到疑似报错</span>
                    <button className="ahb-btn" onClick={askExplain} disabled={explaining}>
                        {explaining ? 'AI 解释中...' : '✨ 询问 AI 解释'}
                    </button>
                    <button className="ahb-close" onClick={() => setAiHint(false)} title="关闭提示">
                        ×
                    </button>
                </div>
            )}
            {explainText && (
                <div className="ai-explain-panel">
                    <div className="aep-head">
                        <span>AI 报错解释</span>
                        <button onClick={() => setExplainText('')}>关闭</button>
                    </div>
                    <pre className="aep-content">{explainText || '思考中...'}</pre>
                </div>
            )}
            {showFind && (
                <div className="find-bar">
                    <input
                        ref={inputRef}
                        placeholder="查找内容..."
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') doFind(!e.shiftKey);
                        }}
                    />
                    <button onClick={() => doFind(true)}>下一个</button>
                    <button onClick={() => doFind(false)}>上一个</button>
                    <button onClick={() => setShowFind(false)}>关闭</button>
                </div>
            )}
            <div
                className={`terminal-container ${THEMES[theme].mode === 'light' ? 'tc-light' : ''}`}
                ref={containerRef}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setCtx({ x: e.clientX, y: e.clientY });
                }}
            />
            {ctx && (
                <div className="ctx-menu" style={{ left: ctx.x, top: ctx.y }} onClick={(e) => e.stopPropagation()}>
                    <button onClick={copySelection}>复制</button>
                    <button onClick={paste}>粘贴</button>
                    <button onClick={selectAll}>全选</button>
                    {snippets.length > 0 && (
                        <>
                            <div className="ctx-sep" />
                            <div className="ctx-label">发送片段</div>
                            {snippets.map((s) => (
                                <button key={s.id} onClick={() => sendSnippet(s.command)} title={s.command}>
                                    {s.name}
                                </button>
                            ))}
                        </>
                    )}
                </div>
            )}
        </div>
    );
}
