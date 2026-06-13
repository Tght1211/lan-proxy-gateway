# AI 配网助手 agent + 终端 UI 精修 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 `lan-proxy-gateway` 注入终端内 AI 配网助手（双格式后端 + 内置免费默认 + JSON 动作 DSL + 确认式写操作），并精修终端仪表盘（网速 sparkline + 稳定性健康条）。

**Architecture:** 新增 `internal/aiagent` 包，定义 `LLMClient` 接口（openai/anthropic 两实现）+ `Controller` 接口（由 `*app.App` 满足，免循环依赖）+ JSON 动作 DSL 解析/执行 + 多轮 `Session`。`internal/console` 在主提示符把无法识别为命令的自然语言转给 agent，并新增「AI 后端」管理子页。终端图表在 `internal/console` 用环形缓冲采集既有 `engine.Client` 数据并用 Unicode 区块渲染。

**Tech Stack:** Go；`github.com/anthropics/anthropic-sdk-go`（Anthropic 格式后端）；OpenAI 格式走原生 `net/http`+SSE（不引第三方库）；既有 `lipgloss` 上色；既有 `app.App` facade / `engine.Client`。

参考 spec：`docs/superpowers/specs/2026-06-13-ai-config-agent-and-terminal-polish-design.md`

---

## 文件结构

**新建：**
- `internal/config/ai.go` — `AIConfig`/`AIBackend` 结构 + 内置免费后端常量 + `normalizeAI()`。
- `internal/aiagent/client.go` — `LLMClient` 接口、`Message`、工厂 `NewClient`。
- `internal/aiagent/client_openai.go` — OpenAI Chat Completions 客户端（HTTP+SSE）。
- `internal/aiagent/client_anthropic.go` — anthropic-sdk-go 适配。
- `internal/aiagent/action.go` — JSON 动作 DSL 结构 + `ParseAction`。
- `internal/aiagent/controller.go` — `Controller` 接口（`*app.App` 满足）。
- `internal/aiagent/executor.go` — 动作执行器（读 inline / 写确认式）。
- `internal/aiagent/prompt.go` — system prompt 模板。
- `internal/aiagent/session.go` — 多轮对话循环。
- `internal/console/sparkline.go` — 网速柱状图环形缓冲 + 渲染。
- `internal/console/health.go` — 稳定性健康条环形缓冲 + 渲染。
- 对应 `_test.go`。

**修改：**
- `internal/config/schema.go` — `Config` 加 `AI AIConfig`。
- `internal/config/load.go:191` `Normalize()` — 末尾调用 `normalizeAI(cfg)`。
- `internal/app/app.go` — 加 `AddRule` facade 方法；`Status` 加 `AI` 脱敏字段。
- `internal/console/console.go:467` `main()` — `default` 分支路由到 agent；菜单加「AI 配网助手」「AI 后端」入口。
- `internal/console/dashboard.go` — 接入 sparkline + health 渲染。
- `go.mod` — 加 anthropic-sdk-go。

---

## Task 1: config 增加 AIConfig + 内置免费后端注入

**Files:**
- Create: `internal/config/ai.go`
- Modify: `internal/config/schema.go`（`Config` 加字段）、`internal/config/load.go`（`Normalize` 末尾调用）
- Test: `internal/config/ai_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/config/ai_test.go
package config

import "testing"

func TestNormalizeInjectsBuiltinAIBackend(t *testing.T) {
	cfg := &Config{}
	Normalize(cfg)
	if !cfg.AI.Enabled {
		t.Fatal("AI 应默认启用")
	}
	if cfg.AI.Active != BuiltinAIBackendID {
		t.Fatalf("active 应为内置后端，得到 %q", cfg.AI.Active)
	}
	var found *AIBackend
	for i := range cfg.AI.Backends {
		if cfg.AI.Backends[i].ID == BuiltinAIBackendID {
			found = &cfg.AI.Backends[i]
		}
	}
	if found == nil {
		t.Fatal("应注入内置免费后端")
	}
	if found.Format != "openai" || found.Model != "openrouter/free" {
		t.Fatalf("内置后端字段不对: %+v", *found)
	}
	if found.APIKey == "" || found.BaseURL == "" {
		t.Fatal("内置后端应带 baseURL 和 key")
	}
}

func TestNormalizeBuiltinAIIsIdempotent(t *testing.T) {
	cfg := &Config{}
	Normalize(cfg)
	Normalize(cfg)
	n := 0
	for _, b := range cfg.AI.Backends {
		if b.ID == BuiltinAIBackendID {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("内置后端应只有 1 个，得到 %d", n)
	}
}

func TestNormalizeKeepsUserActive(t *testing.T) {
	cfg := &Config{AI: AIConfig{Active: "my-claude", Backends: []AIBackend{
		{ID: "my-claude", Format: "anthropic", BaseURL: "https://api.anthropic.com", Model: "claude-opus-4-8"},
	}}}
	Normalize(cfg)
	if cfg.AI.Active != "my-claude" {
		t.Fatalf("用户已设 active 不应被覆盖，得到 %q", cfg.AI.Active)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/config/ -run TestNormalize.*AI -v`
Expected: FAIL（编译错误：`AIConfig`/`BuiltinAIBackendID` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/config/ai.go
package config

// AIConfig 是内置 AI 配网助手 agent 的配置。
type AIConfig struct {
	Enabled  bool        `yaml:"enabled"`
	Active   string      `yaml:"active"`   // 当前后端 id；空 = 内置免费
	Backends []AIBackend `yaml:"backends"`
}

// AIBackend 是一个大模型后端。Format 决定走 openai 还是 anthropic 客户端。
type AIBackend struct {
	ID      string `yaml:"id"`
	Format  string `yaml:"format"` // "openai" | "anthropic"
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key,omitempty"`
	Model   string `yaml:"model"`
	Builtin bool   `yaml:"builtin,omitempty"`
}

// 内置免费后端：OpenAI 格式，开箱即用。key 随包提供，可被薅，必要时换值即可。
const (
	BuiltinAIBackendID      = "free-openrouter"
	builtinAIBackendBaseURL = "http://load.hulupet.cn/proxy/openrouter-2"
	builtinAIBackendAPIKey  = "sk-_yEsRLhQjzTQ9UPPgswHL_xbclZRazIJIqRqsWw1GkFBY-P8"
	builtinAIBackendModel   = "openrouter/free"
)

func builtinAIBackend() AIBackend {
	return AIBackend{
		ID:      BuiltinAIBackendID,
		Format:  "openai",
		BaseURL: builtinAIBackendBaseURL,
		APIKey:  builtinAIBackendAPIKey,
		Model:   builtinAIBackendModel,
		Builtin: true,
	}
}

// normalizeAI 幂等保证内置后端存在；首次填默认 active/enabled。
func normalizeAI(cfg *Config) {
	hasBuiltin := false
	for i := range cfg.AI.Backends {
		if cfg.AI.Backends[i].ID == BuiltinAIBackendID {
			cfg.AI.Backends[i] = builtinAIBackend() // 用内置值覆盖，防用户改坏 key
			hasBuiltin = true
		}
	}
	if !hasBuiltin {
		cfg.AI.Backends = append(cfg.AI.Backends, builtinAIBackend())
	}
	if cfg.AI.Active == "" {
		cfg.AI.Active = BuiltinAIBackendID
		cfg.AI.Enabled = true
	}
}

