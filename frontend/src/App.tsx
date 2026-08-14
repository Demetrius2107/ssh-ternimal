import { useState } from 'react';
import { SshConnect } from '../wailsjs/go/main/App';
import { main } from '../wailsjs/go/models';
import TerminalView from './TerminalView';
import './App.css';

function App() {
    const [host, setHost] = useState('');
    const [port, setPort] = useState(22);
    const [username, setUsername] = useState('root');
    const [password, setPassword] = useState('');
    const [sessionId, setSessionId] = useState<number | null>(null);
    const [connecting, setConnecting] = useState(false);
    const [error, setError] = useState('');

    async function connect() {
        setConnecting(true);
        setError('');
        try {
            const cfg = new main.SshConfig({
                host,
                port,
                username,
                password,
                privateKey: '',
                passphrase: '',
            });
            const id = await SshConnect(cfg);
            setSessionId(id);
        } catch (e: any) {
            setError(e?.message ?? String(e));
        } finally {
            setConnecting(false);
        }
    }

    if (sessionId !== null) {
        return <TerminalView sessionId={sessionId} onClose={() => setSessionId(null)} />;
    }

    return (
        <div className="connect-panel">
            <h1>SSH 终端</h1>
            <form
                onSubmit={(e) => {
                    e.preventDefault();
                    connect();
                }}
            >
                <label>
                    主机
                    <input
                        value={host}
                        onChange={(e) => setHost(e.target.value)}
                        placeholder="192.168.1.100"
                        required
                        autoFocus
                    />
                </label>
                <label>
                    端口
                    <input
                        type="number"
                        value={port}
                        min={1}
                        max={65535}
                        onChange={(e) => setPort(Number(e.target.value))}
                    />
                </label>
                <label>
                    用户名
                    <input value={username} onChange={(e) => setUsername(e.target.value)} required />
                </label>
                <label>
                    密码
                    <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
                </label>
                <button type="submit" disabled={connecting}>
                    {connecting ? '连接中...' : '连接'}
                </button>
            </form>
            {error && <div className="error-box">{error}</div>}
        </div>
    );
}

export default App;
