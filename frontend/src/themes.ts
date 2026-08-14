// 多主题配色预设 (苹果风 Liquid Glass, 系统蓝 #0088FF)
export type ThemeName = 'dark' | 'light' | 'midnight' | 'graphite' | 'solar' | 'forest';

export interface ThemePreset {
    label: string;
    mode: 'dark' | 'light';
    accent: string; // 强调色 (--accent)
    chromeBg: string; // 主背景 (--bg)
    chromeBar: string; // 工具栏/面板背景 (--bar)
    xterm: Record<string, string>; // xterm.js theme
}

export const THEMES: Record<ThemeName, ThemePreset> = {
    dark: {
        label: '暗黑 Homebrew',
        mode: 'dark',
        accent: '#0088FF',
        chromeBg: '#1e1e1e',
        chromeBar: '#2a2a2c',
        xterm: {
            background: '#283033', foreground: '#D9E0E3', cursor: '#D9E0E3', selectionBackground: '#264f78',
            black: '#000000', red: '#C91B00', green: '#00C200', yellow: '#C7C400', blue: '#0225C7',
            magenta: '#CA30C7', cyan: '#00C5C7', white: '#C7C7C7',
            brightBlack: '#686868', brightRed: '#FF6E67', brightGreen: '#5FF967', brightYellow: '#FEFB67',
            brightBlue: '#6871FF', brightMagenta: '#FF77FF', brightCyan: '#5FFDFF', brightWhite: '#FFFFFF',
        },
    },
    light: {
        label: '亮白 Pro',
        mode: 'light',
        accent: '#0088FF',
        chromeBg: '#f5f5f7',
        chromeBar: '#e8e8ed',
        xterm: {
            background: '#FFFFFF', foreground: '#000000', cursor: '#000000', selectionBackground: '#bcd6ee',
            black: '#000000', red: '#C91B00', green: '#00C200', yellow: '#C7C400', blue: '#0225C7',
            magenta: '#CA30C7', cyan: '#00C5C7', white: '#C7C7C7',
            brightBlack: '#686868', brightRed: '#FF6E67', brightGreen: '#5FF967', brightYellow: '#FEFB67',
            brightBlue: '#6871FF', brightMagenta: '#FF77FF', brightCyan: '#5FFDFF', brightWhite: '#FFFFFF',
        },
    },
    midnight: {
        label: '午夜蓝',
        mode: 'dark',
        accent: '#0A84FF',
        chromeBg: '#0d1117',
        chromeBar: '#161b22',
        xterm: {
            background: '#0d1117', foreground: '#c9d1d9', cursor: '#58a6ff', selectionBackground: '#1f6feb',
            black: '#484f58', red: '#ff7b72', green: '#3fb950', yellow: '#d29922', blue: '#58a6ff',
            magenta: '#bc8cff', cyan: '#39c5cf', white: '#b1bac4',
            brightBlack: '#6e7681', brightRed: '#ffa198', brightGreen: '#56d364', brightYellow: '#e3b341',
            brightBlue: '#79c0ff', brightMagenta: '#d2a8ff', brightCyan: '#56d4dd', brightWhite: '#f0f6fc',
        },
    },
    graphite: {
        label: '石墨灰',
        mode: 'dark',
        accent: '#98989D',
        chromeBg: '#1c1c1e',
        chromeBar: '#2c2c2e',
        xterm: {
            background: '#1c1c1e', foreground: '#d0d0d5', cursor: '#98989d', selectionBackground: '#48484a',
            black: '#3a3a3c', red: '#ff453a', green: '#32d74b', yellow: '#ffd60a', blue: '#0a84ff',
            magenta: '#bf5af2', cyan: '#64d2ff', white: '#d0d0d5',
            brightBlack: '#636366', brightRed: '#ff6961', brightGreen: '#30d158', brightYellow: '#ffe14a',
            brightBlue: '#409cff', brightMagenta: '#da8fff', brightCyan: '#8be2ff', brightWhite: '#f2f2f7',
        },
    },
    solar: {
        label: '暖橙',
        mode: 'dark',
        accent: '#FF9F0A',
        chromeBg: '#241a12',
        chromeBar: '#33271a',
        xterm: {
            background: '#241a12', foreground: '#f0e2c8', cursor: '#ff9f0a', selectionBackground: '#7a4d12',
            black: '#4a3a26', red: '#ff453a', green: '#a9b665', yellow: '#d8a657', blue: '#7daea3',
            magenta: '#d3869b', cyan: '#89b482', white: '#e0cfa8',
            brightBlack: '#6a5a40', brightRed: '#ff6961', brightGreen: '#c0ca8e', brightYellow: '#e8c77a',
            brightBlue: '#9ec4bb', brightMagenta: '#e4a6ba', brightCyan: '#a8d0a5', brightWhite: '#fbf1c7',
        },
    },
    forest: {
        label: '森林绿',
        mode: 'dark',
        accent: '#30D158',
        chromeBg: '#101812',
        chromeBar: '#1a241c',
        xterm: {
            background: '#101812', foreground: '#c9dcc9', cursor: '#30d158', selectionBackground: '#1f3d2a',
            black: '#2c3a2e', red: '#ff5f56', green: '#30d158', yellow: '#ffd60a', blue: '#40a8ff',
            magenta: '#bf5af2', cyan: '#5ad8c8', white: '#c9dcc9',
            brightBlack: '#4c5a4e', brightRed: '#ff8a80', brightGreen: '#63e07f', brightYellow: '#ffe14a',
            brightBlue: '#6dbaff', brightMagenta: '#da8fff', brightCyan: '#8bf0e0', brightWhite: '#f0faf0',
        },
    },
};

export const THEME_LIST: ThemeName[] = ['dark', 'light', 'midnight', 'graphite', 'solar', 'forest'];