// ActiveBackend 返回当前激活的后端；找不到则回退内置。
func (c AIConfig) ActiveBackend() AIBackend {
	for _, b := range c.Backends {
		if b.ID == c.Active {
			return b
		}
	}
	return builtinAIBackend()
}
```

在 `internal/config/schema.go` 的 `Config` 结构（第 18-24 行）加字段：

```go
type Config struct {
	Version int           `yaml:"version"`
	Gateway GatewayConfig `yaml:"gateway"`
	Traffic TrafficConfig `yaml:"traffic"`
	Source  SourceConfig  `yaml:"source"`
	Runtime RuntimeConfig `yaml:"runtime"`
	AI      AIConfig      `yaml:"ai"`
}
```

在 `internal/config/load.go` 的 `Normalize()` 末尾（`}` 之前）加一行：

```go
	normalizeAI(cfg)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/config/ -run TestNormalize.*AI -v`
Expected: PASS（三个测试全过）

- [ ] **Step 5: 全包测试 + 提交**

```bash
go test ./internal/config/
git add internal/config/ai.go internal/config/ai_test.go internal/config/schema.go internal/config/load.go
git commit -m "feat(config): add AIConfig with builtin free openrouter backend"
```

---

## Task 2: app facade 增加 AddRule + Status 脱敏 AI 字段

**Files:**
- Modify: `internal/app/app.go`
- Test: `internal/app/ai_status_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/app/ai_status_test.go
package app

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestStatusRedactsAIKey(t *testing.T) {
	cfg := config.Default()
	config.Normalize(cfg)
	a := &App{Cfg: cfg, Plat: nil}
	// Status() 会调用 a.Plat / a.Gateway，这里只测 AI 字段，构造最小 App。
	st := a.aiStatus()
	if len(st) == 0 {
		t.Fatal("应至少有内置后端")
	}
	for _, b := range st {
		if b.HasKey && b.APIKeyMasked == "" {
			t.Fatal("有 key 的后端应给出掩码而非空")
		}
		if b.APIKeyMasked != "" && b.APIKeyMasked[:3] != "sk-" && b.APIKeyMasked != "***" {
			// 掩码必须不是明文 key
			if len(b.APIKeyMasked) > 6 {
				t.Fatalf("掩码疑似泄漏明文: %q", b.APIKeyMasked)
			}
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/app/ -run TestStatusRedactsAIKey -v`
Expected: FAIL（`aiStatus`/`AIBackendStatus` 未定义）

- [ ] **Step 3: 写实现**

在 `internal/app/app.go` 末尾追加：

```go
// AIBackendStatus 是给 UI 看的后端摘要，永不含明文 key。
type AIBackendStatus struct {
	ID           string
	Format       string
	Model        string
	Active       bool
	Builtin      bool
	HasKey       bool
	APIKeyMasked string // "***" 或空；绝不是明文
}

// aiStatus 把 AI 后端脱敏成 UI 摘要。
func (a *App) aiStatus() []AIBackendStatus {
	out := make([]AIBackendStatus, 0, len(a.Cfg.AI.Backends))
	for _, b := range a.Cfg.AI.Backends {
		masked := ""
		if b.APIKey != "" {
			masked = "***"
		}
		out = append(out, AIBackendStatus{
			ID:           b.ID,
			Format:       b.Format,
			Model:        b.Model,
			Active:       b.ID == a.Cfg.AI.Active,
			Builtin:      b.Builtin,
			HasKey:       b.APIKey != "",
			APIKeyMasked: masked,
		})
	}
	return out
}

// AddRule 把一条规则按裁决（direct/proxy/reject）加到 Traffic.Extras，存盘并热重载。
func (a *App) AddRule(ctx context.Context, verdict, rule string) error {
	switch verdict {
	case "direct":
		a.Cfg.Traffic.Extras.Direct = append(a.Cfg.Traffic.Extras.Direct, rule)
	case "proxy":
		a.Cfg.Traffic.Extras.Proxy = append(a.Cfg.Traffic.Extras.Proxy, rule)
	case "reject":
		a.Cfg.Traffic.Extras.Reject = append(a.Cfg.Traffic.Extras.Reject, rule)
	default:
		return fmt.Errorf("不支持的裁决: %s（应为 direct/proxy/reject）", verdict)
	}
	if err := a.Save(); err != nil {
		return err
	}
	if a.Engine != nil && a.Engine.Running() {
		return a.Engine.Reload(ctx, config.EffectiveRuntimeConfig(a.Cfg))
	}
	return nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/app/ -run TestStatusRedactsAIKey -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./internal/app/
git add internal/app/app.go internal/app/ai_status_test.go
git commit -m "feat(app): add AddRule facade and redacted AI backend status"
```

---

## Task 3: aiagent — Message / LLMClient 接口 / 工厂骨架

**Files:**
- Create: `internal/aiagent/client.go`
- Test: `internal/aiagent/client_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/aiagent/client_test.go
package aiagent

import (
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestNewClientUnknownFormat(t *testing.T) {
	_, err := NewClient(config.AIBackend{Format: "bogus"})
	if err == nil {
		t.Fatal("未知 format 应报错")
	}
}

func TestNewClientOpenAIFormat(t *testing.T) {
	c, err := NewClient(config.AIBackend{
		Format: "openai", BaseURL: "http://x", APIKey: "k", Model: "m",
	})
	if err != nil {
		t.Fatalf("openai 后端不应报错: %v", err)
	}
	if c == nil {
		t.Fatal("应返回 client")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/aiagent/ -run TestNewClient -v`
Expected: FAIL（包不存在 / `NewClient` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/client.go
// Package aiagent 是终端内的 AI 配网助手：多轮对话 + JSON 动作 DSL 驱动网关配置。
package aiagent

import (
	"context"
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Message 是一轮对话消息。Role: "system" | "user" | "assistant"。
type Message struct {
	Role    string
	Content string
}

// LLMClient 抽象两种后端格式。只需最朴素的多轮 chat + 流式文本增量，
// 不依赖任何一家的原生 tool/function-calling —— 动作靠回复文本里的 JSON DSL。
type LLMClient interface {
	// Chat 发送多轮消息；onDelta 每收到一段文本增量回调一次（可为 nil）；
	// 返回拼好的完整文本。
	Chat(ctx context.Context, msgs []Message, onDelta func(string)) (string, error)
}

// NewClient 按后端 format 构造对应客户端。
func NewClient(b config.AIBackend) (LLMClient, error) {
	switch b.Format {
	case "openai":
		return newOpenAIClient(b), nil
	case "anthropic":
		return newAnthropicClient(b), nil
	default:
		return nil, fmt.Errorf("不支持的后端 format: %q（应为 openai/anthropic）", b.Format)
	}
}
```

> 注：本步骤会因 `newOpenAIClient`/`newAnthropicClient` 未定义而无法编译——下一步立即补上 openai 客户端，anthropic 在 Task 5 补。为让本任务可独立通过，先在 client.go 末尾加一个临时占位的 anthropic 构造（Task 5 替换）：

```go
// 临时占位：Task 5 用真实 anthropic-sdk-go 实现替换。
func newAnthropicClient(b config.AIBackend) LLMClient { return newOpenAIClient(b) }
```

- [ ] **Step 4: 先实现 openai 客户端（见 Task 4），再运行确认通过**

（本任务与 Task 4 合并验证——Task 4 Step 4 一起跑 `go test ./internal/aiagent/ -run 'TestNewClient|TestOpenAI'`）

- [ ] **Step 5: 提交（与 Task 4 合并提交）**

---

## Task 4: aiagent — OpenAI 格式客户端（HTTP + SSE）

**Files:**
- Create: `internal/aiagent/client_openai.go`
- Test: `internal/aiagent/client_openai_test.go`

- [ ] **Step 1: 写失败测试（用 httptest 喂 SSE 分块）**

```go
// internal/aiagent/client_openai_test.go
package aiagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

func TestOpenAIChatStreamsDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("缺少 bearer 鉴权头")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// 模拟 OpenAI 流式：每个 chunk 一个 delta.content
		io := w.(interface{ Flush() })
		writeSSE(w, `{"choices":[{"delta":{"content":"你好"}}]}`)
		io.Flush()
		writeSSE(w, `{"choices":[{"delta":{"content":"，世界"}}]}`)
		io.Flush()
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := newOpenAIClient(config.AIBackend{
		Format: "openai", BaseURL: srv.URL, APIKey: "testkey", Model: "m",
	})
	var got strings.Builder
	full, err := c.Chat(context.Background(),
		[]Message{{Role: "user", Content: "hi"}},
		func(s string) { got.WriteString(s) })
	if err != nil {
		t.Fatalf("Chat 报错: %v", err)
	}
	if full != "你好，世界" {
		t.Fatalf("完整文本不对: %q", full)
	}
	if got.String() != "你好，世界" {
		t.Fatalf("流式增量不对: %q", got.String())
	}
}

func writeSSE(w http.ResponseWriter, data string) {
	w.Write([]byte("data: " + data + "\n\n"))
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/aiagent/ -run TestOpenAI -v`
Expected: FAIL（`newOpenAIClient` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/client_openai.go
package aiagent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

type openAIClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func newOpenAIClient(b config.AIBackend) *openAIClient {
	return &openAIClient{
		baseURL: strings.TrimRight(b.BaseURL, "/"),
		apiKey:  b.APIKey,
		model:   b.Model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

type oaReqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *openAIClient) Chat(ctx context.Context, msgs []Message, onDelta func(string)) (string, error) {
	reqMsgs := make([]oaReqMessage, len(msgs))
	for i, m := range msgs {
		reqMsgs[i] = oaReqMessage{Role: m.Role, Content: m.Content}
	}
	body, _ := json.Marshal(map[string]any{
		"model":    c.model,
		"messages": reqMsgs,
		"stream":   true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 AI 后端失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("AI 后端返回 HTTP %d", resp.StatusCode)
	}

	var full strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 跳过非 JSON 行（注释/心跳）
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				full.WriteString(ch.Delta.Content)
				if onDelta != nil {
					onDelta(ch.Delta.Content)
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return full.String(), fmt.Errorf("读取 AI 流失败: %w", err)
	}
	return full.String(), nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/aiagent/ -run 'TestNewClient|TestOpenAI' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./internal/aiagent/
git add internal/aiagent/client.go internal/aiagent/client_test.go internal/aiagent/client_openai.go internal/aiagent/client_openai_test.go
git commit -m "feat(aiagent): LLMClient interface + OpenAI-format SSE client"
```

---

## Task 5: aiagent — Anthropic 格式客户端（anthropic-sdk-go）

**Files:**
- Modify: `internal/aiagent/client.go`（删除 Task 3 的临时占位）
- Create: `internal/aiagent/client_anthropic.go`
- Modify: `go.mod` / `go.sum`

- [ ] **Step 1: 装依赖**

Run: `go get github.com/anthropics/anthropic-sdk-go@latest`
Expected: go.mod 出现该依赖。

- [ ] **Step 2: 删掉 Task 3 占位**

删除 `internal/aiagent/client.go` 末尾这段：

```go
// 临时占位：Task 5 用真实 anthropic-sdk-go 实现替换。
func newAnthropicClient(b config.AIBackend) LLMClient { return newOpenAIClient(b) }
```

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/client_anthropic.go
package aiagent

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

type anthropicClient struct {
	client anthropic.Client
	model  string
}

func newAnthropicClient(b config.AIBackend) *anthropicClient {
	opts := []option.RequestOption{}
	if b.APIKey != "" {
		opts = append(opts, option.WithAPIKey(b.APIKey))
	}
	if b.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(b.BaseURL))
	}
	model := b.Model
	if model == "" {
		model = string(anthropic.ModelClaudeOpus4_8)
	}
	return &anthropicClient{client: anthropic.NewClient(opts...), model: model}
}

func (c *anthropicClient) Chat(ctx context.Context, msgs []Message, onDelta func(string)) (string, error) {
	// Anthropic 把 system 单独传，user/assistant 进 messages。
	var system string
	var conv []anthropic.MessageParam
	for _, m := range msgs {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case "assistant":
			conv = append(conv, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		default:
			conv = append(conv, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 8000,
		Messages:  conv,
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}
	stream := c.client.Messages.NewStreaming(ctx, params)
	msg := anthropic.Message{}
	var full string
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return full, err
		}
		if d, ok := ev.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if td, ok := d.Delta.AsAny().(anthropic.TextDelta); ok {
				full += td.Text
				if onDelta != nil {
					onDelta(td.Text)
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return full, fmt.Errorf("调用 Claude 后端失败: %w", err)
	}
	return full, nil
}
```

- [ ] **Step 4: 运行确认编译 + 既有测试通过**

Run: `go build ./... && go test ./internal/aiagent/ -v`
Expected: 编译通过；Task 3/4 测试 PASS。

> 注：anthropic-sdk-go 的精确符号（`anthropic.Client` vs `*anthropic.Client`、`option.WithBaseURL`、`Message.Accumulate`、`ContentBlockDeltaEvent`/`TextDelta`）以 claude-api 技能 `go/claude-api.md` 为准；若编译报符号不符，按该文档修正类型名，**不要臆造**。

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum internal/aiagent/client.go internal/aiagent/client_anthropic.go
git commit -m "feat(aiagent): Anthropic-format backend via anthropic-sdk-go"
```

---

## Task 6: aiagent — JSON 动作 DSL 解析

**Files:**
- Create: `internal/aiagent/action.go`
- Test: `internal/aiagent/action_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/aiagent/action_test.go
package aiagent

import "testing"

func TestParseActionFromFencedBlock(t *testing.T) {
	reply := "好的，我来帮你设置订阅源。\n\n```gateway-action\n" +
		`{"action":"set_source","type":"subscription","url":"https://e.com/s"}` +
		"\n```\n确认后即可生效。"
	act, ok := ParseAction(reply)
	if !ok {
		t.Fatal("应解析出动作")
	}
	if act.Action != "set_source" || act.Type != "subscription" || act.URL != "https://e.com/s" {
		t.Fatalf("动作字段不对: %+v", act)
	}
}

func TestParseActionNoBlock(t *testing.T) {
	if _, ok := ParseAction("纯聊天没有动作块"); ok {
		t.Fatal("无动作块应返回 false")
	}
}

func TestParseActionMalformed(t *testing.T) {
	reply := "```gateway-action\n{不是合法json}\n```"
	if _, ok := ParseAction(reply); ok {
		t.Fatal("非法 JSON 应返回 false")
	}
}

func TestActionIsWrite(t *testing.T) {
	if (Action{Action: "get_status"}).IsWrite() {
		t.Fatal("get_status 应是读")
	}
	if !(Action{Action: "set_mode"}).IsWrite() {
		t.Fatal("set_mode 应是写")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/aiagent/ -run 'TestParseAction|TestActionIsWrite' -v`
Expected: FAIL（`ParseAction`/`Action` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/action.go
package aiagent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Action 是 agent 输出的一个网关动作。一轮最多一个。
type Action struct {
	Action  string `json:"action"`
	Type    string `json:"type,omitempty"`    // set_source: subscription/file/external/remote/none
	URL     string `json:"url,omitempty"`     // set_source subscription
	Path    string `json:"path,omitempty"`    // set_source file
	Server  string `json:"server,omitempty"`  // set_source external/remote
	Port    int    `json:"port,omitempty"`
	Kind    string `json:"kind,omitempty"`    // http/socks5
	Mode    string `json:"mode,omitempty"`    // set_mode / set_gateway_mode
	Enabled *bool  `json:"enabled,omitempty"` // toggle_*
	Verdict string `json:"verdict,omitempty"` // add_rule: direct/proxy/reject
	Rule    string `json:"rule,omitempty"`    // add_rule body
	Summary string `json:"summary,omitempty"` // finish
}

var actionBlockRe = regexp.MustCompile("(?s)```gateway-action\\s*(.*?)```")

// ParseAction 从 agent 回复里提取第一个 gateway-action 代码块并解析。
func ParseAction(reply string) (Action, bool) {
	m := actionBlockRe.FindStringSubmatch(reply)
	if len(m) < 2 {
		return Action{}, false
	}
	var a Action
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &a); err != nil {
		return Action{}, false
	}
	if a.Action == "" {
		return Action{}, false
	}
	return a, true
}

// writeActions 是会改配置的动作（需用户确认）。
var writeActions = map[string]bool{
	"set_source": true, "set_mode": true, "toggle_tun": true,
	"set_gateway_mode": true, "toggle_adblock": true, "add_rule": true,
	"start": true, "stop": true, "restart": true,
}

// IsWrite 报告该动作是否会改网关配置/状态。
func (a Action) IsWrite() bool { return writeActions[a.Action] }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/aiagent/ -run 'TestParseAction|TestActionIsWrite' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./internal/aiagent/
git add internal/aiagent/action.go internal/aiagent/action_test.go
git commit -m "feat(aiagent): JSON action DSL parser"
```

---

## Task 7: aiagent — Controller 接口 + 动作执行器（确认式写）

**Files:**
- Create: `internal/aiagent/controller.go`、`internal/aiagent/executor.go`
- Test: `internal/aiagent/executor_test.go`

- [ ] **Step 1: 写失败测试（用 fake Controller）**

```go
// internal/aiagent/executor_test.go
package aiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

type fakeController struct {
	status   app.Status
	setMode  string
	tunCalls int
	srcSet   *config.SourceConfig
}

func (f *fakeController) Status() app.Status { return f.status }
func (f *fakeController) Health() string      { return "健康" }
func (f *fakeController) SetMode(_ context.Context, m string) error { f.setMode = m; return nil }
func (f *fakeController) ToggleTUN(context.Context) error          { f.tunCalls++; return nil }
func (f *fakeController) ToggleAdblock(context.Context) error      { return nil }
func (f *fakeController) SetGatewayMode(context.Context, string) error { return nil }
func (f *fakeController) SetSource(_ context.Context, s config.SourceConfig) error { f.srcSet = &s; return nil }
func (f *fakeController) AddRule(context.Context, string, string) error { return nil }
func (f *fakeController) Start(context.Context) error { return nil }
func (f *fakeController) Stop() error                 { return nil }

func TestExecuteReadActionInline(t *testing.T) {
	f := &fakeController{status: app.Status{Mode: "rule", Running: true}}
	ex := NewExecutor(f, func(string) bool { t.Fatal("读操作不应请求确认"); return false })
	res := ex.Execute(context.Background(), Action{Action: "get_status"})
	if res.Err != nil {
		t.Fatalf("get_status 不应报错: %v", res.Err)
	}
	if !strings.Contains(res.Observation, "rule") {
		t.Fatalf("观测应含当前模式: %q", res.Observation)
	}
}

func TestExecuteWriteRequiresConfirm(t *testing.T) {
	f := &fakeController{}
	denied := NewExecutor(f, func(string) bool { return false })
	res := denied.Execute(context.Background(), Action{Action: "set_mode", Mode: "global"})
	if f.setMode != "" {
		t.Fatal("用户拒绝时不应执行")
	}
	if !strings.Contains(res.Observation, "拒绝") {
		t.Fatalf("应回灌用户拒绝: %q", res.Observation)
	}

	f2 := &fakeController{}
	ok := NewExecutor(f2, func(string) bool { return true })
	ok.Execute(context.Background(), Action{Action: "set_mode", Mode: "global"})
	if f2.setMode != "global" {
		t.Fatalf("确认后应执行 set_mode，得到 %q", f2.setMode)
	}
}

func TestToggleTUNIdempotent(t *testing.T) {
	// 当前 TUN 已开，要求 enabled:true → 不应再 toggle
	f := &fakeController{status: app.Status{TUN: true}}
	tru := true
	ex := NewExecutor(f, func(string) bool { return true })
	ex.Execute(context.Background(), Action{Action: "toggle_tun", Enabled: &tru})
	if f.tunCalls != 0 {
		t.Fatalf("目标态已满足不应 toggle，调用了 %d 次", f.tunCalls)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/aiagent/ -run 'TestExecute|TestToggle' -v`
Expected: FAIL（`Controller`/`NewExecutor` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/controller.go
package aiagent

import (
	"context"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Controller 是执行器需要的网关能力。*app.App 直接满足它（app 不 import aiagent，无循环）。
type Controller interface {
	Status() app.Status
	Health() string
	SetMode(ctx context.Context, mode string) error
	ToggleTUN(ctx context.Context) error
	ToggleAdblock(ctx context.Context) error
	SetGatewayMode(ctx context.Context, mode string) error
	SetSource(ctx context.Context, src config.SourceConfig) error
	AddRule(ctx context.Context, verdict, rule string) error
	Start(ctx context.Context) error
	Stop() error
}
```

> 注：`*app.App` 已有 Status/SetMode/ToggleTUN/ToggleAdblock/SetGatewayMode/SetSource/Start/Stop（Task 2 加了 AddRule）。但 `App.Health()` 返回 `app.SourceHealth` 而非 `string`——在 `internal/app` 加一个薄封装 `func (a *App) HealthText() string`，并把接口方法名改用 `HealthText`。修正接口：把上面 `Health() string` 改为 `HealthText() string`，fake 里同步改。在 app.go 追加：
> ```go
> // HealthText 给 AI 执行器一个一行健康摘要。
> func (a *App) HealthText() string {
> 	h := a.Health()
> 	if h.Healthy { return "代理源健康" }
> 	if h.Detail != "" { return "代理源异常: " + h.Detail }
> 	return "代理源状态未知"
> }
> ```
> （`SourceHealth` 字段名以 `internal/app/supervisor.go` 实际为准；若不是 `Healthy`/`Detail`，照实改。）

```go
// internal/aiagent/executor.go
package aiagent

import (
	"context"
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Result 是一个动作执行后的回灌内容。Observation 会作为下一轮 user 消息喂回 agent。
type Result struct {
	Observation string
	Err         error
	Done        bool // finish 动作 → 结束本轮
}

// ConfirmFunc 渲染计划卡并返回用户是否批准。
type ConfirmFunc func(plan string) bool

// Executor 执行动作：读 inline，写先 Confirm。
type Executor struct {
	ctrl    Controller
	confirm ConfirmFunc
}

func NewExecutor(ctrl Controller, confirm ConfirmFunc) *Executor {
	return &Executor{ctrl: ctrl, confirm: confirm}
}

func (e *Executor) Execute(ctx context.Context, a Action) Result {
	if a.Action == "finish" {
		return Result{Observation: a.Summary, Done: true}
	}
	// 读操作：inline
	switch a.Action {
	case "get_status":
		s := e.ctrl.Status()
		return Result{Observation: fmt.Sprintf(
			"当前状态: 运行=%v 模式=%s TUN=%v 去广告=%v 网关模式=%s 源=%s",
			s.Running, s.Mode, s.TUN, s.Adblock, s.GatewayMode, s.Source)}
	case "get_health":
		return Result{Observation: e.ctrl.HealthText()}
	}
	// 写操作：确认式
	if a.IsWrite() {
		plan := e.planCard(a)
		if !e.confirm(plan) {
			return Result{Observation: "用户拒绝了这个操作，请换个方案或询问用户。"}
		}
	}
	return e.runWrite(ctx, a)
}

func (e *Executor) runWrite(ctx context.Context, a Action) Result {
	var err error
	switch a.Action {
	case "set_mode":
		err = e.ctrl.SetMode(ctx, a.Mode)
	case "set_gateway_mode":
		err = e.ctrl.SetGatewayMode(ctx, a.Mode)
	case "toggle_tun":
		if a.Enabled == nil || *a.Enabled != e.ctrl.Status().TUN {
			err = e.ctrl.ToggleTUN(ctx)
		}
	case "toggle_adblock":
		if a.Enabled == nil || *a.Enabled != e.ctrl.Status().Adblock {
			err = e.ctrl.ToggleAdblock(ctx)
		}
	case "set_source":
		err = e.ctrl.SetSource(ctx, e.sourceFromAction(a))
	case "add_rule":
		err = e.ctrl.AddRule(ctx, a.Verdict, a.Rule)
	case "start", "restart":
		err = e.ctrl.Start(ctx)
	case "stop":
		err = e.ctrl.Stop()
	default:
		return Result{Observation: fmt.Sprintf("未知动作 %q，请只用约定的动作。", a.Action),
			Err: fmt.Errorf("unknown action %q", a.Action)}
	}
	if err != nil {
		return Result{Observation: "执行失败: " + err.Error(), Err: err}
	}
	return Result{Observation: "执行成功。"}
}

func (e *Executor) sourceFromAction(a Action) config.SourceConfig {
	src := config.SourceConfig{Type: a.Type}
	switch a.Type {
	case "subscription":
		src.Subscription = config.SubscriptionSource{URL: a.URL, Name: "subscription"}
	case "file":
		src.File = config.FileSource{Path: a.Path}
	case "external":
		src.External = config.ExternalProxy{Server: a.Server, Port: a.Port, Kind: a.Kind, Name: "外部代理"}
	case "remote":
		src.Remote = config.RemoteProxy{Server: a.Server, Port: a.Port, Kind: a.Kind, Name: "远程代理"}
	}
	return src
}

func (e *Executor) planCard(a Action) string {
	switch a.Action {
	case "set_source":
		return fmt.Sprintf("设置代理源 → type=%s url=%s path=%s server=%s:%d", a.Type, a.URL, a.Path, a.Server, a.Port)
	case "set_mode":
		return "切换分流模式 → " + a.Mode
	case "set_gateway_mode":
		return "切换网关模式 → " + a.Mode
	case "toggle_tun":
		return fmt.Sprintf("设置 TUN → %v", a.Enabled)
	case "toggle_adblock":
		return fmt.Sprintf("设置去广告 → %v", a.Enabled)
	case "add_rule":
		return fmt.Sprintf("新增规则 → [%s] %s", a.Verdict, a.Rule)
	case "start", "restart":
		return "启动/重启网关"
	case "stop":
		return "停止网关"
	}
	return a.Action
}
```

> 同步修正 Task 7 测试里的 fake：把 `Health() string` 改成 `HealthText() string`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/aiagent/ -run 'TestExecute|TestToggle' -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./...
git add internal/aiagent/controller.go internal/aiagent/executor.go internal/aiagent/executor_test.go internal/app/app.go
git commit -m "feat(aiagent): Controller interface + confirm-gated action executor"
```

---

## Task 8: aiagent — system prompt + Session 对话循环

**Files:**
- Create: `internal/aiagent/prompt.go`、`internal/aiagent/session.go`
- Test: `internal/aiagent/session_test.go`

- [ ] **Step 1: 写失败测试（用 fake LLMClient 脚本化回复）**

```go
// internal/aiagent/session_test.go
package aiagent

import (
	"context"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

// scriptClient 按预设依次返回回复，忽略输入。
type scriptClient struct {
	replies []string
	i       int
}

func (s *scriptClient) Chat(_ context.Context, _ []Message, onDelta func(string)) (string, error) {
	r := s.replies[s.i]
	s.i++
	if onDelta != nil {
		onDelta(r)
	}
	return r, nil
}

func TestSessionRunsActionThenFinishes(t *testing.T) {
	f := &fakeController{status: app.Status{Mode: "rule"}}
	llm := &scriptClient{replies: []string{
		"我先看下状态。\n```gateway-action\n{\"action\":\"get_status\"}\n```",
		"状态正常。\n```gateway-action\n{\"action\":\"finish\",\"summary\":\"已确认状态\"}\n```",
	}}
	ex := NewExecutor(f, func(string) bool { return true })
	sess := NewSession(llm, ex)
	out, err := sess.Handle(context.Background(), "看下状态", nil)
	if err != nil {
		t.Fatalf("Handle 报错: %v", err)
	}
	if out == "" {
		t.Fatal("应有最终回复")
	}
	if llm.i != 2 {
		t.Fatalf("应走两轮（动作 + finish），走了 %d", llm.i)
	}
}

func TestSessionStopsAtMaxTurns(t *testing.T) {
	f := &fakeController{}
	// 永远返回同一个读动作，永不 finish → 应被 maxTurns 截断
	llm := &scriptClient{}
	for i := 0; i < 20; i++ {
		llm.replies = append(llm.replies, "```gateway-action\n{\"action\":\"get_status\"}\n```")
	}
	ex := NewExecutor(f, func(string) bool { return true })
	sess := NewSession(llm, ex)
	_, err := sess.Handle(context.Background(), "循环", nil)
	if err == nil {
		t.Fatal("超过 maxTurns 应报错")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/aiagent/ -run TestSession -v`
Expected: FAIL（`NewSession`/`systemPrompt` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/aiagent/prompt.go
package aiagent

import "fmt"

// systemPrompt 描述 agent 角色 + 动作 DSL 协议。注入当前网关状态快照。
func systemPrompt(ctrl Controller) string {
	s := ctrl.Status()
	return fmt.Sprintf(`你是 lan-proxy-gateway（局域网透明代理网关）的内置配网助手。
帮中文用户用最少的步骤把网关配好。回答简洁、口语化。

当你需要查询状态或改配置时，在回复末尾输出**恰好一个** gateway-action 代码块：
`+"```gateway-action"+`
{"action":"<动作名>", ...字段}
`+"```"+`
可用动作：
- get_status / get_health：查状态/健康（自动执行，无需确认）
- set_source {type:subscription|file|external|remote|none, url|path|server,port,kind}
- set_mode {mode:rule|global|direct}
- set_gateway_mode {mode:tun|forward}
- toggle_tun {enabled:true|false}
- toggle_adblock {enabled:true|false}
- add_rule {verdict:direct|proxy|reject, rule:"DOMAIN-SUFFIX,example.com"}
- start / stop / restart
- finish {summary:"做完了什么"}：任务完成时调用，结束对话。

规则：一轮最多一个动作；改配置的动作会先让用户确认。任务做完务必用 finish 收尾。

当前网关状态：运行=%v 模式=%s TUN=%v 去广告=%v 网关模式=%s 源=%s。`,
		s.Running, s.Mode, s.TUN, s.Adblock, s.GatewayMode, s.Source)
}
```

```go
// internal/aiagent/session.go
package aiagent

import (
	"context"
	"errors"

	_ "github.com/tght/lan-proxy-gateway/internal/app" // 保证 app 被链接（Controller 类型）
)

const maxTurns = 8

// Session 是一次「用户说一句话 → agent 多轮执行 → 给最终答复」的对话。
type Session struct {
	llm  LLMClient
	exec *Executor
	hist []Message
}

func NewSession(llm LLMClient, exec *Executor) *Session {
	return &Session{llm: llm, exec: exec}
}

// Handle 处理一句用户输入，跑完动作循环，返回 agent 的最终自然语言回复。
// onDelta 把流式文本增量回调给 UI（可 nil）。
func (s *Session) Handle(ctx context.Context, userInput string, onDelta func(string)) (string, error) {
	if len(s.hist) == 0 {
		s.hist = append(s.hist, Message{Role: "system", Content: systemPrompt(s.exec.ctrl)})
	}
	s.hist = append(s.hist, Message{Role: "user", Content: userInput})

	var lastReply string
	for turn := 0; turn < maxTurns; turn++ {
		reply, err := s.llm.Chat(ctx, s.hist, onDelta)
		if err != nil {
			return "", err
		}
		s.hist = append(s.hist, Message{Role: "assistant", Content: reply})
		lastReply = reply

		act, ok := ParseAction(reply)
		if !ok {
			return lastReply, nil // 没有动作 = 纯聊天回复，结束
		}
		res := s.exec.Execute(ctx, act)
		if res.Done {
			return lastReply, nil
		}
		// 把执行观测回灌为 user 消息，继续下一轮
		s.hist = append(s.hist, Message{Role: "user", Content: "[执行结果] " + res.Observation})
	}
	return lastReply, errors.New("AI 助手步骤过多已中止，请用菜单手动操作或换个说法")
}
```

> 注：`session.go` 的 `import _ ".../internal/app"` 仅为表达依赖意图，可省略；若 `go vet` 报无用导入则删掉该行。`Executor.ctrl` 字段在同包内可直接访问。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/aiagent/ -run TestSession -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
go test ./...
git add internal/aiagent/prompt.go internal/aiagent/session.go internal/aiagent/session_test.go
git commit -m "feat(aiagent): system prompt + multi-turn session loop"
```

---

## Task 9: console — 主提示符自然语言路由 + AI 后端子页

**Files:**
- Modify: `internal/console/console.go`（`main()` default 分支 + 菜单项）
- Create: `internal/console/ai_console.go`（agent 接线 + 后端管理页）

- [ ] **Step 1: 写实现（无独立单测——交互层，靠手验 Task 12）**

新建 `internal/console/ai_console.go`：

```go
// internal/console/ai_console.go
package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/aiagent"
)

// aiAvailable 报告 AI 是否启用且能构造客户端。
func (c *consoleUI) aiAvailable() bool {
	return c.app.Cfg.AI.Enabled && len(c.app.Cfg.AI.Backends) > 0
}

// handleNaturalLanguage 把一句话交给 AI 配网助手跑一轮。
func (c *consoleUI) handleNaturalLanguage(ctx context.Context, line string) {
	if !c.aiAvailable() {
		warnC.Fprintln(c.out, "无效选项（回车 / R 刷新 · M 菜单 · N 切节点 · T 重测 · Q 退出）")
		return
	}
	backend := c.app.Cfg.AI.ActiveBackend()
	llm, err := aiagent.NewClient(backend)
	if err != nil {
		warnC.Fprintln(c.out, "AI 后端不可用: "+err.Error())
		return
	}
	confirm := func(plan string) bool {
		fmt.Fprintln(c.out, "\nAI 准备执行：")
		fmt.Fprintln(c.out, "  "+plan)
		fmt.Fprint(c.out, "确认? [y/N] ")
		ans := strings.ToLower(strings.TrimSpace(c.readLine()))
		return ans == "y" || ans == "yes"
	}
	exec := aiagent.NewExecutor(c.app, confirm)
	sess := aiagent.NewSession(llm, exec)

	fmt.Fprintln(c.out, "\n🤖 ")
	_, err = sess.Handle(ctx, line, func(s string) { fmt.Fprint(c.out, s) })
	fmt.Fprintln(c.out)
	if err != nil {
		warnC.Fprintln(c.out, err.Error())
	}
}
```

在 `internal/console/console.go` 的 `main()`（第 467 行起）改造 default 分支。**注意**：`choice` 是 lowercased+trimmed 的，自然语言要用原始行。把读入改成保留原始行：

```go
func (c *consoleUI) main(ctx context.Context) error {
	for {
		c.drawDashboardOnce(ctx)
		raw := c.readLine()
		choice := strings.ToLower(strings.TrimSpace(raw))
		switch choice {
		case "", "r":
		case "m", "menu":
			if c.screenMenu(ctx) {
				return nil
			}
		case "n":
			c.screenSwitchNode(ctx)
		case "t":
			c.screenSource(ctx)
		case "q", "exit", "quit":
			return nil
		default:
			// 非命令 → 交给 AI 配网助手（用原始行保留大小写/中文）
			c.handleNaturalLanguage(ctx, strings.TrimSpace(raw))
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}
```

- [ ] **Step 2: 编译确认**

Run: `go build ./...`
Expected: 编译通过（`*app.App` 满足 `aiagent.Controller`——若报不满足，对照 Task 7 接口逐一核对 App 方法签名）。

- [ ] **Step 3: 加菜单提示（可选小步）**

在首页 prompt 提示文案（`drawDashboardOnce` 末尾或 main 的提示里）追加一句引导，例如在无效选项文案旁说明「或直接输入一句话让 AI 帮你配网」。定位 `dashboard.go` 里画提示行的位置，追加：

```go
	fmt.Fprintln(w, "  💬 直接输入一句话，让 AI 配网助手帮你（如：帮我设置订阅源 https://...）")
```

- [ ] **Step 4: 编译 + 全测**

Run: `go build ./... && go test ./...`
Expected: 通过。

- [ ] **Step 5: 提交**

```bash
git add internal/console/ai_console.go internal/console/console.go internal/console/dashboard.go
git commit -m "feat(console): natural-language routing to AI config agent"
```

---

## Task 10: console — 网速 sparkline

**Files:**
- Create: `internal/console/sparkline.go`
- Test: `internal/console/sparkline_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/console/sparkline_test.go
package console

import (
	"strings"
	"testing"
)

func TestSparklinePushAndRender(t *testing.T) {
	s := newSparkline(8)
	for _, v := range []float64{0, 1, 2, 3, 4, 5, 6, 7} {
		s.push(v)
	}
	out := s.render()
	runes := []rune(out)
	if len(runes) != 8 {
		t.Fatalf("应渲染 8 格，得到 %d", len(runes))
	}
	// 最大值映射到最高块 █，最小值映射到最低块 ▁
	if runes[7] != '█' {
		t.Fatalf("最大值应是 █，得到 %q", string(runes[7]))
	}
	if runes[0] != '▁' {
		t.Fatalf("最小值应是 ▁，得到 %q", string(runes[0]))
	}
}

func TestSparklineRingBufferCaps(t *testing.T) {
	s := newSparkline(4)
	for i := 0; i < 10; i++ {
		s.push(float64(i))
	}
	if r := []rune(s.render()); len(r) != 4 {
		t.Fatalf("环形缓冲应固定 4 格，得到 %d", len(r))
	}
}

func TestSparklineAllZero(t *testing.T) {
	s := newSparkline(4)
	for i := 0; i < 4; i++ {
		s.push(0)
	}
	if strings.Trim(s.render(), "▁") != "" {
		t.Fatal("全零应全是最低块，不应除零崩")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/console/ -run TestSparkline -v`
Expected: FAIL（`newSparkline` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/console/sparkline.go
package console

// sparkline 是定长环形缓冲，用 Unicode 区块字符渲染成柱状图。
type sparkline struct {
	buf  []float64
	size int
	n    int // 已写入总数（>size 时取最近 size 个）
}

var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

func newSparkline(size int) *sparkline {
	return &sparkline{buf: make([]float64, size), size: size}
}

func (s *sparkline) push(v float64) {
	if v < 0 {
		v = 0
	}
	s.buf[s.n%s.size] = v
	s.n++
}

// ordered 返回按时间从旧到新排列的窗口值。
func (s *sparkline) ordered() []float64 {
	out := make([]float64, 0, s.size)
	count := s.n
	if count > s.size {
		count = s.size
	}
	start := 0
	if s.n > s.size {
		start = s.n % s.size
	}
	for i := 0; i < count; i++ {
		out = append(out, s.buf[(start+i)%s.size])
	}
	// 不足 size 时左侧补零，保证渲染宽度恒定
	for len(out) < s.size {
		out = append([]float64{0}, out...)
	}
	return out
}

func (s *sparkline) render() string {
	vals := s.ordered()
	max := 0.0
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	r := make([]rune, len(vals))
	for i, v := range vals {
		idx := 0
		if max > 0 {
			idx = int(v / max * float64(len(sparkBlocks)-1))
			if idx >= len(sparkBlocks) {
				idx = len(sparkBlocks) - 1
			}
		}
		r[i] = sparkBlocks[idx]
	}
	return string(r)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/console/ -run TestSparkline -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/console/sparkline.go internal/console/sparkline_test.go
git commit -m "feat(console): network-speed sparkline ring buffer"
```

---

## Task 11: console — 稳定性健康条

**Files:**
- Create: `internal/console/health.go`
- Test: `internal/console/health_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/console/health_test.go
package console

import (
	"strings"
	"testing"
)

func TestHealthBarRecordsAndRenders(t *testing.T) {
	h := newHealthBar(60)
	// 记 3 次健康
	for i := 0; i < 3; i++ {
		h.record(true)
	}
	body := h.renderBar()
	if len([]rune(body)) != 60 {
		t.Fatalf("应渲染 60 格，得到 %d", len([]rune(body)))
	}
}

func TestHealthBarTitleHasCountdown(t *testing.T) {
	h := newHealthBar(60)
	title := h.renderTitle(42)
	if !strings.Contains(title, "近 60 次记录") {
		t.Fatalf("标题应含『近 60 次记录』: %q", title)
	}
	if !strings.Contains(title, "42") {
		t.Fatalf("标题应含倒计时秒数: %q", title)
	}
}

func TestHealthBarFooterPastNow(t *testing.T) {
	h := newHealthBar(60)
	f := h.renderFooter()
	if !strings.Contains(f, "PAST") || !strings.Contains(f, "NOW") {
		t.Fatalf("脚注应含 PAST/NOW: %q", f)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/console/ -run TestHealthBar -v`
Expected: FAIL（`newHealthBar` 未定义）

- [ ] **Step 3: 写实现**

```go
// internal/console/health.go
package console

import "fmt"

// healthBar 是定长环形缓冲，每分钟记一次测速结果，渲染成 PAST→NOW 的健康柱。
type healthBar struct {
	ok   []bool // 每格是否健康
	used []bool // 该格是否已有记录
	size int
	n    int
}

func newHealthBar(size int) *healthBar {
	return &healthBar{ok: make([]bool, size), used: make([]bool, size), size: size}
}

func (h *healthBar) record(healthy bool) {
	i := h.n % h.size
	h.ok[i] = healthy
	h.used[i] = true
	h.n++
}

// renderTitle 渲染标题：左『近 N 次记录』，右『NNs 后刷新』。
func (h *healthBar) renderTitle(secsToNext int) string {
	return fmt.Sprintf("近 %d 次记录%s%ds 后刷新", h.size, strings_pad(), secsToNext)
}

// renderBar 渲染 60 根竖条：健康=▮，异常=▯，无记录=空格。
func (h *healthBar) renderBar() string {
	r := make([]rune, h.size)
	count := h.n
	if count > h.size {
		count = h.size
	}
	start := 0
	if h.n > h.size {
		start = h.n % h.size
	}
	for i := 0; i < h.size; i++ {
		idx := (start + i) % h.size
		switch {
		case !h.used[idx]:
			r[i] = ' '
		case h.ok[idx]:
			r[i] = '▮'
		default:
			r[i] = '▯'
		}
	}
	return string(r)
}

func (h *healthBar) renderFooter() string {
	pad := h.size - len("PAST") - len("NOW")
	if pad < 1 {
		pad = 1
	}
	return "PAST" + spaces(pad) + "NOW"
}

func spaces(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func strings_pad() string { return "   " }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/console/ -run TestHealthBar -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/console/health.go internal/console/health_test.go
git commit -m "feat(console): stability health bar ring buffer"
```

---

## Task 12: console — 把 sparkline + 健康条接进 dashboard，并起每分钟测速 ticker

**Files:**
- Modify: `internal/console/console.go`（`consoleUI` 加字段 + 启动 ticker）、`internal/console/dashboard.go`（渲染两条）

- [ ] **Step 1: 在 `consoleUI` 加字段并初始化**

在 `internal/console/console.go` 的 `consoleUI` 结构体定义处加字段：

```go
	spark  *sparkline
	health *healthBar
```

在构造 `consoleUI` 的地方（grep `&consoleUI{` 定位）初始化：

```go
		spark:  newSparkline(40),
		health: newHealthBar(60),
```

- [ ] **Step 2: 起每分钟测速 ticker（记健康）**

在 `main(ctx)` 进入 for 循环前启动后台 goroutine，每分钟用 `engine.Client.GroupDelay` 测一次主出口组延迟、记入 health：

```go
	go c.runHealthTicker(ctx)
```

新增方法（放 `ai_console.go` 或 `dashboard.go` 均可，建议新文件 `internal/console/health_ticker.go`）：

```go
// internal/console/health_ticker.go
package console

import (
	"context"
	"time"
)

// runHealthTicker 每分钟测一次主出口组延迟，结果记入 health 柱。
func (c *consoleUI) runHealthTicker(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	probe := func() {
		healthy := false
		if c.app.Engine != nil && c.app.Engine.Running() {
			cli := c.app.Engine.APIClient() // 见下方说明
			if cli != nil {
				pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := cli.GroupDelay(pctx, "Proxy", "http://www.gstatic.com/generate_204", 3000)
				cancel()
				healthy = err == nil
			}
		}
		c.health.record(healthy)
	}
	probe() // 立即记一格，避免开局空白
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			probe()
		}
	}
}
```

> 说明：需要拿到 `*engine.Client`。先 grep `func (e *Engine)` 看是否已有暴露 API client 的方法；若无，在 `internal/engine` 加一个 `func (e *Engine) APIClient() *Client { return e.api }`（字段名以实际为准）。测速组名 "Proxy" 若实际主组名不同（如 "🚀 节点选择"），用 `ListProxyGroups` 取第一个 selector 组名，或回退 "GLOBAL"。**实现时先确认组名来源**，避免硬编码测错组。

- [ ] **Step 3: 每次刷新 push 网速到 sparkline**

在 `dashboard.go` 计算出当前上/下行速率处（约 `dashboard.go:158-166` 算 down/up 速率附近），把下行速率 push 进 sparkline：

```go
	c.spark.push(float64(downRate)) // downRate 为本周期下行字节/秒，变量名以实际为准
```

- [ ] **Step 4: 渲染两条到 dashboard**

在 `dashboard.go` 打印 `↓ x/s ↑ y/s` 那行（约 `dashboard.go:393`）之后追加：

```go
	// 网速柱状图（红）
	fmt.Fprintf(w, "  %s\n", speedC.Sprint(c.spark.render()))
	// 稳定性健康条（绿）
	secsToNext := 60 - time.Now().Second() // 距下次整分钟
	fmt.Fprintln(w, "  "+c.health.renderTitle(secsToNext))
	fmt.Fprintln(w, "  "+healthC.Sprint(c.health.renderBar()))
	fmt.Fprintln(w, "  "+c.health.renderFooter())
```

> `speedC`（红）/`healthC`（绿）是颜色封装——参照 `console.go` 里已有的 `warnC` 等 `color.New(...)` 定义补两个：
> ```go
> var speedC  = color.New(color.FgRed)
> var healthC = color.New(color.FgGreen)
> ```
> （若项目用 lipgloss 而非 fatih/color，照既有上色方式改。）`time` 包确保已 import。

- [ ] **Step 5: 编译 + 全测 + 提交**

Run: `go build ./... && go test ./...`
Expected: 通过。

```bash
git add internal/console/console.go internal/console/dashboard.go internal/console/health_ticker.go internal/engine/engine.go
git commit -m "feat(console): wire sparkline + health bar into dashboard with 1-min speed test"
```

---

## Task 13: 收尾 — 手动验证 + 全量回归

- [ ] **Step 1: 构建**

Run: `make build`
Expected: 产出 `gateway` 二进制无错。

- [ ] **Step 2: 全量测试**

Run: `go test ./...`
Expected: 全绿。

- [ ] **Step 3: 手验 AI 助手**

```bash
./gateway   # 进终端控制台
# 在主提示符输入：帮我把分流模式切成 global
# 期望：看到 🤖 流式回复 + 「AI 准备执行：切换分流模式 → global」+ [y/N]
# 输 y → 看到「执行成功」→ 输 status / 看 dashboard 验证 Mode=global
```

- [ ] **Step 4: 手验图表**

```bash
# 主屏应见：↓/↑ 速率行下方一条红色柱状网速图；
# 「近 60 次记录 ... Ns 后刷新」标题 + 绿色健康柱 + PAST...NOW 脚注；
# 等 1 分钟看健康柱新增一格、倒计时归零重置。
```

- [ ] **Step 5: 最终提交（如有手验小修）**

```bash
git add -A
git commit -m "chore: finalize AI config agent + terminal polish"
```

---

## 自查（spec 覆盖核对）

- 双格式后端：Task 4（openai）+ Task 5（anthropic）✅
- 内置免费零配置：Task 1（normalizeAI 注入）✅
- JSON 动作 DSL：Task 6 ✅
- 确认式写操作 + diff 计划卡：Task 7（planCard + confirm）✅
- 主提示符自然语言入口：Task 9 ✅
- key 脱敏不泄漏：Task 2（aiStatus）+ Task 1（normalize 覆盖内置 key）✅
- 网速 sparkline（图1）：Task 10 + Task 12 ✅
- 稳定性健康条 + 每分钟测速 + PAST→NOW + 倒计时（图2）：Task 11 + Task 12 ✅
- AI 后端管理子页：**部分**——Task 9 做了「构造 active 后端 + 用」；增删/切换/测试连通的菜单子页未单列任务。**补充说明**：一期可手改 `gateway.yaml` 切 active；完整后端管理 UI 列为后续小迭代（不阻塞主线价值）。若要本轮做，在 Task 9 后追加一个「screenAIBackends」菜单页任务，复用既有 `screenXxx` 菜单模式。
