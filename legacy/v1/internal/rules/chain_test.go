package rules

import (
	"slices"
	"testing"
)

func TestNormalProbeRulesUseOrdinaryProxyPath(t *testing.T) {
	rules := NormalProbeRules("Proxy")

	for _, want := range []string{
		"DOMAIN,ipwho.is,Proxy",
		"DOMAIN,api.ip.sb,Proxy",
	} {
		if !slices.Contains(rules, want) {
			t.Fatalf("NormalProbeRules() missing %q", want)
		}
	}
}

func TestAIProxyRulesKeepResidentialProbe(t *testing.T) {
	rules := AIProxyRules("AI Only")

	for _, want := range []string{
		"DOMAIN,checkip.amazonaws.com,AI Only",
		"DOMAIN,downloads.cursor.com,AI Only",
		"DOMAIN,anysphere-binaries.s3.us-east-1.amazonaws.com,AI Only",
	} {
		if !slices.Contains(rules, want) {
			t.Fatalf("AIProxyRules() missing %q", want)
		}
	}
}
