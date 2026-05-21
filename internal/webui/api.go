package webui

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Snapshot 是 GET /api/status 返回给前端的完整状态。
// 字段名用 snake_case 跟前端 app.js render() 对齐。
type Snapshot struct {
	Version            string              `json:"version"`
	Platform           string              `json:"platform"`
	Running            bool                `json:"running"`
	LocalIP            string              `json:"local_ip"`
	Router             string              `json:"router"`
	GatewayMode        string              `json:"gateway_mode"`
	GatewayModeLabel   string              `json:"gateway_mode_label"`
	TUNEnabled         bool                `json:"tun_enabled"`
	DNSEnabled         bool                `json:"dns_enabled"`
	DNSPort            int                 `json:"dns_port"`
	TrafficMode        string              `json:"traffic_mode"`
	Adblock            bool                `json:"adblock"`
	AutoGroups         bool                `json:"auto_groups"`
	Rulesets           Rulesets            `json:"rulesets"`
	RulesetDescriptors []RulesetDescriptor `json:"ruleset_descriptors"`
	Rules              CustomRules         `json:"rules"`
	SourceType         string              `json:"source_type"`
	Source             SourcePayload       `json:"source"`
	SourceRuntime      SourceRuntime       `json:"source_runtime"`
	SourceProfiles     []SourcePayload     `json:"source_profiles,omitempty"`
	Script             ScriptPayload       `json:"script"`
	ProxyService       ProxyServicePayload `json:"proxy_service"`
	MixedPort          int                 `json:"mixed_port"`
	RedirPort          int                 `json:"redir_port"`
	MihomoAPIPort      int                 `json:"mihomo_api_port"`
	WebUIPort          int                 `json:"web_ui_port"`
	MixedPortDown      bool                `json:"mixed_port_down"`
	Connectivity       Connectivity        `json:"connectivity"`
}

type ProxyServicePayload struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
}

// SourceRuntime 是首页展示的只读运行态摘要：当前用哪个代理源、源健康状态，
// 以及 mihomo 实际选中的主策略组和节点。
type SourceRuntime struct {
	Label          string `json:"label"`
	Detail         string `json:"detail,omitempty"`
	Status         string `json:"status"` // "ok" | "warn" | "bad" | "unknown"
	StatusText     string `json:"status_text"`
	LastError      string `json:"last_error,omitempty"`
	CheckedAt      string `json:"checked_at,omitempty"`
	FallbackActive bool   `json:"fallback_active,omitempty"`
	ActiveGroup    string `json:"active_group,omitempty"`
	ActiveNode     string `json:"active_node,omitempty"`
	ActivePath     string `json:"active_path,omitempty"`
	AIRoute        string `json:"ai_route,omitempty"`
}

// Rulesets 是 5 个内置规则集的开关组合。键名与 config.RulesetToggles 对应字段一致，
// 只是用 snake_case，便于前端直接展示。
type Rulesets struct {
	ChinaDirect bool `json:"china_direct"`
	Apple       bool `json:"apple"`
	Nintendo    bool `json:"nintendo"`
	Global      bool `json:"global"`
	LANDirect   bool `json:"lan_direct"`
}

// RulesetDescriptor 把每条内置规则集对外暴露成"key + 中文名 + verdict + 样例 +
// 完整规则原文"，让前端可以"展开细则"看到这套规则到底匹配什么、走 DIRECT 还是 Proxy。
// 没有用 `Rulesets` 嵌套是因为它是开关用的；这里是元数据，不参与 PATCH。
type RulesetDescriptor struct {
	Key     string   `json:"key"`     // "china_direct"
	Label   string   `json:"label"`   // "国内域名直连"
	Verdict string   `json:"verdict"` // "DIRECT" / "Proxy" / "REJECT"
	Note    string   `json:"note"`    // 行业术语注脚，例如 "GeoSite CN"
	Count   int      `json:"count"`   // 该集合规则总数
	Sample  []string `json:"sample"`  // 前 8 条规则原文，给折叠预览用
	Rules   []string `json:"rules"`   // 完整规则原文，展开后滚动查看 / 复制
	Enabled bool     `json:"enabled"` // 当前是否启用，回填给前端
}

// CustomRules 是用户自己写的"硬"规则；优先级高于内置规则集。
// 每条字符串就是 mihomo 规则原文（如 "DOMAIN-SUFFIX,corp.example.com"），不带 verdict。
type CustomRules struct {
	Direct []string               `json:"direct"`
	Proxy  []string               `json:"proxy"`
	Reject []string               `json:"reject"`
	Groups []config.TargetedRules `json:"groups,omitempty"`
}

