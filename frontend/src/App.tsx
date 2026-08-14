import { useEffect, useState } from 'react';
import { SshClose } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import ConnectForm from './ConnectForm';
import Workspace from './Workspace';
import './App.css';

interface OpenSession {
    id: number;
    label: string;
}

// App 根组件: 会话标签栏 + 多会话工作区 + 新建连接模态
function App() {
    const [openSessions, setOpenSessions] = useState<OpenSession[]>([]);
    const [activeId, setActiveId] = useState<number | null>(null);
    const [showConnect, setShowConnect] = useState(false);

    // 会话意外结束 (ssh-exit) 时自动移除标签; EventsOn 返回注销函数
    useEffect(() => {
        const onExit = (e: { sessionId: number }) => {
            setOpenSessions((prev) => prev.filter((s) => s.id !== e.sessionId));
            setActiveId((prev) => (prev === e.sessionId ? null : prev));
        };
        return EventsOn('ssh-exit', onExit);
    }, []);

    function closeSession(id: number) {
        SshClose(id);
        setOpenSessions((prev) => prev.filter((s) => s.id !== id));
        setActiveId((prev) => (prev === id ? null : prev));
    }

    function onConnected(id: number, label: string) {
        setOpenSessions((prev) => [...prev, { id, label }]);
        setActiveId(id);
        setShowConnect(false);
    }

    return (
        <div className="app-root">
            <div className="session-tabbar">
                <button className="btn-new" onClick={() => setShowConnect(true)}>
                    ＋ 新建连接
                </button>
                {openSessions.map((s) => (
                    <div
                        key={s.id}
                        className={`session-tab ${s.id === activeId ? 'active' : ''}`}
                        onClick={() => setActiveId(s.id)}
                        title={s.label}
                    >
                        <span className="st-label">{s.label}</span>
                        <button
                            className="tab-x"
                            onClick={(e) => {
                                e.stopPropagation();
                                closeSession(s.id);
                            }}
                        >
                            ×
                        </button>
                    </div>
                ))}
            </div>
            <div className="session-body">
                {openSessions.map((s) => (
                    <div key={s.id} className={`session-pane ${s.id === activeId ? '' : 'hidden'}`}>
                        <Workspace sessionId={s.id} active={s.id === activeId} onClose={() => closeSession(s.id)} />
                    </div>
                ))}
                {openSessions.length === 0 && (
                    <div className="empty-hint">点击左上角「＋ 新建连接」建立 SSH 会话</div>
                )}
            </div>
            {showConnect && (
                <div className="modal-mask" onClick={() => setShowConnect(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()}>
                        <ConnectForm onConnected={onConnected} onCancel={() => setShowConnect(false)} />
                    </div>
                </div>
            )}
        </div>
    );
}

export default App;
