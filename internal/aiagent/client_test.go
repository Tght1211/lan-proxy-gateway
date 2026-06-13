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
