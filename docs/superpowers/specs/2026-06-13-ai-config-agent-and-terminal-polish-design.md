# 设计 · 内置 AI 配网助手 agent + 终端 UI 精修

> 日期：2026-06-13　子项目 A + A′（合并）　状态：已确认，待用户复核 spec
> 关联：`2026-06-13-gateway-defects-and-redesign-analysis.md`
> 目标四原则：**稳定 · 速度快 · 低门槛 · 直观**

---

## 0. 一句话目标

给 `lan-proxy-gateway` 注入一个**终端内的 AI 配网助手**：用户用自然语言（「帮我把 Switch 接入代理」「AI 流量为什么不走代理」）就能把网关配好、开箱即用；大模型后端可配置（OpenAI / Anthropic 两种格式），并**内置一个免费后端零配置可用**。同时把终端仪表盘精修到位——实时网速柱状图 + 稳定性健康条。传统菜单配置完整保留，AI 只是「再上一档」的增强。

---

## 1. 范围

**做**：
- A. AI 配网助手 agent：双格式后端 + 内置免费默认 + JSON 动作 DSL + 确认式写操作 + 主提示符自然语言入口。
- A′. 终端 UI 两个图表：网速 sparkline（红柱）、稳定性健康条（绿柱，近 60 次 / 每分钟测速 / PAST→NOW）。

**不做（本 spec 之外，见分析报告 §7）**：移除 Web UI（子项目 B）、TUI 大重构（C）、AI 域名表数据化（D）、渲染降级兜底（E）。

---

## 2. 架构总览

```
                 ┌──────────────────────────────────────────────┐
   用户在主提示符  │  internal/console  (终端)                     │
   输入一句话 ───▶│   route: 命令? → 既有 dispatch                │
                 │          人话? → agentui.Session.Handle()      │
                 └───────────────┬──────────────────────────────┘
                                 │
                 ┌───────────────▼───────────────┐
                 │  internal/aiagent  (新包)       │
                 │   Session: 多轮对话循环          │
                 │   ├─ LLMClient (接口)            │   ←─ internal/config: ai.backends
                 │   │   ├─ openaiClient (HTTP+SSE) │       active 后端
                 │   │   └─ anthropicClient (sdk-go)│
                 │   ├─ 解析 JSON 动作 DSL          │
                 │   └─ 执行器 → internal/app.App   │   ←─ 复用现有 facade
                 └───────────────┬───────────────┘
                                 │ SetSource/ToggleTUN/SetMode/Status/Health…
                 ┌───────────────▼───────────────┐
                 │  internal/app.App (既有 facade) │
                 └────────────────────────────────┘
```

终端图表（A′）走另一条独立数据线，不经过 agent：

```
console/dashboard ──poll──▶ engine.Client.GetConnections()  → 上/下行速率 → sparkline 环形缓冲
                  ──1min──▶ engine.Client.GroupDelay()       → 健康/延迟 → health 环形缓冲(60)
```

---

## 3. 配置模型（新增 `ai` 段）

`internal/config/schema.go` 增加：

```go
type AIConfig struct {
    Enabled  bool        `yaml:"enabled"`
    Active   string      `yaml:"active"`            // 当前后端 id；空 = 第一个内置
    Backends []AIBackend `yaml:"backends"`
}

type AIBackend struct {
    ID      string `yaml:"id"`
    Format  string `yaml:"format"`                  // "openai" | "anthropic"
    BaseURL string `yaml:"base_url"`
    APIKey  string `yaml:"api_key,omitempty"`
    Model   string `yaml:"model"`
    Builtin bool   `yaml:"builtin,omitempty"`       // 内置免费，Normalize 时确保存在
}
```

`Config` 加字段 `AI AIConfig yaml:"ai"`。

**Default() / Normalize() 行为**：
- 若 `ai.backends` 缺少内置项，自动注入内置免费后端（幂等）：
  ```yaml
  ai:
    enabled: true
    active: free-openrouter
    backends:
      - id: free-openrouter
        format: openai
        base_url: http://load.hulupet.cn/proxy/openrouter-2
        api_key: sk-_yEsRLhQjzTQ9UPPgswHL_xbclZRazIJIqRqsWw1GkFBY-P8
        model: openrouter/free
        builtin: true
  ```
- 内置 key 以常量编进二进制（`internal/aiagent` 的 `const builtinFree...`），Normalize 注入时引用；用户删了也会补回，改了 active 指向自己后端则用用户的。
- **安全**：`gateway.yaml` 已是 0600；`Status()/Snapshot` 等对外结构**不得**包含 `APIKey`（脱敏成 `sk-***` 或省略）；日志打印 backend 只打 id/format/model，绝不打 key。

---

## 4. 子项目 A：AI 配网助手 agent

### 4.1 新包 `internal/aiagent`

