# lan-proxy-gateway 缺陷分析 & 重设计方向

> 日期：2026-06-13　范围：全项目体检 + 终端优先重设计 + AI-API 中转新能力
> 目标四原则：**稳定 · 速度快 · 低门槛 · 直观**

本文是「先分析、后逐个设计」的第一份产物。只列**问题**和**方向**，具体实现拆成后续子项目（每个走 设计→计划→实现）。

---

## 0. 体检快照（量化现状）

| 维度 | 现状 | 含义 |
|---|---|---|
| Web UI 体积 | `embed/webui` **2.2M / 114 文件** + `internal/webui` **~1138 行 Go** | 嵌进二进制，拖大体积、拖慢编译 |
| Web UI 触点 | **20 个 Go 文件**引用 webui/WebUIToken/web_ui | 移除是一次面较广的手术 |
| 终端 console | `internal/console` **~3700 行**（console.go 单文件 **2166 行**）+ cmd 层 console | 单文件过大，职责糅杂，难维护 |
| AI 能力 | 仅「域名分流」（residential-chain 预设硬编码域名表） | **没有** API 中转、没有 key 注入、没有内置免费额度 |
| 配置入口 | 三处可改配置：CLI 菜单 / Web UI / 直接编 yaml | 多入口 = 多份同步逻辑 = 多 bug 面 |

---

## 1. 🔴 稳定性缺陷（Stability）

### 1.1 Web UI 默认监听 `0.0.0.0:19091`，token 是唯一防线
`schema.go:194-200` 的注释自己承认：没有 token，**任何同网段设备都能改本机配置**，更糟的是通过 `SetScript` 指向恶意 `.js` 实现 **RCE**（goja 执行任意脚本）。一个「让小白省心」的旁路由网关，却默认对整个局域网敞开一个能远程改配置+跑脚本的控制面——这是当前**最大的安全/稳定隐患**。移除 Web UI 直接消灭这一整类风险面。

### 1.2 增强脚本是无沙箱的任意代码执行
`engine/render.go:89-94` → `script.Apply` 用 goja 跑用户/预设 JS。脚本路径来自配置，配置可被 Web UI 远程改。即使移除 Web UI，`ScriptPath` 仍是「配置即代码执行」，渲染失败会 `return nil, err` **直接阻断整个 render**（start/reload 全挂）。缺乏：超时、panic 恢复、脚本校验。

### 1.3 渲染管线「全有或全无」，单点失败拖垮启动
`render.go` 里 materialize 源、跑脚本任一失败都整体返回 error。订阅源临时挂掉、脚本小错 → 用户直接起不来，没有「降级到上次可用配置」的兜底。对「稳定」诉求是硬伤。

### 1.4 AI 域名表硬编码在预设模板里，过期即失效
`residential-chain.tmpl.js:156-229` 把 Claude/OpenAI/Cursor 等域名写死在 JS 模板里。厂商一换域名（如新增 `*.anthropic.com` 子域、`sora.com` 等）就漏路由，用户无法在不改源码的情况下更新。应数据化、可热更新。

### 1.5 多配置入口的同步竞态
CLI 菜单 `banner()` 每次重载 yaml、Web UI 走 RWMutex、用户还能手编 yaml。三方写同一文件，`saveAndMaybeReload` 失败时「配置已落盘但 reload 失败」的中间态会让 UI 显示与实际运行不一致。入口收敛到「终端单一真相源」能根除。

---

## 2. 🟡 速度 / 体积缺陷（Performance）

### 2.1 2.2M 嵌入式 Web 资产白白进二进制
114 个 Nuxt 产物（`_nuxt/*.js`）通过 `go:embed` 进二进制。删掉 Web UI 后**二进制立减 ~2M+**，编译更快，`make build-all` 三平台产物全部受益。

### 2.2 仪表盘是静态快照，靠手动 Enter/R 刷新
`console.go` 的 dashboard 不是实时的，流量/节点状态要按键才更新（探索报告确认）。「直观」要求实时，应改成定时拉 mihomo `/traffic` 流 + 增量重绘。

### 2.3 每次 render 都重新部署 Web UI 到 workdir
`render.go:38 deployWebUI(workDir)` 在**每次** render（含 reload）都把 UI 释放到磁盘。移除后这条 I/O 也省掉。

---

## 3. 🟠 低门槛缺陷（Low Barrier）

### 3.1 「内置免费 AI」目前不存在 —— 最大的低门槛机会
用户要的是：局域网任何人**不用自己的 key**，把 AI 客户端指向网关就能用。现在完全没有。这是本轮**价值最高**的新能力（详见 §6）。

### 3.2 AI 路由要求用户先有「住宅 IP 代理」才能用
现有 AI 能力绑死在 `ChainResidentialConfig`（住宅 IP 链式代理）。普通用户没有住宅 IP 代理，等于这条「卖点」对小白不可用。AI-API 中转端点不依赖住宅 IP，开箱即用，正好补这个门槛。

