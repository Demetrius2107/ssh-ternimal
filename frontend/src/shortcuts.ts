// shortcuts.ts 快捷键系统: 动作映射 + localStorage 持久化 + 按键事件解析/匹配
//
// 设计: 每个动作可绑定多个组合键 (数组), 默认一组; 用户可在设置页新增/修改/删除键位。
// 持久化键 "shortcuts": { [actionId]: string[] } — 缺失动作回退默认。

export interface ShortcutAction {
    id: string;
    label: string; // 设置页展示名
    desc: string;  // 说明
    defaults: string[]; // 默认组合键
}

// 可绑定的动作清单 (执行逻辑在 App.tsx 全局 keydown 分发)
export const SHORTCUT_ACTIONS: ShortcutAction[] = [
    { id: 'openConnect', label: '新建连接', desc: '打开新建连接对话框', defaults: ['Ctrl+N'] },
    { id: 'openHistory', label: '历史日志', desc: '打开日志检索/回放面板', defaults: ['Ctrl+Shift+H'] }, // Ctrl+H 是终端退格, 避开
    { id: 'openTunnel', label: '隧道管理', desc: '打开 SSH 隧道面板', defaults: ['Ctrl+T'] },
    { id: 'openMonitor', label: '主机监控', desc: '打开主机监控', defaults: ['Ctrl+M'] },
    { id: 'openSnippets', label: '命令片段', desc: '打开命令片段面板', defaults: ['Ctrl+Shift+S'] },
    { id: 'openAudit', label: '会话审计', desc: '打开会话审计面板', defaults: ['Ctrl+Shift+A'] },
    { id: 'openSettings', label: '设置', desc: '打开设置页面', defaults: ['Ctrl+,'] },
    { id: 'terminalFind', label: '终端查找', desc: '终端内查找', defaults: ['Ctrl+F'] },
    { id: 'splitMode', label: '分屏', desc: '多会话分屏切换', defaults: ['Ctrl+Shift+W'] },
];

const STORE_KEY = 'shortcuts';

// 读取配置: {actionId: string[]}; 缺失动作回退默认
export function loadShortcuts(): Record<string, string[]> {
    const out: Record<string, string[]> = {};
    for (const a of SHORTCUT_ACTIONS) out[a.id] = [...a.defaults];
    try {
        const raw = localStorage.getItem(STORE_KEY);
        if (!raw) return out;
        const saved = JSON.parse(raw) as Record<string, string[]>;
        for (const a of SHORTCUT_ACTIONS) {
            const arr = saved[a.id];
            if (Array.isArray(arr) && arr.length > 0) out[a.id] = arr.filter((s) => typeof s === 'string');
        }
    } catch {
        /* 损坏配置回退默认 */
    }
    return out;
}

// 保存配置
export function saveShortcuts(map: Record<string, string[]>): void {
    localStorage.setItem(STORE_KEY, JSON.stringify(map));
}

// 恢复默认
export function defaultShortcuts(): Record<string, string[]> {
    const out: Record<string, string[]> = {};
    for (const a of SHORTCUT_ACTIONS) out[a.id] = [...a.defaults];
    return out;
}

// ---------- 按键事件 → 规范字符串 ----------
const KEY_NAMES: Record<string, string> = {
    Escape: 'Esc',
    Enter: 'Enter',
    Tab: 'Tab',
    Backspace: 'Backspace',
    Delete: 'Del',
    ArrowUp: '↑', ArrowDown: '↓', ArrowLeft: '←', ArrowRight: '→',
    PageUp: 'PgUp', PageDown: 'PgDn', Home: 'Home', End: 'End',
    ' ': 'Space',
};

// 事件 → "Ctrl+Shift+K" 形式
export function formatShortcut(e: KeyboardEvent): string {
    const parts: string[] = [];
    if (e.ctrlKey || e.metaKey) parts.push('Ctrl');
    if (e.altKey) parts.push('Alt');
    if (e.shiftKey) parts.push('Shift');
    let key = e.key;
    if (key === 'Control' || key === 'Meta' || key === 'Alt' || key === 'Shift') return ''; // 纯修饰键不记录
    key = KEY_NAMES[key] ?? (key.length === 1 ? key.toUpperCase() : key);
    parts.push(key);
    return parts.join('+');
}

// 判断事件是否命中某动作的任一组合键
export function matchShortcut(e: KeyboardEvent, actionId: string, map: Record<string, string[]>): boolean {
    const keys = map[actionId];
    if (!keys || keys.length === 0) return false;
    const fmt = formatShortcut(e);
    if (!fmt) return false;
    return keys.includes(fmt);
}

// 全局: 检测是否纯修饰键 (供录制时跳过)
export function isModifierOnly(e: KeyboardEvent): boolean {
    return ['Control', 'Meta', 'Alt', 'Shift'].includes(e.key);
}