// Connectivity 把"设备接入指引"打成结构化数据，避免前端去解析 ASCII 表格。
// Methods 按场景分行，每行附带需要填写的具体字段。
type Connectivity struct {
	LocalIP   string         `json:"local_ip"`
	Router    string         `json:"router"`
	MixedPort int            `json:"mixed_port"`
	DNSPort   int            `json:"dns_port"`
	Methods   []AccessMethod `json:"methods"`
	Notes     []string       `json:"notes"`
}

// AccessMethod 一行 = 一种设备类型 + 推荐接入方式 + 要填的具体字段。
// 前端按表格 / 卡片渲染都行。
type AccessMethod struct {
	Scenario    string        `json:"scenario"`    // 例: "投影仪 / 电视 / IoT"
	Recommended string        `json:"recommended"` // 例: "改网关 + 改 DNS"
	Fields      []AccessField `json:"fields"`
}

// AccessField 是 "字段名 → 取值" 这样的一对儿，前端按 chip / kv 渲染。
type AccessField struct {
	Label string `json:"label"`          // "网关" / "DNS" / "代理"
	Value string `json:"value"`          // 实际填入的字符串
	Mono  bool   `json:"mono,omitempty"` // true = 等宽显示（IP / 端口）
}

// SourcePayload 是 GET / POST /api/source 用的 source 镜像。
// 跟 config.SourceConfig 字段对齐但只暴露 webui 关心的子集（不含 ScriptPath /
// ChainResidential / Profiles —— 那些 v1 才会用，webui 先不暴露）。
type SourcePayload struct {
	Type         string             `json:"type"`
	Subscription *SubscriptionField `json:"subscription,omitempty"`
	External     *ExternalField     `json:"external,omitempty"`
	File         *FileField         `json:"file,omitempty"`
	Remote       *RemoteField       `json:"remote,omitempty"`
}

type SubscriptionField struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}
type ExternalField struct {
	Name   string `json:"name,omitempty"`
	Server string `json:"server"`
	Port   int    `json:"port"`
	Kind   string `json:"kind"`
}
type FileField struct {
	Path string `json:"path"`
}
type RemoteField struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ScriptPayload 是"增强脚本"卡的整体形态。三种 mode 三选一：
//
//   - "none":   不启用任何脚本
//   - "preset": 用 ChainResidential 预设把订阅的机场节点 + 一个住宅 IP 串成
//     「🛫 AI起飞节点 → 🛬 AI落地节点（dialer-proxy 化）」链路，
//     专治 ChatGPT / Claude 等 AI 站点对机场 IP 的封锁。
//   - "custom": 指向一个用户自己写的 .js（goja 引擎执行），完全自由。
//
// CLI 菜单上同一个入口（[S] 增强脚本）用的就是这套逻辑，本字段把它原样搬到 Web。
type ScriptPayload struct {
	Mode             string                   `json:"mode"` // "none" | "preset" | "custom"
	CustomPath       string                   `json:"custom_path,omitempty"`
	ChainResidential *ChainResidentialPayload `json:"chain_residential,omitempty"`
}

