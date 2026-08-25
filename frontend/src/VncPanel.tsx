import { useEffect, useRef, useState } from 'react';
import { VncConnect, VncKeyEvent, VncPointerEvent, VncClose } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

// VncFrame 帧事件负载 (与后端 VncFrame 一致)
interface VncFrame {
    sessionId: number;
    width: number;
    height: number;
    data: string; // RGBA base64
}

// X11 keysym 特殊键映射 (0xFF00 区)
const KEY_KEYSYM: Record<string, number> = {
    Escape: 0xFF1B,
    Enter: 0xFF0D,
    Backspace: 0xFF08,
    Tab: 0xFF09,
    Delete: 0xFFFF,
    ArrowUp: 0xFF52,
    ArrowDown: 0xFF54,
    ArrowLeft: 0xFF51,
    ArrowRight: 0xFF53,
    Home: 0xFF50,
    End: 0xFF57,
    PageUp: 0xFF55,
    PageDown: 0xFF56,
    F1: 0xFFBE, F2: 0xFFBF, F3: 0xFFC0, F4: 0xFFC1,
    F5: 0xFFC2, F6: 0xFFC3, F7: 0xFFC4, F8: 0xFFC5,
    F9: 0xFFC6, F10: 0xFFC7, F11: 0xFFC8, F12: 0xFFC9,
    Shift: 0xFFE1, Control: 0xFFE3, Alt: 0xFFE9, Meta: 0xFFE7,
    ' ': 0x20,
};

// VncPanel 内嵌 VNC: canvas 渲染远程桌面 + 鼠标键盘输入转发
export default function VncPanel() {
    const [host, setHost] = useState('');
    const [port, setPort] = useState(5900);
    const [password, setPassword] = useState('');
    const [connected, setConnected] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const [err, setErr] = useState('');
    const sessionIdRef = useRef(0);
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const buttonsRef = useRef(0); // 当前按住的鼠标按钮掩码
    const frameRef = useRef<VncFrame | null>(null);

    // 接收帧事件并渲染
    useEffect(() => {
        const off = EventsOn('vnc-frame', (f: VncFrame) => {
            if (f.sessionId !== sessionIdRef.current) return;
            frameRef.current = f;
            renderFrame(f);
        });
        return off;
    }, []);

    function renderFrame(f: VncFrame) {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const dpr = window.devicePixelRatio || 1;
        // 用 CSS 尺寸等比缩放显示
        canvas.width = f.width;
        canvas.height = f.height;
        canvas.style.width = `${f.width}px`;
        canvas.style.height = `${f.height}px`;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        const bytes = atob(f.data);
        const img = new ImageData(f.width, f.height);
        const arr = img.data;
        for (let i = 0; i < arr.length; i++) arr[i] = bytes.charCodeAt(i);
        ctx.putImageData(img, 0, 0);
        void dpr;
    }

    async function connect() {
        if (!host.trim()) {
            setErr('请输入主机地址');
            return;
        }
        setConnecting(true);
        setErr('');
        try {
            const id = await VncConnect(host.trim(), port, password);
            sessionIdRef.current = id;
            setConnected(true);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setConnecting(false);
        }
    }

    async function disconnect() {
        if (sessionIdRef.current) {
            VncClose(sessionIdRef.current);
            sessionIdRef.current = 0;
        }
        setConnected(false);
        const canvas = canvasRef.current;
        canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height);
    }

    // canvas 坐标 → VNC 绝对坐标
    function toVncPos(e: React.MouseEvent): { x: number; y: number } {
        const canvas = canvasRef.current;
        if (!canvas) return { x: 0, y: 0 };
        const rect = canvas.getBoundingClientRect();
        const f = frameRef.current;
        const x = Math.round(((e.clientX - rect.left) / rect.width) * (f?.width ?? canvas.width));
        const y = Math.round(((e.clientY - rect.top) / rect.height) * (f?.height ?? canvas.height));
        return { x: Math.max(0, x), y: Math.max(0, y) };
    }

    function onMouseMove(e: React.MouseEvent) {
        if (!connected) return;
        const { x, y } = toVncPos(e);
        VncPointerEvent(sessionIdRef.current, buttonsRef.current, x, y);
    }

    function onMouseDown(e: React.MouseEvent) {
        if (!connected) return;
        e.preventDefault();
        const mask = e.button === 0 ? 1 : e.button === 1 ? 2 : 4;
        buttonsRef.current |= mask;
        const { x, y } = toVncPos(e);
        VncPointerEvent(sessionIdRef.current, buttonsRef.current, x, y);
        canvasRef.current?.focus();
    }

    function onMouseUp(e: React.MouseEvent) {
        if (!connected) return;
        const mask = e.button === 0 ? 1 : e.button === 1 ? 2 : 4;
        buttonsRef.current &= ~mask;
        const { x, y } = toVncPos(e);
        VncPointerEvent(sessionIdRef.current, buttonsRef.current, x, y);
    }

    function onWheel(e: React.WheelEvent) {
        if (!connected) return;
        // 滚轮: 模拟中键点击 (4=上滚, 8=下滚) — RFB 无标准滚轮, 用中键替代
        const dir = e.deltaY < 0 ? 4 : 8;
        const { x, y } = toVncPos(e);
        VncPointerEvent(sessionIdRef.current, dir, x, y);
        VncPointerEvent(sessionIdRef.current, 0, x, y);
    }

    // 按键: 按下发 down, 松开发 up (分别只发一次, 避免远端收到重复键)
    function onKeyDown(e: React.KeyboardEvent) {
        if (!connected) return;
        e.preventDefault();
        const ks = KEY_KEYSYM[e.key] ?? (e.key.length === 1 ? e.key.charCodeAt(0) : 0);
        if (!ks) return;
        VncKeyEvent(sessionIdRef.current, ks, true);
    }

    function onKeyUp(e: React.KeyboardEvent) {
        if (!connected) return;
        e.preventDefault();
        const ks = KEY_KEYSYM[e.key] ?? (e.key.length === 1 ? e.key.charCodeAt(0) : 0);
        if (!ks) return;
        VncKeyEvent(sessionIdRef.current, ks, false);
    }

    return (
        <div className="vnc-panel">
            <div className="vnc-toolbar">
                <input
                    className="ep-path"
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="VNC 主机 (默认端口 5900)"
                    disabled={connected}
                />
                <input
                    className="vnc-port"
                    type="number"
                    value={port}
                    min={1}
                    max={65535}
                    onChange={(e) => setPort(Number(e.target.value))}
                    disabled={connected}
                    placeholder="端口"
                />
                <input
                    className="ep-path vnc-pass"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="密码 (可空)"
                    disabled={connected}
                />
                {connected ? (
                    <button className="vnc-disconnect" onClick={disconnect}>
                        断开
                    </button>
                ) : (
                    <button onClick={connect} disabled={connecting || !host.trim()}>
                        {connecting ? '连接中...' : '连接'}
                    </button>
                )}
            </div>
            {err && <div className="error-box">{err}</div>}
            {!connected && <div className="hist-empty">输入主机地址和密码，连接内嵌 VNC 桌面（需服务器开启 VNC 服务）</div>}
            <div className="vnc-viewport">
                <canvas
                    ref={canvasRef}
                    className="vnc-canvas"
                    tabIndex={0}
                    onMouseMove={onMouseMove}
                    onMouseDown={onMouseDown}
                    onMouseUp={onMouseUp}
                    onWheel={onWheel}
                    onKeyDown={onKeyDown}
                    onKeyUp={onKeyUp}
                />
            </div>
        </div>
    );
}
