import { useRef, useState } from 'react';
import { EditorLoadRemote, EditorSaveRemote, EditorSaveLocal, PickFile } from '../wailsjs/go/main/App';

// EditorPanel 内置文本编辑器: 作为 SFTP 上传/下载的中间态
// 打开远程文件 → 本地编辑 → 保存回传远程 / 另存本地; 支持新建文件
export default function EditorPanel({ sessionId }: { sessionId: number }) {
    const [remotePath, setRemotePath] = useState('');
    const [content, setContent] = useState('');
    const [filename, setFilename] = useState('未命名');
    const [dirty, setDirty] = useState(false);
    const [msg, setMsg] = useState('');
    const [err, setErr] = useState('');
    const [busy, setBusy] = useState(false);
    const taRef = useRef<HTMLTextAreaElement>(null);

    function updateContent(v: string) {
        setContent(v);
        setDirty(true);
    }

    // 打开远程文件
    async function loadRemote() {
        if (!remotePath.trim()) {
            setErr('请输入远程文件路径');
            return;
        }
        setBusy(true);
        setErr('');
        setMsg('');
        try {
            const text = await EditorLoadRemote(sessionId, remotePath.trim());
            setContent(text);
            setFilename(remotePath.split('/').pop() || '未命名');
            setDirty(false);
            setMsg('已加载远程文件');
            taRef.current?.focus();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setBusy(false);
        }
    }

    // 保存回传远程
    async function saveRemote() {
        if (!remotePath.trim()) {
            setErr('请输入远程文件路径');
            return;
        }
        setBusy(true);
        setErr('');
        setMsg('');
        try {
            await EditorSaveRemote(sessionId, remotePath.trim(), content);
            setDirty(false);
            setMsg('已写回远程 ✅');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setBusy(false);
        }
    }

    // 另存本地
    async function saveLocal() {
        setBusy(true);
        setErr('');
        setMsg('');
        try {
            // 用远程文件名作为默认保存名 (前端无法传默认名给系统对话框, 先保存到远程同名路径不可行)
            // 直接弹出保存对话框: 由用户选择本地路径
            const localPath = await PickFile();
            if (!localPath) return; // 用户取消
            await EditorSaveLocal(localPath, content);
            setDirty(false);
            setMsg(`已保存到本地: ${localPath}`);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setBusy(false);
        }
    }

    // 新建: 清空
    function newFile() {
        if (dirty && !window.confirm('当前内容未保存，确定新建？')) return;
        setContent('');
        setRemotePath('');
        setFilename('未命名');
        setDirty(false);
        setMsg('');
        setErr('');
        taRef.current?.focus();
    }

    return (
        <div className="editor-panel">
            <div className="editor-toolbar">
                <input
                    className="ep-path"
                    value={remotePath}
                    onChange={(e) => setRemotePath(e.target.value)}
                    placeholder="/etc/nginx/nginx.conf 或 /home/user/deploy.sh"
                    onKeyDown={(e) => e.key === 'Enter' && loadRemote()}
                />
                <button onClick={loadRemote} disabled={busy || !remotePath.trim()}>
                    打开远程
                </button>
                <button onClick={saveRemote} disabled={busy || !remotePath.trim() || !dirty}>
                    保存回传
                </button>
                <button onClick={saveLocal} disabled={busy || !content}>
                    存本地
                </button>
                <button onClick={newFile} disabled={busy}>
                    新建
                </button>
            </div>
            <div className="editor-status">
                <span className="ep-file">{filename}</span>
                <span className="ep-meta">
                    {content.length.toLocaleString()} 字符
                    {dirty ? ' · 未保存' : ''}
                </span>
                {msg && <span className="ep-msg">{msg}</span>}
            </div>
            {err && <div className="error-box">{err}</div>}
            <textarea
                ref={taRef}
                className="editor-textarea"
                value={content}
                onChange={(e) => updateContent(e.target.value)}
                spellCheck={false}
                placeholder={'在此编写文件内容...\n\n支持 shell / python / yaml / 配置文件等，\n编辑完成后「保存回传」写回远程，或「存本地」下载。'}
            />
        </div>
    );
}
