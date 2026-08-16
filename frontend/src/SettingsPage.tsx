import { useEffect, useState } from 'react';
import { AiSetKey, AiConfigure, AiStatus, CheckUpdate, DownloadUpdate, ApplyUpdate, VaultExport, VaultImport, ListCredentials, SaveCredential, DeleteCredential } from '../wailsjs/go/main/App';
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

type SectionId = 'appearance' | 'terminal' | 'shortcuts' | 'ai' | 'credentials' | 'vault' | 'about';

// CredentialSection 集中凭据管理 (Keychain): 一处修改, 引用该凭据的会话全局生效
function CredentialSection() {
    const [list, setList] = useState<model.CredentialListEntry[]>([]);
    const [name, setName] = useState('');
    const [credType, setCredType] = useState('password');
    const [username, setUsername] = useState('');
    const [secret, setSecret] = useState('');
    const [msg, setMsg] = useState('');
    const [err, setErr] = useState('');

    async function refresh() {
        try {
            setList((await ListCredentials()) ?? []);
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
        try {
            await SaveCredential(name.trim(), credType, username.trim(), secret);
            setName('');
            setSecret('');
            setUsername('');
            setMsg('凭据已保存');
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    async function doDelete(id: string) {
        if (!window.confirm('确认删除该凭据？引用它的会话将无法解析密码。')) return;
        try {
            await DeleteCredential(id);
            refresh();
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="sp-section">
            <h2 className="sp-h2">集中凭据 (Keychain)</h2>
            <p className="sp-desc">
                将常用密码/密钥集中保存，多个会话可引用同一凭据——一处修改，所有引用它的会话全局生效。
                凭据内容仅存于系统凭据库（加密），不落盘明文。
            </p>
            <div className="set-group">
                <div className="set-label">新建凭据</div>
                <input className="set-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="凭据名, 如 生产环境 root" />
                <select className="set-select" value={credType} onChange={(e) => setCredType(e.target.value)}>
                    <option value="password">密码</option>
                    <option value="privateKey">私钥</option>
                </select>
                <input className="set-input" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="用户名" />
                <input
                    className="set-input"
                    type="password"
                    value={secret}
                    onChange={(e) => setSecret(e.target.value)}
                    placeholder={credType === 'password' ? '密码' : '私钥 PEM 内容'}
                />
                <button className="set-save" onClick={doSave} disabled={!name.trim() || !username.trim() || !secret}>
                    保存凭据
                </button>
            </div>
            {msg && <div className="tunnel-msg">{msg}</div>}
            {err && <div className="error-box">{err}</div>}
            <div className="set-group">
                <div className="set-label">已保存凭据</div>
                <div className="sc-list">
                    {list.length === 0 && <div className="hist-empty">暂无凭据，先创建一个吧</div>}
                    {list.map((c) => (
                        <div key={c.id} className="sc-item">
                            <div className="sc-info">
                                <div className="sc-label">{c.name}</div>
                                <div className="sc-desc">
                                    {c.username} · {c.type === 'password' ? '密码' : '私钥'} · {c.createdAt}
                                </div>
                            </div>
                            <button className="si-del" onClick={() => doDelete(c.id)}>
                                删除
                            </button>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}

// VaultSection Vault 端到端加密备份: 导出全部会话/凭据为加密串, 或从加密串恢复
function VaultSection() {
    const [password, setPassword] = useState('');
    const [importData, setImportData] = useState('');
    const [exported, setExported] = useState('');
    const [busy, setBusy] = useState(false);
    const [msg, setMsg] = useState('');
    const [err, setErr] = useState('');

    async function doExport() {
        if (!password) {
            setErr('请设置备份密码 (用于加密, 恢复时需输入同一密码)');
            return;
        }
        setBusy(true);
        setErr('');
        setMsg('');
        try {
            const data = await VaultExport(password);
            setExported(data);
            setMsg(`已导出加密备份 (${Math.round(data.length / 1024)} KB)，请妥善保存`);
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setBusy(false);
        }
    }

    async function doImport() {
        if (!importData.trim() || !password) {
            setErr('请粘贴备份内容并输入备份密码');
            return;
        }
        setBusy(true);
        setErr('');
        setMsg('');
        try {
            await VaultImport(importData.trim(), password);
            setMsg('恢复完成 ✅ 会话与凭据已导入');
            setImportData('');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setBusy(false);
        }
    }

    function copyExport() {
        if (!exported) return;
        navigator.clipboard
            .writeText(exported)
            .then(() => setMsg('备份内容已复制到剪贴板'))
            .catch(() => setErr('复制失败，请手动选择复制'));
    }

    return (
        <div className="sp-section">
            <h2 className="sp-h2">Vault 备份</h2>
            <p className="sp-desc">
                端到端加密备份（AES-256-GCM）：将全部会话配置与凭据导出为加密字符串，可保存到任意位置或用于多端同步；恢复时输入同一密码即可解密。
            </p>
            <div className="set-group">
                <div className="set-label">备份密码</div>
                <input
                    className="set-input"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="加密/解密密码（务必牢记）"
                />
            </div>
            <div className="set-group">
                <div className="set-label">导出备份</div>
                <button className="set-save" onClick={doExport} disabled={busy || !password}>
                    {busy ? '处理中...' : '导出加密备份'}
                </button>
                {exported && (
                    <>
                        <textarea
                            className="vault-out"
                            value={exported}
                            readOnly
                            rows={4}
                            onClick={(e) => e.currentTarget.select()}
                        />
                        <button className="set-save" onClick={copyExport}>
                            复制备份内容
                        </button>
                    </>
                )}
            </div>
            <div className="set-group">
                <div className="set-label">从备份恢复</div>
                <textarea
                    className="vault-out"
                    value={importData}
                    onChange={(e) => setImportData(e.target.value)}
                    placeholder="粘贴加密备份内容..."
                    rows={3}
                />
                <button className="set-save" onClick={doImport} disabled={busy || !importData.trim() || !password}>
                    导入并恢复
                </button>
            </div>
            {msg && <div className="tunnel-msg">{msg}</div>}
            {err && <div className="error-box">{err}</div>}
        </div>
    );
}

// AboutSection 关于与更新: 版本信息 + 检查更新/下载/应用
function AboutSection() {
    const [checking, setChecking] = useState(false);
    const [downloading, setDownloading] = useState(false);
    const [update, setUpdate] = useState<model.UpdateInfo | null>(null);
    const [localPath, setLocalPath] = useState('');
    const [msg, setMsg] = useState('');
    const [err, setErr] = useState('');

    async function check() {
        setChecking(true);
        setErr('');
        setMsg('');
        setUpdate(null);
        setLocalPath('');
        try {
            const u = await CheckUpdate();
            setUpdate(u);
            if (!u.hasUpdate) setMsg('已是最新版本 ✅');
        } catch (e: any) {
            setErr(e?.message ?? '检查更新失败 (网络不可达?)');
        } finally {
            setChecking(false);
        }
    }

    async function download() {
        if (!update?.downloadUrl) {
            setErr('没有可用的下载地址');
            return;
        }
        setDownloading(true);
        setErr('');
        setMsg('');
        try {
            const p = await DownloadUpdate(update.downloadUrl);
            setLocalPath(p);
            setMsg('下载完成，可应用更新');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        } finally {
            setDownloading(false);
        }
    }

    async function apply() {
        if (!localPath) return;
        try {
            await ApplyUpdate(localPath, true);
            setMsg('已启动安装程序，安装完成后将更新到新版本');
        } catch (e: any) {
            setErr(e?.message ?? String(e));
        }
    }

    return (
        <div className="sp-section">
            <h2 className="sp-h2">关于</h2>
            <div className="about-card">
                <div className="about-logo">SSH</div>
                <div className="about-name">ssh-terminal</div>
                <div className="about-desc">本地优先的现代 SSH/Telnet 终端 · SFTP · 隧道 · AI 辅助 · 审计回放</div>
                <div className="about-stack">Wails (Go) + React + TypeScript + xterm.js</div>
                <div className="about-ver">v0.9.0 · 功能对标 Termius / XShell</div>
            </div>

            <div className="set-group">
                <div className="set-label">软件更新</div>
                <div className="upd-actions">
                    <button className="set-save" onClick={check} disabled={checking || downloading}>
                        {checking ? '检查中...' : '检查更新'}
                    </button>
                    {update?.hasUpdate && (
                        <>
                            <span className="upd-new">发现新版本 {update.latestVersion}</span>
                            <button className="set-save" onClick={download} disabled={downloading || !!localPath}>
                                {downloading ? '下载中...' : localPath ? '已下载' : '下载'}
                            </button>
                            {localPath && (
                                <button className="set-save" onClick={apply}>
                                    应用更新
                                </button>
                            )}
                        </>
                    )}
                </div>
                {update?.notes && <div className="upd-notes">{update.notes}</div>}
                {msg && <div className="tunnel-msg">{msg}</div>}
                {err && <div className="error-box">{err}</div>}
            </div>
        </div>
    );
}

const SECTIONS: Array<{ id: SectionId; label: string }> = [
    { id: 'appearance', label: '外观' },
    { id: 'terminal', label: '终端' },
    { id: 'shortcuts', label: '快捷键' },
    { id: 'ai', label: 'AI 辅助' },
    { id: 'credentials', label: '凭据' },
    { id: 'vault', label: 'Vault 备份' },
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

                {section === 'credentials' && (
                    <CredentialSection />
                )}

                {section === 'vault' && (
                    <VaultSection />
                )}

                {section === 'about' && (
                    <AboutSection />
                )}
            </div>
        </div>
    );
}
