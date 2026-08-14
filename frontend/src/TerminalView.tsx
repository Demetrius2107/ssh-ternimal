import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { SshSend, SshResize } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { THEMES, type ThemeName } from './themes';
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

// TerminalView 终端组件: 输出渲染/输入转发/尺寸同步/查找/右键菜单/主题实时切换
export default function TerminalView({ sessionId, active, theme }: { sessionId: number; active: boolean; theme: ThemeName }) {
    const containerRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const termRef = useRef<Terminal | null>(null);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const [exitMsg, setExitMsg] = useState('');
    const [showFind, setShowFind] = useState(false);
    const [ctx, setCtx] = useState<{ x: number; y: number } | null>(null);

    useEffect(() => {
        const term = new Terminal({
            fontFamily: 'Consolas, "Courier New", monospace',
            fontSize: 14,
            cursorBlink: true,
            scrollback: 10000,
            // 双击选中单词是 xterm 内建默认行为
            theme: THEMES[theme].xterm,
        });
        const fit = new FitAddon();
        const search = new SearchAddon();
        term.loadAddon(fit);
        term.loadAddon(search);
        term.open(containerRef.current!);
        fit.fit();
        SshResize(sessionId, term.rows, term.cols);
        termRef.current = term;
        searchAddonRef.current = search;

        const onOutput = (e: SshOutput) => {
            if (e.sessionId === sessionId) term.write(e.data);
        };
        const onExit = (e: SshExit) => {
            if (e.sessionId === sessionId) setExitMsg(e.error ? `会话已退出: ${e.error}` : '会话已结束');
        };
        const offOutput = EventsOn('ssh-output', onOutput);
        const offExit = EventsOn('ssh-exit', onExit);

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
            term.dispose();
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sessionId]);

    // 主题实时切换 (无需重启)
    useEffect(() => {
        if (termRef.current) termRef.current.options.theme = THEMES[theme].xterm;
    }, [theme]);

    // Ctrl+F 只对激活会话生效
    useEffect(() => {
        if (!active) return;
        const onKey = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
                e.preventDefault();
                setShowFind(true);
                setTimeout(() => inputRef.current?.focus(), 0);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [active]);

    // 右键菜单: 点击别处/失焦关闭
    useEffect(() => {
        if (!ctx) return;
        const close = () => setCtx(null);
        window.addEventListener('click', close);
        window.addEventListener('blur', close);
        return () => {
            window.removeEventListener('click', close);
            window.removeEventListener('blur', close);
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
                </div>
            )}
        </div>
    );
}
