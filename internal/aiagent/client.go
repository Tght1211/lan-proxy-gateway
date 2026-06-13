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
