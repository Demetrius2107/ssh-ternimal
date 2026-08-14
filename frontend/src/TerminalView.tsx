import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { SshSend, SshResize } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import '@xterm/xterm/css/xterm.css';

interface SshOutput {
    sessionId: number;
    data: string;
}

interface SshExit {
    sessionId: number;
    error: string;
}

// TerminalView 终端组件: 会话输出渲染、输入转发、尺寸同步、查找 (Ctrl+F)
export default function TerminalView({ sessionId, active }: { sessionId: number; active: boolean }) {
    const containerRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);
    const termRef = useRef<Terminal | null>(null);
    const searchAddonRef = useRef<SearchAddon | null>(null);
    const [exitMsg, setExitMsg] = useState('');
    const [showFind, setShowFind] = useState(false);

    useEffect(() => {
        const term = new Terminal({
            fontFamily: 'Consolas, "Courier New", monospace',
            fontSize: 14,
            cursorBlink: true,
            scrollback: 10000,
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#aeafad',
                selectionBackground: '#264f78',
                black: '#1e1e1e',
                red: '#f44747',
                green: '#6a9955',
                yellow: '#dcdcaa',
                blue: '#569cd6',
                magenta: '#c586c0',
                cyan: '#4ec9b0',
                white: '#d4d4d4',
            },
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
            fit.fit();
            SshResize(sessionId, term.rows, term.cols);
        });
        ro.observe(containerRef.current!);

        return () => {
            ro.disconnect();
            dataDispose.dispose();
            offOutput();
            offExit();
            term.dispose();
        };
    }, [sessionId]);

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
            <div className="terminal-container" ref={containerRef} />
        </div>
    );
}
