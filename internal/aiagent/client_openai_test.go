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
