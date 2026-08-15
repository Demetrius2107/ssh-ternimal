import { useEffect, useState } from 'react';
import { AiSetKey, AiConfigure, AiStatus } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { THEMES, THEME_LIST, type ThemeName } from './themes';
import { SHORTCUT_ACTIONS, loadShortcuts, saveShortcuts, defaultShortcuts, formatShortcut, isModifierOnly } from './shortcuts';

interface Props {
    theme: ThemeName;
    onTheme: (t: ThemeName) => void;
    fontFamily: string;
    onFontFamily: (f: string) => void;
    fontSize: number;
    onFontSize: (n: number) => void;
    onClose: () => void;
}

// 终端字体选项
const FONT_OPTIONS: Array<[string, string]> = [
    ['Consolas, "Courier New", monospace', 'Consolas (默认)'],
    ['"Courier New", Consolas, monospace', 'Courier New'],
    ['"JetBrains Mono", Consolas, monospace', 'JetBrains Mono'],
    ['"SF Mono", Consolas, monospace', 'SF Mono'],
    ['"Microsoft YaHei", Consolas, monospace', '微软雅黑'],
    ['monospace', '系统等宽'],
];

type SectionId = 'appearance' | 'terminal' | 'shortcuts' | 'ai' | 'about';

const SECTIONS: Array<{ id: SectionId; label: string }> = [
    { id: 'appearance', label: '外观' },
    { id: 'terminal', label: '终端' },
    { id: 'shortcuts', label: '快捷键' },
    { id: 'ai', label: 'AI 辅助' },
    { id: 'about', label: '关于' },
];

