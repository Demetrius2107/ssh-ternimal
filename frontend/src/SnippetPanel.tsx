import { useEffect, useState } from 'react';
import { ListSnippets, SaveSnippet, DeleteSnippet } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

// SnippetPanel 命令片段管理: 新增/删除/列表 (快速发送在终端右键菜单)
export default function SnippetPanel() {
    const [snippets, setSnippets] = useState<model.Snippet[]>([]);
    const [name, setName] = useState('');
    const [command, setCommand] = useState('');
    const [err, setErr] = useState('');
    const [msg, setMsg] = useState('');

    async function refresh() {
        try {
            setSnippets((await ListSnippets()) ?? []);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    useEffect(() => {
        refresh();
    }, []);

    async function doSave() {
        setErr('');
        setMsg('');
        if (!name.trim() || !command.trim()) {
            setErr('名称和命令不能为空');
            return;
        }
        try {
            await SaveSnippet(name.trim(), command.trim(), '');
            setName('');
            setCommand('');
            setMsg('已保存');
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    async function doDelete(id: string) {
        try {
            await DeleteSnippet(id);
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="snippet-panel">
            <div className="snippet-form">
                <input value={name} onChange={(e) => setName(e.target.value)} placeholder="名称, 如: 查看磁盘" />
                <input value={command} onChange={(e) => setCommand(e.target.value)} placeholder="命令, 如: df -h" />
                <button onClick={doSave}>保存</button>
            </div>
            {err && <div className="error-box">{err}</div>}
            {msg && <div className="tunnel-msg">{msg}</div>}
            <div className="snippet-list">
                {snippets.length === 0 && <div className="hist-empty">暂无命令片段，先添加一个吧</div>}
                {snippets.map((s) => (
                    <div key={s.id} className="snippet-item">
                        <span className="si-name">{s.name}</span>
                        <code className="si-cmd">{s.command}</code>
                        <button className="si-del" onClick={() => doDelete(s.id)}>
                            删除
                        </button>
                    </div>
                ))}
            </div>
            <div className="hist-empty hint">在终端中右键 → 「发送片段」可快速执行</div>
        </div>
    );
}