| 文件 | 职责 |
|---|---|
| `client.go` | `LLMClient` 接口 + 工厂 `New(backend AIBackend) (LLMClient, error)` |
| `client_openai.go` | OpenAI Chat Completions 客户端：原生 `net/http` + SSE 流式解析。覆盖 `base_url`/`api_key`/`model`。 |
| `client_anthropic.go` | 官方 `anthropic-sdk-go`，`option.WithBaseURL`+`option.WithAPIKey`，model 取 backend.Model（默认 `claude-opus-4-8`）。 |
| `session.go` | 多轮对话循环：拼 system prompt、发消息、收回复、解析动作、执行、把结果回灌、直到 agent 给最终答复或无动作。 |
| `actions.go` | JSON 动作 DSL 定义 + 解析 + 执行器（调用 `app.App`）。 |
| `prompt.go` | system prompt 模板（含动作 schema、当前网关状态快照注入）。 |

`LLMClient` 接口（统一抽象，屏蔽两种格式）：
```go
type LLMClient interface {
    // Chat 发送多轮消息，流式回调每个文本增量；返回完整文本。
    Chat(ctx context.Context, msgs []Message, onDelta func(string)) (string, error)
}
type Message struct{ Role, Content string } // role: system|user|assistant
```
> 注：**不**走两家的原生 tool/function-calling——动作靠下面的 JSON DSL，从回复文本里解析，因此 `LLMClient` 只需最朴素的 chat+stream，免费模型也能驱动。后续若要给强后端加原生 tool-calling，再在接口上扩展，不影响默认路径。

### 4.2 JSON 动作 DSL

