import { useEffect, useRef, useState } from 'react';
import { SearchHistory, ReadHistory, PickFile, EditorSaveLocal } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';

// LogPanel 日志面板: 跨会话关键字检索 → 命中文件 → 查看 (关键字高亮/行过滤) + 回放 + 书签/导出
export default function LogPanel() {
    const [kw, setKw] = useState('');
    const [matches, setMatches] = useState<model.HistoryMatch[]>([]);
    const [active, setActive] = useState<model.HistoryMatch | null>(null);
    const [content, setContent] = useState('');
    const [err, setErr] = useState('');
    const [searching, setSearching] = useState(false);

    // 回放状态
    const [playing, setPlaying] = useState(false);
    const [pos, setPos] = useState(0);
    const linesRef = useRef<string[]>([]);
    const timerRef = useRef<number | null>(null);

    // 书签: {path, line, text}[] 持久化到 localStorage
    const [bookmarks, setBookmarks] = useState<{ path: string; line: number; text: string }[]>(() => {
        try {
            return JSON.parse(localStorage.getItem('logBookmarks') || '[]');
        } catch {
            return [];
        }
    });

    function persistBookmarks(b: typeof bookmarks) {
        setBookmarks(b);
        localStorage.setItem('logBookmarks', JSON.stringify(b));
    }

    // 添加书签 (当前查看日志的当前行)
    function addBookmark() {
        if (!active) return;
        const line = playing ? pos : linesRef.current.length;
        const text = linesRef.current[Math.min(line, linesRef.current.length - 1)]?.trim().slice(0, 60) || '';
        persistBookmarks([...bookmarks, { path: active.path, line, text }]);
        setErr('');
    }

    // 跳转到书签对应日志
    async function gotoBookmark(b: { path: string; line: number }) {
        const hit = matches.find((m) => m.path === b.path);
        if (hit) {
            await open(hit);
            stopReplay();
            setPos(b.line);
            setPlaying(false);
        } else {
            setErr('书签对应的日志文件不在本次检索结果中，请先检索到它');
        }
    }

    // 删除书签
    function removeBookmark(i: number) {
        persistBookmarks(bookmarks.filter((_, idx) => idx !== i));
    }

    // 导出当前日志到本地
    async function exportLog() {
        if (!active || !content) return;
        try {
            const p = await PickFile();
            if (!p) return;
            await EditorSaveLocal(p, content);
            setErr('');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    async function doSearch() {
        if (!kw.trim()) return;
        setSearching(true);
        setErr('');
        setActive(null);
        try {
            setMatches((await SearchHistory(kw.trim())) ?? []);
            if (matches.length === 0) setErr('无命中');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setSearching(false);
        }
    }

    // 查看: 读取内容, 按行拆分为回放数据
    async function open(m: model.HistoryMatch) {
        stopReplay();
        setActive(m);
        setErr('');
        try {
            const text = await ReadHistory(m.path);
            setContent(text);
            linesRef.current = text.split('\n');
            setPos(0);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    function stopReplay() {
        setPlaying(false);
        if (timerRef.current) {
            clearInterval(timerRef.current);
            timerRef.current = null;
        }
    }

    // 回放: 每秒递增显示行数
    function toggleReplay() {
        if (!active) return;
        if (playing) {
            stopReplay();
            return;
        }
        setPlaying(true);
        timerRef.current = window.setInterval(() => {
            setPos((p) => {
                if (p >= linesRef.current.length) {
                    stopReplay();
                    return p;
                }
                return p + 4;
            });
        }, 80);
    }

    useEffect(() => stopReplay, []);

    // 行过滤 (仅显示含关键字的行)
    const lines = linesRef.current;
    const filtered = kw.trim()
        ? lines
              .map((l, i) => ({ l, i }))
              .filter((x) => x.l.toLowerCase().includes(kw.trim().toLowerCase()))
        : lines.map((l, i) => ({ l, i }));
    const visible = playing ? filtered.slice(0, pos) : filtered;
    const showFiltered = kw.trim() && active;

    return (
        <div className="log-panel">
            <div className="log-search">
                <input
                    value={kw}
                    onChange={(e) => setKw(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && doSearch()}
                    placeholder="检索关键字 (ERROR / 订单号 / IP 等), 回车搜索"
                    autoFocus
                />
                <button onClick={doSearch} disabled={searching}>
                    {searching ? '检索中...' : '检索'}
                </button>
            </div>
            {err && <div className="error-box">{err}</div>}
            {!active && (
                <div className="log-matches">
                    {matches.length === 0 && !searching && <div className="hist-empty">输入关键字回车，检索全部历史会话</div>}
                    {matches.map((m) => (
                        <div key={m.path} className="log-match" onClick={() => open(m)}>
                            <div className="lm-head">
                                <span className="lm-name">{m.name}</span>
                                <span className="lm-count">{m.count} 行命中</span>
                            </div>
                            {m.preview && <code className="lm-preview">{m.preview}</code>}
                        </div>
                    ))}
                </div>
            )}
            {active && (
                <div className="log-view">
                    <div className="log-toolbar">
                        <button onClick={() => setActive(null)}>← 返回结果</button>
                        <span className="lt-info">
                            {active.name} · {lines.length} 行
                            {showFiltered ? ` · 过滤 ${filtered.length} 行` : ''}
                            {playing ? ` · 回放 ${pos}/${lines.length}` : ''}
                        </span>
                        <button onClick={toggleReplay}>{playing ? '⏸ 暂停' : '▶ 回放'}</button>
                        <button onClick={addBookmark} title="将当前查看位置保存为书签">🔖 书签</button>
                        <button onClick={exportLog} title="导出当前日志到本地">💾 导出</button>
                        {playing && (
                            <button
                                onClick={() => {
                                    stopReplay();
                                    setPos(lines.length);
                                }}
                            >
                                跳到末尾
                            </button>
                        )}
                    </div>
                    {bookmarks.length > 0 && (
                        <div className="log-bookmarks">
                            {bookmarks.map((b, i) => (
                                <div key={i} className="lb-item">
                                    <span className="lb-info" onClick={() => gotoBookmark(b)} title="点击跳转">
                                        📍 L{b.line} · {b.text || b.path.split('/').pop()}
                                    </span>
                                    <button className="lb-del" onClick={() => removeBookmark(i)} title="删除书签">
                                        ×
                                    </button>
                                </div>
                            ))}
                        </div>
                    )}
                    <pre className="log-content">
                        {visible.map((x) => (
                            <div key={x.i} className="log-line">
                                {x.l || ' '}
                            </div>
                        ))}
                    </pre>
                </div>
            )}
        </div>
    );
}
