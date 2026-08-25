import { useEffect, useState } from 'react';
import {
    SftpPwd,
    SftpListDir,
    SftpMkdir,
    SftpDelete,
    SftpRename,
    SftpUpload,
    SftpDownload,
    EditRemoteFile,
    LocalListDir,
    LocalParent,
    LocalMkdir,
    LocalDelete,
    LocalRename,
    PickFile,
    PickDir,
} from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { EventsOn, EventsOff, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime';

function formatSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
    return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function remoteParent(p: string): string {
    const idx = p.lastIndexOf('/');
    if (idx <= 0) return '/';
    return p.substring(0, idx);
}

function basename(p: string): string {
    return p.split(/[\\/]/).pop() || p;
}

export default function FilePanel({ sessionId }: { sessionId: number }) {
    const [localDir, setLocalDir] = useState('C:\\');
    const [localEntries, setLocalEntries] = useState<model.FileEntry[]>([]);
    const [remoteDir, setRemoteDir] = useState('');
    const [remoteEntries, setRemoteEntries] = useState<model.FileEntry[]>([]);
    const [localInput, setLocalInput] = useState('C:\\');
    const [remoteInput, setRemoteInput] = useState('');
    const [selLocal, setSelLocal] = useState<model.FileEntry | null>(null);
    const [selRemote, setSelRemote] = useState<model.FileEntry | null>(null);
    const [transfers, setTransfers] = useState<Record<number, model.TransferTask>>({});
    const [conflict, setConflict] = useState<'overwrite' | 'skip' | 'rename'>('overwrite');
    const [error, setError] = useState('');
    const [dragOver, setDragOver] = useState(false); // 拖拽悬停高亮

    async function loadLocal(dir: string) {
        try {
            setLocalEntries(await LocalListDir(dir));
            setLocalDir(dir);
            setLocalInput(dir);
            setSelLocal(null);
            setError('');
        } catch (e: any) {
            setError(`本地: ${e?.message ?? e}`);
        }
    }

    async function loadRemote(dir: string) {
        try {
            setRemoteEntries(await SftpListDir(sessionId, dir));
            setRemoteDir(dir);
            setRemoteInput(dir);
            setSelRemote(null);
            setError('');
        } catch (e: any) {
            setError(`远程: ${e?.message ?? e}`);
        }
    }

    useEffect(() => {
        loadLocal('C:\\');
        SftpPwd(sessionId)
            .then((pwd) => loadRemote(pwd))
            .catch((e: any) => setError(`远程: ${e?.message ?? e}`));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sessionId]);

    useEffect(() => {
        const onProgress = (t: model.TransferTask) => {
            setTransfers((prev) => ({ ...prev, [t.taskId]: t }));
        };
        EventsOn('sftp-progress', onProgress);
        return () => {
            EventsOff('sftp-progress');
        };
    }, []);

    async function refreshAll() {
        await loadLocal(localDir);
        await loadRemote(remoteDir);
    }

    async function mkdirRemote() {
        const name = window.prompt('远程新建目录名:');
        if (!name) return;
        try {
            await SftpMkdir(sessionId, `${remoteDir}/${name}`);
            loadRemote(remoteDir);
        } catch (e: any) {
            setError(`远程: ${e?.message ?? e}`);
        }
    }

    async function mkdirLocal() {
        const name = window.prompt('本地新建目录名:');
        if (!name) return;
        try {
            await LocalMkdir(`${localDir}\\${name}`);
            loadLocal(localDir);
        } catch (e: any) {
            setError(`本地: ${e?.message ?? e}`);
        }
    }

    async function deleteRemote() {
        if (!selRemote) return;
        if (!window.confirm(`确认删除远程 ${selRemote.path} ?`)) return;
        try {
            await SftpDelete(sessionId, selRemote.path);
            loadRemote(remoteDir);
        } catch (e: any) {
            setError(`远程: ${e?.message ?? e}`);
        }
    }

    async function deleteLocal() {
        if (!selLocal) return;
        if (!window.confirm(`确认删除本地 ${selLocal.path} ?`)) return;
        try {
            await LocalDelete(selLocal.path);
            loadLocal(localDir);
        } catch (e: any) {
            setError(`本地: ${e?.message ?? e}`);
        }
    }

    async function renameRemote() {
        if (!selRemote) return;
        const name = window.prompt('远程重命名 (新名称):', selRemote.name);
        if (!name || name === selRemote.name) return;
        try {
            await SftpRename(sessionId, selRemote.path, `${remoteDir}/${name}`);
            loadRemote(remoteDir);
        } catch (e: any) {
            setError(`远程: ${e?.message ?? e}`);
        }
    }

    async function renameLocal() {
        if (!selLocal) return;
        const name = window.prompt('本地重命名 (新名称):', selLocal.name);
        if (!name || name === selLocal.name) return;
        try {
            await LocalRename(selLocal.path, `${localDir}\\${name}`);
            loadLocal(localDir);
        } catch (e: any) {
            setError(`本地: ${e?.message ?? e}`);
        }
    }

    async function doUpload() {
        if (!selLocal) {
            setError('请先在左侧选中本地文件或目录');
            return;
        }
        try {
            await SftpUpload(sessionId, selLocal.path, `${remoteDir}/${basename(selLocal.path)}`, conflict);
            setError('');
        } catch (e: any) {
            setError(`上传: ${e?.message ?? e}`);
        }
    }

    async function uploadFromDialog(mode: 'file' | 'dir') {
        try {
            const p = mode === 'file' ? await PickFile() : await PickDir();
            if (!p) return;
            await SftpUpload(sessionId, p, `${remoteDir}/${basename(p)}`, conflict);
            setError('');
        } catch (e: any) {
            setError(`上传: ${e?.message ?? e}`);
        }
    }

    // 拖拽上传: 用 Wails OnFileDrop 监听, 拖入的文件路径由运行时提供 (比读 File.path 可靠)
    // useDropTarget=true 仅在落点带 --wails-drop-target 的元素上触发
    useEffect(() => {
        OnFileDrop((_x, _y, paths) => {
            setDragOver(false);
            if (!paths || paths.length === 0) return;
            for (const p of paths) {
                if (!p) continue;
                SftpUpload(sessionId, p, `${remoteDir}/${basename(p)}`, conflict)
                    .catch((err: any) => setError(`拖拽上传 ${basename(p)} 失败: ${err?.message ?? err}`));
            }
        }, true);
        return () => OnFileDropOff();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [remoteDir, conflict, sessionId]);

    async function doDownload() {
        if (!selRemote) {
            setError('请先在右侧选中远程文件或目录');
            return;
        }
        try {
            await SftpDownload(sessionId, selRemote.path, `${localDir}\\${basename(selRemote.path)}`, conflict);
            setError('');
        } catch (e: any) {
            setError(`下载: ${e?.message ?? e}`);
        }
    }

    // 远程编辑: 后端下载到临时文件 → 系统编辑器打开 → 关闭后自动回传
    async function editRemote() {
        if (!selRemote || selRemote.isDir) {
            setError('请先在右侧选中远程文件');
            return;
        }
        try {
            setError('远程编辑: 已打开编辑器, 保存并关闭后自动回传…');
            await EditRemoteFile(sessionId, selRemote.path);
            setError('远程编辑: 已回传 ✅');
            loadRemote(remoteDir);
        } catch (e: any) {
            setError(`远程编辑: ${e?.message ?? e}`);
        }
    }

    const transferList = Object.values(transfers);

    return (
        <div className="file-panel">
            {error && <div className="error-box file-error">{error}</div>}
            <div className="file-panes">
                {/* 本地栏 */}
                <div className="pane">
                    <div className="pane-header">
                        <button onClick={() => LocalParent(localDir).then((p) => loadLocal(p)).catch((e: any) => setError(`本地: ${e?.message ?? e}`))}>⬆</button>
                        <input
                            value={localInput}
                            onChange={(e) => setLocalInput(e.target.value)}
                            onKeyDown={(e) => e.key === 'Enter' && loadLocal(localInput)}
                        />
                    </div>
                    <div className="pane-actions">
                        <button onClick={mkdirLocal}>新建目录</button>
                        <button onClick={deleteLocal} disabled={!selLocal}>删除</button>
                        <button onClick={renameLocal} disabled={!selLocal}>重命名</button>
                        <button onClick={loadLocal.bind(null, localDir)}>刷新</button>
                    </div>
                    <div className="file-list">
                        {localEntries.length === 0 ? (
                            <div className="file-empty">空目录</div>
                        ) : (
                            localEntries.map((e) => (
                                <div
                                    key={e.path}
                                    className={`file-row ${e.isDir ? 'row-dir' : 'row-file'} ${selLocal?.path === e.path ? 'selected' : ''}`}
                                    onClick={() => setSelLocal(e)}
                                    onDoubleClick={() => e.isDir && loadLocal(e.path)}
                                >
                                    <span className="f-name">{e.isDir ? '📁 ' : '📄 '}{e.name}</span>
                                    <span className="f-size">{e.isDir ? '' : formatSize(e.size)}</span>
                                    <span className="f-time">{e.modTime}</span>
                                </div>
                            ))
                        )}
                    </div>
                </div>
                {/* 远程栏 */}
                <div className="pane">
                    <div className="pane-header">
                        <button onClick={() => loadRemote(remoteParent(remoteDir))}>⬆</button>
                        <input
                            value={remoteInput}
                            onChange={(e) => setRemoteInput(e.target.value)}
                            onKeyDown={(e) => e.key === 'Enter' && loadRemote(remoteInput)}
                        />
                    </div>
                    <div className="pane-actions">
                        <button onClick={mkdirRemote}>新建目录</button>
                        <button onClick={deleteRemote} disabled={!selRemote}>删除</button>
                        <button onClick={renameRemote} disabled={!selRemote}>重命名</button>
                        <button onClick={editRemote} disabled={!selRemote || selRemote.isDir} title="下载到本地, 用系统编辑器打开, 关闭后自动回传">
                            编辑
                        </button>
                        <button onClick={loadRemote.bind(null, remoteDir)}>刷新</button>
                    </div>
                    <div
                        className={`file-list wails-drop-target ${dragOver ? 'drag-over' : ''}`}
                        style={{ ['--wails-drop-target' as any]: 'drag' }}
                        onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
                        onDragLeave={() => setDragOver(false)}
                    >
                        {remoteEntries.length === 0 ? (
                            <div className="file-empty">{dragOver ? '松开以上传到当前目录' : '空目录 (可将文件拖拽到此处上传)'}</div>
                        ) : (
                            remoteEntries.map((e) => (
                                <div
                                    key={e.path}
                                    className={`file-row ${e.isDir ? 'row-dir' : 'row-file'} ${selRemote?.path === e.path ? 'selected' : ''}`}
                                    onClick={() => setSelRemote(e)}
                                    onDoubleClick={() => e.isDir && loadRemote(e.path)}
                                >
                                    <span className="f-name">{e.isDir ? '📁 ' : '📄 '}{e.name}</span>
                                    <span className="f-size">{e.isDir ? '' : formatSize(e.size)}</span>
                                    <span className="f-time">{e.modTime}</span>
                                </div>
                            ))
                        )}
                    </div>
                </div>
            </div>
            {/* 传输动作栏 */}
            <div className="transfer-actions">
                <button onClick={() => uploadFromDialog('file')}>上传文件…</button>
                <button onClick={() => uploadFromDialog('dir')}>上传目录…</button>
                <button onClick={doUpload} disabled={!selLocal}>上传选中</button>
                <button onClick={doDownload} disabled={!selRemote}>下载选中</button>
                <select
                    className="conflict-select"
                    value={conflict}
                    onChange={(e) => setConflict(e.target.value as 'overwrite' | 'skip' | 'rename')}
                    title="同名文件冲突策略"
                >
                    <option value="overwrite">覆盖</option>
                    <option value="skip">跳过</option>
                    <option value="rename">自动改名</option>
                </select>
                <button onClick={refreshAll}>刷新全部</button>
            </div>
            {/* 传输队列 */}
            {transferList.length > 0 && (
                <div className="transfers">
                    {transferList.map((t) => {
                        const pct = t.size > 0 ? Math.min(100, Math.round((t.transferred / t.size) * 100)) : 0;
                        return (
                            <div key={t.taskId} className="transfer-row">
                                <span className={`t-status t-${t.status}`}>{t.direction === 'upload' ? '↑' : '↓'}</span>
                                <span className="t-name">
                                    {t.direction === 'upload' ? t.localPath : t.remotePath}
                                    {t.isDir ? ' (📁 目录)' : ''}
                                    {t.status === 'running' && t.currentFile ? ` → ${t.currentFile}` : ''}
                                </span>
                                <span className="t-pct">
                                    {t.status === 'error'
                                        ? t.error
                                        : `${formatSize(t.transferred)} / ${formatSize(t.size)} (${pct}%)`}
                                </span>
                                <div className="t-bar">
                                    <div className="t-bar-fill" style={{ width: `${pct}%` }} />
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
