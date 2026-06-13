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
