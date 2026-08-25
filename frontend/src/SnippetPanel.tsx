import { useEffect, useRef, useState } from 'react';
import { ListSnippets, SaveSnippet, DeleteSnippet, ExportSnippets, ImportSnippets, PickFile, EditorSaveLocal, EditorLoadLocal } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

// 分组数据: 保存当前分组过滤 + 编辑/新建用的分组字段
interface SnippetForm {
    name: string;
    command: string;
    group: string;
    id: string; // 空=新建, 非空=编辑已有
}

const EMPTY_FORM: SnippetForm = { name: '', command: '', group: '', id: '' };

// SnippetPanel 命令片段管理: 新增/编辑/删除/分组/导入导出 (快速发送在终端右键菜单)
export default function SnippetPanel() {
    const [snippets, setSnippets] = useState<model.Snippet[]>([]);
    const [form, setForm] = useState<SnippetForm>(EMPTY_FORM);
    const [editing, setEditing] = useState(false); // 是否处于编辑模式
    const [groupFilter, setGroupFilter] = useState(''); // 分组过滤 (空=全部)
    const [groups, setGroups] = useState<string[]>([]);
    const [err, setErr] = useState('');
    const [msg, setMsg] = useState('');
    const inputRef = useRef<HTMLInputElement>(null);

    async function refresh() {
        try {
            const list = (await ListSnippets()) ?? [];
            setSnippets(list);
            // 去重分组名
            const gs = Array.from(new Set(list.map((s) => s.group).filter((g) => g)));
            setGroups(gs);
            // 若当前过滤的分组已被删除, 复位
            if (groupFilter && !gs.includes(groupFilter)) setGroupFilter('');
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
        if (!form.name.trim() || !form.command.trim()) {
            setErr('名称和命令不能为空');
            return;
        }
        try {
            await SaveSnippet(form.name.trim(), form.command.trim(), form.group.trim(), form.id);
            setForm(EMPTY_FORM);
            setEditing(false);
            setMsg(form.id ? '已更新' : '已保存');
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    // 编辑: 回填表单并滚动到顶部
    function startEdit(s: model.Snippet) {
        setForm({ name: s.name, command: s.command, group: s.group ?? '', id: s.id });
        setEditing(true);
        setErr('');
        setMsg('');
        window.scrollTo({ top: 0, behavior: 'smooth' });
        setTimeout(() => inputRef.current?.focus(), 0);
    }

    function cancelEdit() {
        setForm(EMPTY_FORM);
        setEditing(false);
    }

    async function doDelete(id: string) {
        if (!window.confirm('确认删除该片段？')) return;
        try {
            await DeleteSnippet(id);
            // 删除的若是正在编辑的, 退出编辑态
            if (form.id === id) cancelEdit();
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    // 导出: 保存为 JSON 文件 (用 PickFile 选保存路径)
    async function doExport() {
        setErr('');
        setMsg('');
        try {
            const jsonStr = await ExportSnippets();
            const p = await PickFile();
            if (!p) return;
            await EditorSaveLocal(p, jsonStr);
            setMsg('已导出片段 JSON');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    // 导入: 从 JSON 文件读取
    async function doImport() {
        setErr('');
        setMsg('');
        try {
            const p = await PickFile();
            if (!p) return;
            const text = await EditorLoadLocal(p);
            const n = await ImportSnippets(text);
            setMsg(`已导入 ${n} 条片段`);
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    // 过滤后的片段
    const visible = groupFilter ? snippets.filter((s) => (s.group ?? '') === groupFilter) : snippets;

    return (
        <div className="snippet-panel">
            <div className="snippet-form">
                <input
                    ref={inputRef}
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    placeholder="名称, 如: 查看磁盘"
                />
                <input
                    value={form.command}
                    onChange={(e) => setForm({ ...form, command: e.target.value })}
                    placeholder="命令, 如: df -h"
                />
                <input
                    value={form.group}
                    onChange={(e) => setForm({ ...form, group: e.target.value })}
                    placeholder="分组 (可选)"
                    list="snippet-groups"
                />
                <datalist id="snippet-groups">
                    {groups.map((g) => (
                        <option key={g} value={g} />
                    ))}
                </datalist>
                <button onClick={doSave}>{editing ? '更新' : '保存'}</button>
                {editing && (
                    <button className="snippet-cancel" onClick={cancelEdit}>
                        取消
                    </button>
                )}
            </div>
            <div className="snippet-toolbar">
                <select
                    className="snippet-group-filter"
                    value={groupFilter}
                    onChange={(e) => setGroupFilter(e.target.value)}
                    title="按分组过滤"
                >
                    <option value="">全部片段</option>
                    {groups.map((g) => (
                        <option key={g} value={g}>
                            {g}
                        </option>
                    ))}
                </select>
                <span className="snippet-count">共 {visible.length} 条</span>
                <span className="snippet-toolbar-spacer" />
                <button onClick={doExport}>导出</button>
                <button onClick={doImport}>导入</button>
            </div>
            {err && <div className="error-box">{err}</div>}
            {msg && <div className="tunnel-msg">{msg}</div>}
            <div className="snippet-list">
                {visible.length === 0 && <div className="hist-empty">暂无命令片段，先添加一个吧</div>}
                {visible.map((s) => (
                    <div key={s.id} className="snippet-item">
                        <span className="si-name">
                            {s.name}
                            {s.group && <span className="si-group">{s.group}</span>}
                        </span>
                        <code className="si-cmd">{s.command}</code>
                        <div className="si-actions">
                            <button className="si-edit" onClick={() => startEdit(s)}>
                                编辑
                            </button>
                            <button className="si-del" onClick={() => doDelete(s.id)}>
                                删除
                            </button>
                        </div>
                    </div>
                ))}
            </div>
            <div className="hist-empty hint">在终端中右键 → 「发送片段」可快速执行 · 支持 {'{host}'} {'{user}'} {'{port}'} {'{date}'} 变量</div>
        </div>
    );
}