// ChainResidentialPayload 是住宅 IP 链式预设要的全部字段，跟 config.ChainResidentialConfig
// 一一对应。Password 在传输时不掩码（HTTPS 局域网下传输，UI 用 type=password 遮挡）。
type ChainResidentialPayload struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // "http" | "socks5"
	Server      string `json:"server"`
	Port        int    `json:"port"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	DialerProxy string `json:"dialer_proxy"` // 默认 "🛫 AI起飞节点"
}

// routes 注册 /api/* 和静态资源。所有 /api/* 路径都套 requireToken 中间件，
// 静态资源（HTML/CSS/JS）和 /api/ping（探活）不鉴权。
func (s *Server) routes(mux *http.ServeMux, ctrl Controller) {
	// 静态资源：embed 的 static/* 直接挂根。
	mux.Handle("/", http.FileServer(staticFileSystem()))

	// auth 是个 helper：自动给 /api/* 套上 token 校验中间件，少 50 行重复代码。
	auth := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, s.requireToken(h))
	}

	// 健康检查不需要 token（cmd/webui.go probeURL 在用，外加监控）
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	auth("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "use GET")
			return
		}
		snap := ctrl.Snapshot()
		writeJSON(w, http.StatusOK, snap)
	})

	auth("/api/config/gateway", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var req struct {
			Mode string `json:"mode"`
		}
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		mode := strings.TrimSpace(req.Mode)
		if mode != "tun" && mode != "forward" {
			writeErr(w, http.StatusBadRequest, "mode 必须是 tun 或 forward")
			return
		}
		// SetGatewayMode 内部包含 stop+start，可能耗时几秒；让 client 用更长超时。
		if err := ctrl.SetGatewayMode(r.Context(), mode); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/tun", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var req struct {
			Enabled *bool `json:"enabled,omitempty"`
		}
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Enabled == nil {
			writeErr(w, http.StatusBadRequest, "需要 enabled 字段")
			return
		}
		if err := ctrl.SetTUNEnabled(r.Context(), *req.Enabled); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/traffic", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var req struct {
			Mode       *string `json:"mode,omitempty"`
			Adblock    *bool   `json:"adblock,omitempty"`
			AutoGroups *bool   `json:"auto_groups,omitempty"`
		}
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx := r.Context()
		if req.Mode != nil {
			m := strings.TrimSpace(*req.Mode)
			if m != "rule" && m != "global" && m != "direct" {
				writeErr(w, http.StatusBadRequest, "mode 必须是 rule/global/direct")
				return
			}
			if err := ctrl.SetTrafficMode(ctx, m); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if req.Adblock != nil {
			if err := ctrl.SetAdblock(ctx, *req.Adblock); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		if req.AutoGroups != nil {
			if err := ctrl.SetAutoGroups(ctx, *req.AutoGroups); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/rulesets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var rs Rulesets
		if err := decode(r, &rs); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := ctrl.SetRulesets(r.Context(), rs); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeErr(w, http.StatusMethodNotAllowed, "use PUT")
			return
		}
		var rs CustomRules
		if err := decode(r, &rs); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := ctrl.SetCustomRules(r.Context(), rs); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/script", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeErr(w, http.StatusMethodNotAllowed, "use PUT")
			return
		}
		var sp ScriptPayload
		if err := decode(r, &sp); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateScript(sp); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := ctrl.SetScript(r.Context(), sp); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/ports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var p Ports
		if err := decode(r, &p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := ctrl.SetPorts(r.Context(), p); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/proxy-service", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var p ProxyServicePayload
		if err := decode(r, &p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(p.Username) == "" && strings.TrimSpace(p.Password) != "" {
			writeErr(w, http.StatusBadRequest, "设置密码时 username 不能为空")
			return
		}
		if err := ctrl.SetProxyService(r.Context(), p); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/config/dns", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			writeErr(w, http.StatusMethodNotAllowed, "use PATCH")
			return
		}
		var req struct {
			Enabled *bool `json:"enabled,omitempty"`
		}
		if err := decode(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Enabled == nil {
			writeErr(w, http.StatusBadRequest, "需要 enabled 字段")
			return
		}
		if err := ctrl.SetDNSEnabled(r.Context(), *req.Enabled); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/source", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "use POST")
			return
		}
		var p SourcePayload
		if err := decode(r, &p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateSource(p); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := ctrl.SetSource(r.Context(), p); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ctrl.Snapshot())
	})

	auth("/api/control/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "use POST")
			return
		}
		if err := ctrl.Reload(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	auth("/api/control/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "use POST")
			return
		}
		if err := ctrl.Restart(r.Context()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	auth("/api/update", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			info, err := ctrl.CheckUpdate(r.Context())
			if err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, info)
		case http.MethodPost:
			if err := ctrl.RunUpdate(r.Context()); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "use GET or POST")
		}
	})
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func validateScript(s ScriptPayload) error {
	switch s.Mode {
	case "none", "":
		return nil
	case "custom":
		if strings.TrimSpace(s.CustomPath) == "" {
			return errors.New("custom 模式下 custom_path 不能为空")
		}
	case "preset":
		if s.ChainResidential == nil {
			return errors.New("preset 模式下 chain_residential 必填")
		}
		c := s.ChainResidential
		k := strings.ToLower(c.Kind)
		if k != "http" && k != "socks5" {
			return errors.New("chain_residential.kind 必须是 http 或 socks5")
		}
		if strings.TrimSpace(c.Server) == "" || c.Port <= 0 {
			return errors.New("chain_residential 必须填 server 和 port")
		}
		if strings.TrimSpace(c.Name) == "" {
			return errors.New("chain_residential.name 不能为空")
		}
	default:
		return errors.New("script.mode 必须是 none/preset/custom")
	}
	return nil
}

func validateSource(p SourcePayload) error {
	switch p.Type {
	case "none":
		return nil
	case "subscription":
		if p.Subscription == nil || strings.TrimSpace(p.Subscription.URL) == "" {
			return errors.New("subscription.url 不能为空")
		}
	case "external":
		if p.External == nil || p.External.Port <= 0 {
			return errors.New("external.port 必须 > 0")
		}
		k := strings.ToLower(p.External.Kind)
		if k != "http" && k != "socks5" {
			return errors.New("external.kind 必须是 http 或 socks5")
		}
	case "file":
		if p.File == nil || strings.TrimSpace(p.File.Path) == "" {
			return errors.New("file.path 不能为空")
		}
	case "remote":
		if p.Remote == nil || p.Remote.Port <= 0 || strings.TrimSpace(p.Remote.Server) == "" {
			return errors.New("remote 需要填 server 和 port")
		}
	default:
		return errors.New("source.type 必须是 none/subscription/external/file/remote")
	}
	return nil
}
