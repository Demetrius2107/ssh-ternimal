import { useEffect, useState } from 'react';
import { AiSetKey, AiConfigure, AiStatus } from '../wailsjs/go/main/App';
import { model } from '../wailsjs/go/models';
import { THEMES, THEME_LIST, type ThemeName } from './themes';

interface Props {
    theme: ThemeName;
    onTheme: (t: ThemeName) => void;
    fontFamily: string;
    onFontFamily: (f: string) => void;
    fontSize: number;
    onFontSize: (n: number) => void;
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

const SHORTCUTS: Array<[string, string]> = [
    ['Ctrl+F', '终端内查找'],
    ['ESC', '关闭弹窗 / 右键菜单'],
    ['右键 → 发送片段', '快速执行命令片段'],
    ['双击文件', '进入目录 (SFTP)'],
];

// SettingsPanel 设置面板: 主题 / 终端字体字号 / AI 配置 / 快捷键说明
export default function SettingsPanel({ theme, onTheme, fontFamily, onFontFamily, fontSize, onFontSize }: Props) {
    const [aiProvider, setAiProvider] = useState('deepseek');
    const [aiModel, setAiModel] = useState('deepseek-chat');
    const [aiKey, setAiKey] = useState('');
    const [aiLimit, setAiLimit] = useState(5_000_000);
    const [aiStatus, setAiStatus] = useState<model.AiStatus | null>(null);
    const [aiMsg, setAiMsg] = useState('');
    const [aiErr, setAiErr] = useState('');

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
        <div className="settings-panel">
            <div className="set-group">
                <div className="set-label">外观主题</div>
                <select className="set-select" value={theme} onChange={(e) => onTheme(e.target.value as ThemeName)}>
                    {THEME_LIST.map((t) => (
                        <option key={t} value={t}>
                            {THEMES[t].label}
                        </option>
                    ))}
                </select>
            </div>
            <div className="set-group">
                <div className="set-label">终端字体</div>
                <select
                    className="set-select"
                    value={fontFamily}
                    onChange={(e) => onFontFamily(e.target.value)}
                >
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
            <div className="set-group">
                <div className="set-label">AI 辅助</div>
                <select className="set-select" value={aiProvider} onChange={(e) => setAiProvider(e.target.value)}>
                    <option value="deepseek">DeepSeek (云端, 需 API Key)</option>
                    <option value="ollama">Ollama (本地, 无需 Key)</option>
                </select>
                <input
                    className="set-input"
                    value={aiModel}
                    onChange={(e) => setAiModel(e.target.value)}
                    placeholder="模型, 如 deepseek-chat / qwen2.5"
                />
                {aiProvider === 'deepseek' && (
                    <input
                        className="set-input"
                        type="password"
                        value={aiKey}
                        onChange={(e) => setAiKey(e.target.value)}
                        placeholder={aiStatus?.keyConfigured ? '已配置 (留空不改)' : 'DeepSeek API Key'}
                    />
                )}
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
            <div className="set-group">
                <div className="set-label">快捷键</div>
                <div className="set-shortcuts">
                    {SHORTCUTS.map(([k, v]) => (
                        <div key={k} className="set-sc-row">
                            <kbd>{k}</kbd>
                            <span>{v}</span>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    );
}
