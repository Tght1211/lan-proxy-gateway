package config

import (
	_ "embed"
	"encoding/json"
)

//go:embed default_routing.json
var defaultRoutingJSON []byte

// DefaultRoutingRules returns a fresh copy of the bundled ordered presets.
func DefaultRoutingRules() []RoutingRule {
	var rules []RoutingRule
	if err := json.Unmarshal(defaultRoutingJSON, &rules); err != nil {
		panic("invalid bundled routing presets: " + err.Error())
	}
	return rules
}
