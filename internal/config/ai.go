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
	// base_url 遵循 OpenAI 惯例：含 /v1，客户端再拼 /chat/completions。
	// 实测 {base}/v1/chat/completions 才是正确的 OpenAI 兼容端点。
	builtinAIBackendBaseURL = "http://load.hulupet.cn/proxy/openrouter-2/v1"
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
