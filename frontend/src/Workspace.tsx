import { useState } from 'react';
import TerminalView from './TerminalView';
import FilePanel from './FilePanel';

interface Props {
    sessionId: number;
    active: boolean; // 当前会话标签是否激活 (控制 Ctrl+F 等全局快捷键只作用于激活会话)
    onClose: () => void;
}

// Workspace 单个会话的工作区: 终端/文件 子标签页 (隐藏不卸载, 保持会话与终端缓冲)
export default function Workspace({ sessionId, active, onClose }: Props) {
    const [activeTab, setActiveTab] = useState<'terminal' | 'files'>('terminal');

    return (
        <div className="workspace">
            <div className="tab-bar">
                <button className={activeTab === 'terminal' ? 'tab active' : 'tab'} onClick={() => setActiveTab('terminal')}>
                    终端
                </button>
                <button className={activeTab === 'files' ? 'tab active' : 'tab'} onClick={() => setActiveTab('files')}>
                    文件
                </button>
                <span className="tab-spacer" />
                <button className="tab-disconnect" onClick={onClose}>
                    断开
                </button>
            </div>
            <div className={`tab-pane ${activeTab === 'terminal' ? '' : 'hidden'}`}>
                <TerminalView sessionId={sessionId} active={active} />
            </div>
            <div className={`tab-pane ${activeTab === 'files' ? '' : 'hidden'}`}>
                <FilePanel sessionId={sessionId} />
            </div>
        </div>
    );
}
