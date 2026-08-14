import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'

// 全局错误覆盖层: 页面异常时直接把错误显示在窗口上, 避免黑屏无信息
function showFatalError(title: string, detail: string) {
    const el = document.getElementById('root')
    if (!el) return
    el.innerHTML = `<div style="padding:24px;font-family:Consolas,monospace;background:#1e1e1e;color:#ddd;min-height:100vh;box-sizing:border-box">
        <h2 style="color:#ff6b6b;margin:0 0 12px;font-size:18px">⚠️ ${title}</h2>
        <pre style="white-space:pre-wrap;word-break:break-all;color:#ff9d9d;margin:0;font-size:13px;line-height:1.5">${detail}</pre>
    </div>`
}

window.addEventListener('error', (e) => {
    showFatalError('JS 运行时错误', `${e.message}\n${e.filename}:${e.lineno}:${e.colno}`)
})
window.addEventListener('unhandledrejection', (e) => {
    showFatalError(
        '未处理的 Promise 拒绝',
        e.reason instanceof Error ? `${e.reason.message}\n${e.reason.stack ?? ''}` : String(e.reason),
    )
})

const container = document.getElementById('root')

try {
    const root = createRoot(container!)
    root.render(
        <React.StrictMode>
            <App/>
        </React.StrictMode>
    )
} catch (err) {
    showFatalError('应用启动失败', err instanceof Error ? `${err.message}\n${err.stack ?? ''}` : String(err))
}
