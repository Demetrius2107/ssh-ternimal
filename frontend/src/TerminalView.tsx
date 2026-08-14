import { useEffect, useRef, useState } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SshSend, SshResize, SshClose } from '../wailsjs/go/main/App';
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime';
import '@xterm/xterm/css/xterm.css';

interface SshOutput {
    sessionId: number;
    data: string;
}

interface SshExit {
    sessionId: number;
    error: string;
}

export default function TerminalView({ sessionId, onClose }: { sessionId: number; onClose: () => void }) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [exitMsg, setExitMsg] = useState('');

    useEffect(() => {
        const term = new Terminal({
            fontFamily: 'Consolas, "Courier New", monospace',
            fontSize: 14,
            cursorBlink: true,
            scrollback: 5000,
            theme: { background: '#1e1e1e', foreground: '#d4d4d4' },
        });
        const fit = new FitAddon();
        term.loadAddon(fit);
        term.open(containerRef.current!);
        fit.fit();
        SshResize(sessionId, term.rows, term.cols);

        const onOutput = (e: SshOutput) => {
            if (e.sessionId === sessionId) term.write(e.data);
        };
        const onExit = (e: SshExit) => {
            if (e.sessionId === sessionId) setExitMsg(e.error ? `会话已退出: ${e.error}` : '会话已结束');
        };
        EventsOn('ssh-output', onOutput);
        EventsOn('ssh-exit', onExit);

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
            EventsOff('ssh-output');
            EventsOff('ssh-exit');
            SshClose(sessionId);
            term.dispose();
        };
    }, [sessionId]);

    return (
        <div className="terminal-layout">
            <div className="terminal-toolbar">
                <span>
                    SSH 会话 #{sessionId}
                    {exitMsg && <span className="exit-msg"> — {exitMsg}</span>}
                </span>
                <button
                    onClick={() => {
                        SshClose(sessionId);
                        onClose();
                    }}
                >
                    断开
                </button>
            </div>
            <div className="terminal-container" ref={containerRef} />
        </div>
    );
}