// SettingsPage 独立设置页面: 左侧目录树 + 右侧分层配置 (Notion/XShell 风格)
export default function SettingsPage({ theme, onTheme, fontFamily, onFontFamily, fontSize, onFontSize, onClose }: Props) {
    const [section, setSection] = useState<SectionId>('appearance');

    // AI 配置状态
    const [aiProvider, setAiProvider] = useState('deepseek');
    const [aiModel, setAiModel] = useState('deepseek-chat');
    const [aiKey, setAiKey] = useState('');
    const [aiLimit, setAiLimit] = useState(5_000_000);
    const [aiStatus, setAiStatus] = useState<model.AiStatus | null>(null);
    const [aiMsg, setAiMsg] = useState('');
    const [aiErr, setAiErr] = useState('');

    const [shortcutMap, setShortcutMap] = useState<Record<string, string[]>>(() => loadShortcuts());
    const [recording, setRecording] = useState<string | null>(null); // 正在录制的动作 id

    // 录制按键: 捕获阶段监听, 按下组合键即保存
    useEffect(() => {
        if (!recording) return;
        const onKey = (e: KeyboardEvent) => {
            e.preventDefault();
            e.stopPropagation();
            if (e.key === 'Escape') {
                setRecording(null); // ESC 取消录制 (不保存)
                return;
            }
            if (isModifierOnly(e)) return;
            const fmt = formatShortcut(e);
            if (!fmt) return;
            setShortcutMap((prev) => {
                const next = { ...prev };
                const keys = next[recording] ?? [];
                if (!keys.includes(fmt)) keys.push(fmt);
                next[recording] = keys;
                saveShortcuts(next);
                return next;
            });
            setRecording(null);
        };
        window.addEventListener('keydown', onKey, true); // capture: 先于全局快捷键执行
        return () => window.removeEventListener('keydown', onKey, true);
    }, [recording]);

    // 删除某个动作的某个组合键
    function removeKey(actionId: string, key: string) {
        setShortcutMap((prev) => {
            const next = { ...prev };
            next[actionId] = (next[actionId] ?? []).filter((k) => k !== key);
            saveShortcuts(next);
            return next;
        });
    }

    // 恢复默认
    function resetShortcuts() {
        const def = defaultShortcuts();
        setShortcutMap(def);
        saveShortcuts(def);
    }

    useEffect(() => {
        AiStatus()
            .then((s) => {
                setAiStatus(s);
                setAiProvider(s.provider || 'deepseek');
                setAiModel(s.model || 'deepseek-chat');
                setAiLimit(s.monthlyLimit || 5_000_000);
            })
            .catch(() => undefined);
    }, []);

    async function saveAi() {
        setAiMsg('');
        setAiErr('');
        try {
            AiConfigure(aiProvider, aiModel, aiLimit);
            if (aiProvider === 'deepseek' && aiKey.trim()) {
                await AiSetKey(aiKey.trim());
                setAiKey('');
            }
            setAiStatus(await AiStatus());
            setAiMsg('AI 配置已保存');
        } catch (e: any) {
            setAiErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="settings-page">
            {/* 左侧目录树 */}
            <aside className="sp-side">
                <div className="sp-side-title">设置</div>
                <nav className="sp-nav">
                    {SECTIONS.map((s) => (
                        <button
                            key={s.id}
                            className={`sp-nav-item ${section === s.id ? 'active' : ''}`}
                            onClick={() => setSection(s.id)}
                        >
                            {s.label}
                        </button>
                    ))}
                </nav>
                <button className="sp-back" onClick={onClose}>
                    ← 返回
                </button>
            </aside>

            {/* 右侧内容区 */}
            <div className="sp-main">
                {section === 'appearance' && (
                    <div className="sp-section">
                        <h2 className="sp-h2">外观</h2>
                        <div className="set-group">
                            <div className="set-label">主题</div>
                            <select className="set-select" value={theme} onChange={(e) => onTheme(e.target.value as ThemeName)}>
                                {THEME_LIST.map((t) => (
                                    <option key={t} value={t}>
                                        {THEMES[t].label}
                                    </option>
                                ))}
                            </select>
                        </div>
                        <div className="set-group">
                            <div className="set-label">预览</div>
                            <div className="theme-swatch" style={{ background: THEMES[theme].chromeBg, color: '#ccc' }}>
                                <span className="ts-dot" style={{ background: THEMES[theme].accent }} />
                                {THEMES[theme].label} · 强调色 {THEMES[theme].accent}
                            </div>
                        </div>
                    </div>
                )}

                {section === 'terminal' && (
                    <div className="sp-section">
                        <h2 className="sp-h2">终端</h2>
                        <div className="set-group">
                            <div className="set-label">字体</div>
                            <select className="set-select" value={fontFamily} onChange={(e) => onFontFamily(e.target.value)}>
                                {FONT_OPTIONS.map(([val, label]) => (
                                    <option key={val} value={val}>
                                        {label}
                                    </option>
                                ))}
                            </select>
                        </div>
                        <div className="set-group">
                            <div className="set-label">字号</div>
                            <div className="set-size-row">
                                <input
                                    type="range"
                                    min={11}
                                    max={22}
                                    step={1}
                                    value={fontSize}
                                    onChange={(e) => onFontSize(Number(e.target.value))}
                                />
                                <span className="set-size-val">{fontSize}px</span>
                            </div>
                        </div>
                    </div>
                )}

                {section === 'shortcuts' && (
                    <div className="sp-section">
                        <h2 className="sp-h2">快捷键</h2>
                        <p className="sp-desc">点击组合键可重新录制；支持新增多个键位、删除、恢复默认。修改立即生效并本地保存。</p>
                        <div className="sc-list">
                            {SHORTCUT_ACTIONS.map((a) => {
                                const keys = shortcutMap[a.id] ?? a.defaults;
                                return (
                                    <div key={a.id} className={`sc-item ${recording === a.id ? 'recording' : ''}`}>
                                        <div className="sc-info">
                                            <div className="sc-label">{a.label}</div>
                                            <div className="sc-desc">{a.desc}</div>
                                        </div>
                                        <div className="sc-keys">
                                            {keys.length === 0 && <span className="sc-none">未绑定</span>}
                                            {keys.map((k, i) => (
                                                <span key={i} className="sc-kbd-wrap">
                                                    <kbd className="sc-kbd">{k}</kbd>
                                                    <button
                                                        className="sc-kbd-x"
                                                        title="删除此键位"
                                                        onClick={() => removeKey(a.id, k)}
                                                    >
                                                        ×
                                                    </button>
                                                </span>
                                            ))}
                                            <button
                                                className="sc-record"
                                                onClick={() => setRecording(recording === a.id ? null : a.id)}
                                                title={recording === a.id ? '点击取消录制' : '录制新组合键'}
                                            >
                                                {recording === a.id ? '录制中... 按组合键 (ESC 取消)' : '＋ 新增'}
                                            </button>
                                        </div>
                                    </div>
                                );
                            })}
                        </div>
                        <div className="sc-actions">
                            <button className="set-save" onClick={resetShortcuts}>
                                恢复默认
                            </button>
                            <span className="sc-hint">快捷键全局生效，终端内查找等动作也会跟随新键位。</span>
                        </div>
                    </div>
                )}

                {section === 'ai' && (
                    <div className="sp-section">
                        <h2 className="sp-h2">AI 辅助</h2>
                        <div className="set-group">
                            <div className="set-label">服务</div>
                            <select className="set-select" value={aiProvider} onChange={(e) => setAiProvider(e.target.value)}>
                                <option value="deepseek">DeepSeek (云端, 需 API Key)</option>
                                <option value="ollama">Ollama (本地, 无需 Key)</option>
                            </select>
                        </div>
                        <div className="set-group">
                            <div className="set-label">模型</div>
                            <input className="set-input" value={aiModel} onChange={(e) => setAiModel(e.target.value)} placeholder="deepseek-chat / qwen2.5" />
                        </div>
                        {aiProvider === 'deepseek' && (
                            <div className="set-group">
                                <div className="set-label">API Key</div>
                                <input
                                    className="set-input"
                                    type="password"
                                    value={aiKey}
                                    onChange={(e) => setAiKey(e.target.value)}
                                    placeholder={aiStatus?.keyConfigured ? '已配置 (留空不改)' : 'DeepSeek API Key'}
                                />
                            </div>
                        )}
                        <div className="set-group">
                            <div className="set-label">月度限额</div>
                            <div className="set-size-row">
                                <input
                                    type="range"
                                    min={100_000}
                                    max={20_000_000}
                                    step={100_000}
                                    value={aiLimit}
                                    onChange={(e) => setAiLimit(Number(e.target.value))}
                                />
                                <span className="set-size-val">{(aiLimit / 1_000_000).toFixed(1)}M/月</span>
                            </div>
                        </div>
                        {aiStatus && (
                            <div className="set-hint">
                                当月已用 {aiStatus.monthUsage.toLocaleString()} / {aiStatus.monthlyLimit.toLocaleString()} token
                                {aiProvider === 'deepseek' && !aiStatus.keyConfigured ? ' · API Key 未配置' : ''}
                            </div>
                        )}
                        <button className="set-save" onClick={saveAi}>
                            保存 AI 配置
                        </button>
                        {aiMsg && <div className="tunnel-msg">{aiMsg}</div>}
                        {aiErr && <div className="error-box">{aiErr}</div>}
                    </div>
                )}

                {section === 'about' && (
                    <div className="sp-section">
                        <h2 className="sp-h2">关于</h2>
                        <div className="about-card">
                            <div className="about-logo">SSH</div>
                            <div className="about-name">ssh-terminal</div>
                            <div className="about-desc">
                                本地优先的现代 SSH/Telnet 终端 · SFTP · 隧道 · AI 辅助 · 审计回放
                            </div>
                            <div className="about-stack">Wails (Go) + React + TypeScript + xterm.js</div>
                            <div className="about-ver">v0.9 · 功能对标 Termius / XShell</div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