### 3.3 端口/TUN/DNS 概念对小白过重
`runtime.ports`（mixed/redir/api/web_ui）、TUN vs forward、`strict-route`、`dialer-proxy`……菜单里大量裸露 mihomo 术语。低门槛要求「场景化向导」而非「参数面板」。

### 3.4 install 向导与日常 console 割裂
`cmd/install.go`（245 行）一套交互，`internal/console` 又一套，新手走完 install 进 console 像换了个程序。应统一交互范式。

---

## 4. 🔵 直观性缺陷（Intuitive）

### 4.1 终端交互是「线性 REPL + 多级数字菜单」，不是现代 TUI
探索报告：M 进菜单 → 6 个子菜单 → 数字选择。这是 90 年代风格。对标优秀开源 CLI（k9s、lazygit、gh dash、btop、charm 自家 soft-serve），应是**单屏多面板 + 键盘导航 + 实时刷新**。用户明确说「专注终端交互」，这是核心战场。

### 4.2 console.go 单文件 2166 行，职责糅杂
dashboard 绘制、菜单分发、节点切换、源测试、设备标签全堆一个文件。难读难改，AI/人都容易在里面引入回归。重构时应按「屏（screen）/组件」拆分。

### 4.3 状态可见性差
AI 起飞/落地节点、源健康、设备列表分散在不同菜单深处，没有「一眼看全」的主屏。直观 = 重要状态常驻可见。

---

## 5. Web UI 移除清单（已确认彻底移除）

删除：`internal/webui/`、`embed/webui/`、`data/ui/`、`cmd/webui.go`、`cmd/webui_daemon*.go`、`internal/app/webui_adapter*.go`。
改动：`cmd/start.go`（去守护进程+横幅）、`cmd/restart.go`/`cmd/stop.go`、`internal/engine/render.go`（去 `deployWebUI`）、`internal/engine/engine.go`、`internal/config/schema.go`（去 `WebUIToken`、`RuntimePorts.WebUI`）、`internal/config/load.go`（去 token 生成/迁移）、相关 `*_test.go`。
收益：二进制 −2.2M、消灭 LAN 远程改配置+RCE 风险面、删除一整套状态同步适配层。
注意：保留 mihomo 自带的 `/ui`（external-ui）能力评估——那是 mihomo 的控制台不是本项目的 Web UI，按需决定是否一并下线（建议保留，属于「Mihomo 完整控制台」高级入口）。

---

## 6. 新能力：内置 AI 配网助手 agent（本轮第一个落地子项目）

> 形态已澄清并确认：**不是 API 中转站**，而是给项目注入一个**终端内的 AI 配网 agent**——用自然语言帮用户快速把 `lan-proxy-gateway` 配好、真正开箱即用，同时完整保留传统菜单配置。详见专项 spec：`2026-06-13-ai-config-agent-and-terminal-polish-design.md`。

**核心要点**：
- Agent 的**大模型后端可配置**，支持两种格式 + 内置免费零配置默认：
  - OpenAI 格式（轻量 Chat Completions 客户端）：内置免费 `openrouter/free`（默认）+ 用户第三方 baseUrl/key
  - Anthropic 格式（官方 `anthropic-sdk-go`，覆盖 base_url/key）：用户第三方 Claude 兼容 baseUrl/key
- **动作机制用 JSON 动作 DSL**，不依赖原生 function-calling（免费模型未必支持 tool-use），解析后调用现有 `App` facade。
- **安全**：读操作 inline 自动跑；写配置先列计划 + diff，用户确认再落盘 + reload。
- **入口**：主提示符输命令走命令、输人话走 agent；菜单保留。
- key 不入日志/快照，`gateway.yaml` 0600。

---

## 7. 子项目拆解 & 建议顺序

| # | 子项目 | 价值 | 依赖 | 顺序 |
|---|---|---|---|---|
| A | **内置 AI 配网助手 agent + 内置免费后端** | 最高（低门槛卖点） | 无 | **先做** |
| A′ | **终端 UI 精修**：网速 sparkline（图1红柱）+ 稳定性健康条（图2绿柱，近60次/每分钟测速/PAST→NOW） | 高（直观） | 无 | 与 A 同 spec |
| B | **彻底移除 Web UI** | 高（瘦身+安全） | 无 | 次做（清场） |
| C | **终端 TUI 重构**（实时主屏+面板化+console.go 拆分） | 高（直观核心） | B 之后更干净 | 第三 |
| D | AI 域名表数据化 + 可热更新 | 中（稳定） | — | 随 C |
| E | 渲染管线降级兜底 + 脚本沙箱（超时/recover） | 中（稳定） | — | 收尾 |

> 按已确认决策：**先做 A（AI 配网 agent）+ A′（终端两个图表）**，合并到一份 spec。本文交付后转入该 spec。
