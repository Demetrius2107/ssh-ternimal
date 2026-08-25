import { useRef, useState } from 'react';
import { EditorLoadRemote, EditorSaveRemote, EditorSaveLocal, PickFile } from '../wailsjs/go/main/App';

// ---------- 轻量语法高亮 (逐行 token 扫描, 覆盖 shell/conf/yaml/json/python 等运维场景) ----------
// HTML 完全由代码构建: 先 tokenize 再逐段转义, 避免正则误匹配已生成的标签属性
// (如关键词 class 不能匹配到 <span class="hl-..."> 的 class 属性)

const KEYWORDS = new Set([
    'if', 'then', 'else', 'elif', 'fi', 'for', 'in', 'do', 'done', 'while', 'case', 'esac',
    'function', 'return', 'export', 'local', 'echo', 'printf', 'cd', 'exit', 'set', 'unset',
    'source', 'alias', 'def', 'class', 'import', 'from', 'as', 'try', 'except', 'finally',
    'with', 'raise', 'yield', 'lambda', 'pass', 'break', 'continue', 'True', 'False', 'None',
    'and', 'or', 'not', 'if',
]);

function escapeHtml(s: string): string {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// 单行 token 扫描: 把行拆成 (文本, 类型) 片段
function tokenizeLine(line: string): Array<{ text: string; cls: string | null }> {
    const segs: Array<{ text: string; cls: string | null }> = [];
    const re = /('(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"|\b\d+(?:\.\d+)?\b|\b[a-zA-Z_][\w.-]*\b|.)/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(line)) !== null) {
        const t = m[0];
        let cls: string | null = null;
        if ((t.startsWith("'") && t.endsWith("'")) || (t.startsWith('"') && t.endsWith('"'))) {
            cls = 'hl-string';
        } else if (/^\d+(\.\d+)?$/.test(t)) {
            cls = 'hl-number';
        } else if (/^[a-zA-Z_][\w-]*$/.test(t) && KEYWORDS.has(t)) {
            cls = 'hl-keyword';
        }
        segs.push({ text: t, cls });
    }
    return segs;
}

function highlight(code: string): string {
    const out: string[] = [];
    for (const rawLine of code.split('\n')) {
        // 1. 注释行 (行首 # 或 //, 允许前导空白; 不匹配 shebang)
        const cm = rawLine.match(/^(\s*)(#!|#|\/\/)(.*)$/);
        if (cm) {
            const cls = cm[2] === '#!' ? 'hl-shebang' : 'hl-comment';
            out.push(cm[1] + `<span class="${cls}">${escapeHtml(cm[2] + cm[3])}</span>`);
            continue;
        }
        // 2. yaml/conf 键名: 行首 word:
        const am = rawLine.match(/^(\s*)([\w.-]+):(\s*)$/);
        if (am) {
            out.push(am[1] + `<span class="hl-attr">${escapeHtml(am[2])}</span>:` + escapeHtml(am[3]));
            continue;
        }
        // 3. 其余: token 扫描
        let html = '';
        for (const seg of tokenizeLine(rawLine)) {
            const text = escapeHtml(seg.text);
            html += seg.cls ? `<span class="${seg.cls}">${text}</span>` : text;
        }
        out.push(html);
    }
    // 尾部换行让 pre 高度与 textarea 一致
    return out.join('\n') + '\n';
}

// EditorPanel 内置文本编辑器: 作为 SFTP 上传/下载的中间态
// 打开远程文件 → 本地编辑 → 保存回传远程 / 另存本地; 支持新建文件
// 语法高亮: textarea 文字透明 + 下方 pre 渲染高亮 HTML, 两者同步滚动
export default function EditorPanel({ sessionId }: { sessionId: number }) {
    const [remotePath, setRemotePath] = useState('');
    const [content, setContent] = useState('');
    const [filename, setFilename] = useState('未命名');
    const [dirty, setDirty] = useState(false);
    const [msg, setMsg] = useState('');
    const [err, setErr] = useState('');
    const [busy, setBusy] = useState(false);
    const taRef = useRef<HTMLTextAreaElement>(null);
    const preRef = useRef<HTMLPreElement>(null);

    function updateContent(v: string) {
        setContent(v);
        setDirty(true);
    }

    // 同步 textarea 和高亮 pre 的滚动位置
    function onScroll() {
        if (preRef.current && taRef.current) {
            preRef.current.scrollTop = taRef.current.scrollTop;
            preRef.current.scrollLeft = taRef.current.scrollLeft;
        }
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
            <div className="editor-wrap">
                {/* 高亮层: 在 textarea 下方渲染彩色 HTML, 不可交互 */}
                <pre
                    ref={preRef}
                    className="editor-highlight"
                    dangerouslySetInnerHTML={{ __html: highlight(content) }}
                    aria-hidden="true"
                />
                {/* 编辑层: 文字透明, 覆盖在高亮层之上, 光标可见 */}
                <textarea
                    ref={taRef}
                    className="editor-textarea"
                    value={content}
                    onChange={(e) => updateContent(e.target.value)}
                    onScroll={onScroll}
                    spellCheck={false}
                    placeholder={'在此编写文件内容...\n\n支持 shell / python / yaml / 配置文件等，\n编辑完成后「保存回传」写回远程，或「存本地」下载。'}
                />
            </div>
        </div>
    );
}