Agent 在回复里用一个 fenced code block 输出动作（约定 ```` ```gateway-action ````），其余是给用户看的自然语言。一轮最多一个动作；执行结果回灌后继续。

动作集（**一期**，全部映射到既有 `app.App` 方法）：

| action | 字段 | 映射 | 读/写 |
|---|---|---|---|
| `get_status` | — | `App.Status()` | 读·自动 |
| `get_health` | — | `App.Health()` | 读·自动 |
| `set_source` | `type`,`url`/`path`/`server`… | `App.SetSource(SourceConfig)` | 写·确认 |
| `set_mode` | `mode`(rule/global/direct) | `App.SetMode` | 写·确认 |
| `toggle_tun` | `enabled` | `App.ToggleTUN`（按目标态幂等） | 写·确认 |
| `set_gateway_mode` | `mode`(tun/forward) | `App.SetGatewayMode` | 写·确认 |
| `toggle_adblock` | `enabled` | `App.ToggleAdblock`（按目标态） | 写·确认 |
| `add_rule` | `verdict`,`rule` | 改 `Traffic.Extras` 后 `App.Save`+reload | 写·确认 |
| `start` / `stop` / `restart` | — | `App.Start`/`Stop` | 写·确认 |
| `finish` | `summary` | 无（结束本轮，纯收尾） | — |

> 解析失败/未知 action：不执行，把错误回灌让 agent 改正（最多重试 N=3 轮，超出则提示用户手动菜单兜底）。
> `toggle_*` 现有签名是「翻转」；为让 agent 能表达「确保开/关」，执行器先读当前态，只在与目标态不一致时调用，保证幂等。

### 4.3 安全 / 确认式写操作

- **读操作**（get_status/get_health）：inline 自动执行，结果回灌，用户无感。
- **写操作**：执行器**不直接落盘**。先产出一个「计划卡」给用户看：
  ```
  AI 准备执行：设置订阅源
    source.type:  none → subscription
    source.url:   (空) → https://example.com/sub
  确认? [y/N]
  ```
  diff 由「当前 Config 相关字段 vs 动作目标值」生成。用户 `y` → 调 facade（内部已 Save+reload）；否则取消并把「用户拒绝」回灌给 agent。
- 网关是 sudo 级系统组件，**绝不**全自动改配置——吸取分析报告 §1.1 的教训。

### 4.4 终端入口（`internal/console`）

- 主提示符既有逻辑：先按既有命令表 dispatch；**无法匹配为命令**且非空白 → 视为自然语言，转 `aiagent.Session.Handle(line)`。
- 单独入口冗余保底：菜单加一项「AI 配网助手」进入连续对话模式（输入 `/exit` 回主界面）。
- `ai.enabled=false` 或后端不可用时：自然语言输入回退到「未识别命令」提示 + 指引开启 AI；不阻塞既有命令。
- 流式输出：`onDelta` 直接 `fmt.Print` 到终端，呈现打字机效果。

### 4.5 后端管理（最小可用）

- 菜单加「AI 后端」子页：列出 backends、切换 active、增删用户后端（填 format/base_url/key/model）、一键测试连通（发一句 "ping" 看是否有回）。
- 一期不做跨格式互转、不做多后端并发；`active` 单选。

---

## 5. 子项目 A′：终端 UI 精修

数据线独立于 agent，挂在 `internal/console/dashboard.go` 既有快照刷新里。

### 5.1 网速 sparkline（对标图1 红柱）

- 数据：复用既有 `engine.Client.GetConnections()` 的 `DownloadTotal/UploadTotal` 差值算每刷新周期的上/下行速率（dashboard 已有此逻辑，见 `dashboard.go:139-200`）。
- 结构：新增环形缓冲 `type sparkline struct { buf []float64; n int }`（容量约 40，对应图1宽度）。每次刷新 push 一个速率值。
- 渲染：用 Unicode 区块字符 `▁▂▃▄▅▆▇█` 把每个值按「窗口内最大值」归一化映射成柱高；红色（lipgloss / ANSI）。上下行各一条，或合并下行为主。
- 放在主屏「↓ x/s ↑ y/s」那一行下方常驻。

### 5.2 稳定性健康条（对标图2 绿柱）

- 标题行：`近 60 次记录` ……右侧 `NNs 后刷新`；底部左 `PAST` 右 `NOW`。
- 数据：每分钟一次「测速/健康探测」——用 `engine.Client.GroupDelay(group, testURL, timeoutMs)` 对主出口组测延迟（已存在）；记录成功/延迟值。
- 结构：固定长度 60 的环形缓冲（60 格 = 60 分钟）。每分钟 append 一格。
- 渲染：60 根等宽竖条，全绿表示健康；失败/超时格子变暗或变红。`NNs 后刷新` = 距下次整分钟测速的倒计时。
- 测速 cadence 用独立 ticker（1 分钟），与 dashboard 的秒级刷新解耦，避免每秒打扰出口。

### 5.3 刷新模型

- 现状 dashboard 是「按键刷新」的静态快照。一期**最小改动**：sparkline/健康条数据随既有快照刷新累积；倒计时和柱状重绘在每次 `drawDashboardOnce` 时按当前缓冲渲染。
- 是否引入秒级自动重绘（真正的实时滚动）属子项目 C 的 TUI 重构范畴；本 spec 先把**数据采集 + 渲染**就位，确保切到自动刷新时零改动即生效。

---

## 6. 依赖与体积

- 新增 Go 依赖：`github.com/anthropics/anthropic-sdk-go`（Anthropic 格式后端）。OpenAI 格式走原生 `net/http`，**不**引入第三方 OpenAI 库（chat completions + SSE 足够简单，自己写约 150 行，省一个依赖、可控）。
- sparkline / 健康条用 Unicode + 既有 lipgloss，无新依赖。

---

## 7. 验证

- **单测**：
  - `aiagent/actions_test.go`：DSL 解析（合法/非法/未知 action）、`toggle_*` 幂等执行器（mock App）。
  - `aiagent/client_openai_test.go`：SSE 流式解析（httptest 喂分块响应）。
  - `config` 测试：Normalize 注入内置后端幂等；Snapshot 不泄漏 APIKey。
  - `console/sparkline_test.go`、`health_test.go`：归一化映射、环形缓冲、60 格倒计时。
- **手验**：`make build` → 跑起来 → 主提示符输「帮我把订阅源设成 <url>」→ 看计划卡 → 确认 → `status` 验证生效；主屏看到红柱滚动 + 绿健康条 + 倒计时。
- TDD：按 superpowers test-driven-development，先写测试再实现。

---

## 8. 风险

| 风险 | 缓解 |
|---|---|
| 免费 `openrouter/free` 不稳/限流/不支持长上下文 | JSON DSL 不依赖 tool-use；超时/出错清晰提示并可一键切到用户后端；agent 失败不影响传统菜单。 |
| 内置 key 泄漏/被滥用 | key 只在二进制内 + 0600 配置；不入日志/快照；这是「提供免费算力」性质，可接受被薅，必要时换 key 即可。 |
| Agent 改错网关配置断网 | 全部写操作确认式 + diff；facade 内 reload 失败有 warning 且配置可回退（既有行为）。 |
| 自然语言误判为命令/反之 | 先尝命令匹配，失败才转 agent；命令永远优先，零回归。 |

---

## 9. 落地顺序（交给 writing-plans 细化）

1. config：`AIConfig` 结构 + Default/Normalize 注入内置后端 + Snapshot 脱敏（含测试）。
2. `aiagent`：`LLMClient` 接口 + openai 客户端（SSE）+ anthropic 客户端 + 动作 DSL + 执行器（含测试，先 TDD）。
3. `aiagent.Session` 对话循环 + system prompt（注入状态快照）。
4. console：主提示符自然语言路由 + 「AI 配网助手」对话模式 + 「AI 后端」管理子页。
5. A′：sparkline + 健康条 数据结构/采集/渲染（含测试）。
6. 手验 + `go test ./...` 全绿。
