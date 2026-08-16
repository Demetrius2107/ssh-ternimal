# FreeRDP 内嵌评估（M4 V2 剩余项）

> 更新：2026-08-16
> 级别：L1 专项评估（涉及 cgo 编译链、跨平台打包、协议复杂度）
> 结论：**当前阶段不建议实施 RDP 内嵌**——三方案在本机环境下均不可行或成本过高；保留外接 mstsc + 内嵌 VNC 即可覆盖需求。

---

## 1. 背景与目标

选型文档 M4 规划"云桌面 RDP/VNC 集成，先外接、再内嵌 FreeRDP"。当前状态：

| 项 | 状态 |
|---|---|
| RDP 外接（mstsc） | ✅ 已实现（生成临时 .rdp 调系统 mstsc） |
| VNC 内嵌（自研 RFB 客户端） | ✅ 已实现（`internal/vnccore` + canvas 渲染） |
| **RDP 内嵌** | ⏳ 本次评估对象 |

目标：把 RDP 会话渲染进应用窗口（对标 MobaXterm 内嵌），支持剪贴板/文件重定向。

## 2. 三条技术路线对比

### 路线 A：cgo 编译 FreeRDP（官方库绑定）

| 维度 | 评估 |
|---|---|
| 原理 | Go 通过 cgo 链接 FreeRDP C 库（`github.com/freerdp/freerdp` 提供 Go 绑定） |
| 能力 | 完整 RDP：CredSSP/NLA、剪贴板、文件重定向、H.264、多显示器 |
| 本机可行性 | ❌ **不可行**：本机无 gcc/cc 工具链（`which gcc` 为空）、未安装 FreeRDP 开发库、网络受限（HTTPS 被中间人拦截）难以通过 winget/msys2 可靠安装 |
| 跨平台代价 | 每平台需对应 C 工具链 + FreeRDP 库：Windows (mingw)、macOS (brew)、Linux (apt)——打包复杂翻倍 |
| 维护风险 | cgo 与纯 Go 混合：构建标签、链接器、CI 环境差异；FreeRDP 升级需同步编译链 |

### 路线 B：纯 Go RDP 客户端库

| 维度 | 评估 |
|---|---|
| 候选库 | `github.com/icodeho/go-rdp`（早期实验）等 |
| 成熟度 | ❌ **无成熟实现**：RDP 协议（RFC 主协议 + 大量 MS-RDP 扩展）极其复杂，纯 Go 项目均停留在早期阶段，不支持 NLA/CredSSP 等现代认证 |
| 能力 | 仅能连配置极简的测试服务器，无法连真实 Windows（NLA 默认开启即失败） |
| 结论 | **不可用**，投入重写协议层的成本远超收益 |

### 路线 C：Guacamole 架构（WebSocket 桥接）

| 维度 | 评估 |
|---|---|
| 原理 | 服务端（Apache Guacamole 的 guacd）解析 RDP → WebSocket 协议 → 前端 canvas 渲染（noVNC 同族） |
| 能力 | RDP/VNC 统一走 WebSocket，前端渲染成熟 |
| 本机可行性 | 🟡 部分可行：Go 侧可用 `github.com/gorilla/websocket`（本机可下载 v1.5.3）转发，但 **guacd 是 Java/C 服务端**，需额外部署 |
| 代价 | 引入常驻服务端进程（与"本地优先、无外部依赖"定位冲突）；二进制体积与启动复杂度上升 |
| 结论 | 适合"云桌面托管平台"场景（Termius/公司内网门户），**不适合个人桌面工具** |

## 3. 本机环境实测结论

| 检查项 | 结果 |
|---|---|
| gcc/cc 工具链 | ❌ 无（`which gcc` 无输出） |
| FreeRDP 安装 | ❌ 未安装（`C:\Program Files\FreeRDP` 不存在） |
| Go 侧 RDP 库 | 仅早期实验品，无成熟可用 |
| gorilla/websocket | ✅ 可下载（v1.5.3）——但仅支撑 Guacamole 前端侧，服务端仍需部署 |
| 网络 | HTTPS 直连被中间人拦截，工具链下载依赖 http 镜像/浏览器，不稳定 |

## 4. 风险评估

| 风险 | 等级 | 说明 |
|---|---|---|
| 编译链环境不可复现 | 🔴 高 | cgo + FreeRDP 在无稳定网络的本机装不起来，CI/换机即断 |
| 打包复杂度翻倍 | 🔴 高 | 三平台交叉编译 FreeRDP，超出个人项目维护能力 |
| RDP 协议认证（NLA/CredSSP） | 🟡 中 | 纯 Go 路线绕不开，自研成本按"月"计 |
| 体积膨胀 | 🟡 中 | 内嵌 FreeRDP 或 Guacamole 均显著增加产物体积 |

## 5. 结论与推荐

**推荐：当前阶段不实施 RDP 内嵌，维持现有方案。**

| 决策 | 理由 |
|---|---|
| 保留 RDP 外接（mstsc） | 已实现、零成本、Windows 原生体验完整（剪贴板/磁盘重定向系统自带） |
| 保留 VNC 内嵌 | 覆盖 Linux 桌面场景（服务器多为 Linux，VNC 更常用） |
| RDP 内嵌标记"搁置" | 三方案均不可行/成本过高，**需求可由外接 mstsc 完全满足** |

**触发重新评估的条件**（未来满足其一再立项）：
1. 出现成熟的纯 Go RDP 客户端（支持 NLA/CredSSP）
2. 用户明确需要**窗口内嵌 Windows 桌面**且接受外接方案无法满足的交互（如需要与 SFTP/终端同窗操作 RDP）
3. 环境获得稳定工具链网络（可自动安装 mingw/FreeRDP）

## 6. 若未来落地（预留路径）

```
方案 A 落地路径（推荐优先，条件成熟时）:
1. winget 安装 mingw + FreeRDP（需稳定网络）
2. 独立 Go module（仿 server/ 模式）: rdp/ 目录, 纯 cgo 绑定, 不污染主构建
3. 前端复用 VncPanel 的 canvas 渲染框架（仅替换帧数据源）
4. 打包时按平台分别构建, CI 提供对应工具链镜像

方案 C 落地路径（面向"云桌面平台"转型时）:
1. 部署 guacd 服务端（Docker 编排）
2. 客户端新增"云桌面"连接类型, gorilla/websocket 桥接
3. 与云同步 server 模块可共用部署环境
```

> 关联：docs/04 3.8 预分析（FreeRDP 内嵌）→ 本文件为正式评估结论。
