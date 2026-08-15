# ssh-terminal

本地优先的现代 SSH/Telnet 终端 + SFTP 文件管理 + AI 辅助运维工具。

基于 **Wails (Go) + React + TypeScript + xterm.js** 构建，对标 Termius / XShell，主打**数据不出网、安全审计、开箱即用的完整运维工作流**。

> 特性概览：多协议终端 · SFTP 双栏文件管理 · SSH 隧道 · 跳板机 · 会话分组 · 命令片段 · 日志检索与回放 · 会话审计 · 分屏广播 · AI 辅助（DeepSeek / Ollama）· 深度玻璃化 UI

---

## ✨ 功能清单

### 终端与连接

| 功能 | 说明 |
|---|---|
| 多协议 | SSH / Telnet（自研 IAC 协商） |
| 认证方式 | 密码、私钥（含口令）、OTP 双因素、keyboard-interactive |
| 跳板机 | 链式 ProxyJump，逐级认证 |
| 编码支持 | **GBK / UTF-8 自动探测**（Windows 服务器中文不乱码），可强制指定 |
| 主机密钥校验 | known_hosts 三种模式：自动接受 / 首次确认（指纹展示）/ 关闭 |
| 断线重连 | 意外断开自动重连（退避 2s/4s/6s），标签页常驻不丢会话 |
| 会话分组 | 分组/未分组管理，保存的会话按分组展示与移动 |

### 终端体验

- 多会话标签页、**分屏模式**（多会话同屏 + 广播命令）
- 终端内查找（Ctrl+F）、右键菜单（复制/粘贴/全选/发送片段）
- 6 套主题（暗黑/亮白/午夜/石墨/日光/森林）、字体与字号可调
- **日志关键字着色**：ERROR/异常/WARN 等自动高亮（纯文本日志）
- 网络状态栏：实时速率、时长、保活 RTT

### 文件与传输（SFTP）

- 双栏文件管理：上传/下载/删除/重命名/新建目录/权限修改
- 目录传输、大小校验、冲突策略（覆盖/跳过/自动改名）
- **远程编辑**：下载 → 系统编辑器打开 → 关闭后自动回传
- 传输队列与进度展示

### 隧道

- 本地转发 `-L` / 动态转发 `-D` (SOCKS5) / 远程转发 `-R`
- 隧道管理面板，一键启停

### 日志、审计与片段

- **历史日志**：会话输出实时落盘，跨会话**关键字检索**，命中文件列表 + 行数
- **日志回放**：逐行播放 / 暂停 / 跳至末尾
- **会话审计**：自动记录每次连接（时间/主机/用户/协议/时长/流量），点击**回放**该会话完整输出——操作留痕，适合内网/政企运维场景
- **命令片段**：快捷命令管理，终端右键菜单一键发送

### AI 辅助（M5）

- **AiProvider 抽象**：DeepSeek（云端）/ Ollama（本地）可插拔
- 上下文注入：自动携带当前会话最近终端输出（报错现场即问即答）
- **强制脱敏**：发送前过滤密码/令牌/私钥/数据库连接串/用户敏感词
- **成本控制**：月度 token 限额（可调）+ 模型分档 + 流式中断
- 流式渲染对话面板

### 其他

- 自定义背景图片、ESC 关闭弹窗、Termius 风格深度玻璃化 UI
- 密码存系统凭据库（Windows Credential Manager），会话配置存 bbolt
- 历史文件滚动清理（数量/容量上限）

---

## 🧱 技术栈

| 层 | 选型 |
|---|---|
| 后端 | Go + Wails v2（绑定层在根 `package main`，业务在 `internal/`） |
| 前端 | React 19 + TypeScript + Vite + xterm.js（fit/search addon） |
| 存储 | bbolt（会话/片段/AI 用量/审计）+ Windows 凭据库（密码）+ 历史日志文件 |
| 协议 | `golang.org/x/crypto/ssh`、`pkg/sftp`、自研 telnetcore |
| AI | 纯标准库 `net/http` + SSE 解析（无外部依赖） |

## 🏗️ 架构

```
┌────────────────────────────────────────────────┐
│  React + xterm.js  (终端 / SFTP / AI / 审计)    │
├────────────────────────────────────────────────┤
│  Wails 绑定层  app.go (Connect/Send/隧道/AI…)   │
├────────────────────────────────────────────────┤
│  internal/                                       │
│   ├── sshcore      SSH 会话 + known_hosts 校验   │
│   ├── telnetcore   Telnet 会话 (IAC 协商)        │
│   ├── enc          流式编码转换 (UTF-8/GBK)      │
│   ├── transfer     SFTP 传输引擎                 │
│   ├── ai           AI Provider/脱敏/成本控制     │
│   ├── store        bbolt 持久化                  │
│   └── localfs      本地文件操作                  │
└────────────────────────────────────────────────┘
```

## 🚀 构建与运行

```bash
# 前置: Go 1.25+、Node.js、Wails CLI v2

# 开发模式（热重载）
wails dev

# 构建产物 (Windows)
wails build   # → build/bin/ssh-terminal.exe
```

> ⚠️ 必须用 `wails build` 产物运行（带 wails 构建标签）；`go build ./...` 生成的根目录 exe 无法启动。

## 📁 目录结构

```
├── app.go               # Wails 绑定层 (终端/隧道/会话管理/AI/审计)
├── history.go           # 历史日志落盘 + 滚动清理 + 跨会话检索
├── tunnel.go            # SSH 端口转发
├── internal/
│   ├── ai/              # AI Provider/脱敏/用量
│   ├── enc/             # GBK/UTF-8 流式转换
│   ├── localfs/         # 本地文件操作
│   ├── model/           # DTO 模型
│   ├── sshcore/         # SSH 会话 + knownhosts
│   ├── store/           # bbolt 持久化
│   ├── telnetcore/      # Telnet 会话
│   └── transfer/        # SFTP 传输引擎
└── frontend/
    └── src/             # React 前端 (TerminalView/FilePanel/AiPanel/...)
```

## ✅ 测试

```bash
go test ./...   # 6 个业务包 40+ 用例 (store/telnetcore/sshcore/localfs/transfer/ai/enc)
```

## 📌 已知限制 / 路线图

- [ ] 云桌面 VNC（当前仅 RDP mstsc 外接）
- [ ] HTTP/SOCKS 代理连接
- [ ] 云同步 Vault（Termius 模式，V2 计划）
- [ ] SSH ID / Passkeys
- [ ] 端点监控告警、定时任务、RBAC
- [ ] 移动端

## 📄 文档

- [功能与 UI 差距补充分析（对标 Termius / termius-plus）](docs/02-功能与UI差距补充分析.md)
- [项目状态记录](docs/项目状态.md)

## 📜 协议

本项目为个人项目，功能对标商业产品（Termius/XShell）与开源项目（termius-plus，Mulan PSL-2.0），代码为自研实现。
