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
