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
