package platform

// Tests for the portable parseLeftoverRulePrefs helper. No build tag —
// runs on darwin / windows CI too since the parser is pure Go (lives in
// cleanup_parser.go, not cleanup_linux.go).

import (
	"reflect"
	"testing"
)

// 真实采到的 mihomo TUN strict-route 残留场景，用户在 issue #5 里类似的输出：
//   $ ip rule list
//   0:      from all lookup local
//   9000:   from all unreachable
//   9001:   from all lookup main suppress_prefixlength 0
//   32766:  from all lookup main
//   32767:  from all lookup default
const ipRuleSampleWithMihomoLeftover = `0:	from all lookup local
9000:	from all unreachable
9001:	from all lookup main suppress_prefixlength 0
32766:	from all lookup main
32767:	from all lookup default
`

// 干净系统（gateway 没跑过/已经清理干净）的标准输出：高 pref 没有 unreachable 项，
// parser 必须返回空 slice，不能误删任何管理员手动加过的策略路由。
const ipRuleCleanSystem = `0:	from all lookup local
32766:	from all lookup main
32767:	from all lookup default
`

// 管理员加的 9500 pref 普通规则（lookup another_table，不是 unreachable）。
// parser 不应该误删这种合法的高 pref 规则 —— 只针对 mihomo 的 unreachable 签名。
const ipRuleAdminHighPref = `0:	from all lookup local
9500:	from 10.0.0.0/8 lookup admin_isolation
32766:	from all lookup main
`

func TestParseLeftoverRulePrefs_DetectsMihomoSignature(t *testing.T) {
	got := parseLeftoverRulePrefs(ipRuleSampleWithMihomoLeftover)
	want := []int{9000}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected only pref 9000 (unreachable signature), got %v", got)
	}
}

func TestParseLeftoverRulePrefs_CleanSystemReturnsEmpty(t *testing.T) {
	got := parseLeftoverRulePrefs(ipRuleCleanSystem)
	if len(got) != 0 {
		t.Fatalf("clean system must yield no leftover prefs; got %v", got)
	}
}

func TestParseLeftoverRulePrefs_DoesNotTouchAdminRules(t *testing.T) {
	// 9500 是高 pref 但不是 unreachable —— 必须保留
	got := parseLeftoverRulePrefs(ipRuleAdminHighPref)
	if len(got) != 0 {
		t.Fatalf("admin's high-pref non-unreachable rule must be preserved; got %v", got)
	}
}

func TestParseLeftoverRulePrefs_LowPrefUnreachableIgnored(t *testing.T) {
	// 极端情况：管理员把 unreachable 用在了低 pref（比如 5000）—— 不在 mihomo
	// 范围 [9000,9999]，不该删。这条断言是向上的安全栏：宁可不清也别误清。
	input := `0:	from all lookup local
5000:	from 10.0.0.0/8 unreachable
32766:	from all lookup main
`
	got := parseLeftoverRulePrefs(input)
	if len(got) != 0 {
		t.Fatalf("pref outside [9000,9999] must be ignored even if unreachable; got %v", got)
	}
}

func TestParseLeftoverRulePrefs_HighPrefAboveRangeIgnored(t *testing.T) {
	// pref 10000+ 也不动 —— mihomo 实际只用 9000 一档，给点缓冲到 9999。
	input := `0:	from all lookup local
10500:	from all unreachable
32766:	from all lookup main
`
	got := parseLeftoverRulePrefs(input)
	if len(got) != 0 {
		t.Fatalf("pref >= 10000 must be ignored; got %v", got)
	}
}

func TestParseLeftoverRulePrefs_MultipleMihomoLeftoversAllReturned(t *testing.T) {
	// 罕见但理论可能：mihomo 在不同 pref 上都加了 unreachable（旧版本 / 多
	// instance / 自定义配置）。parser 应该全报上来给清理。
	input := `0:	from all lookup local
9000:	from all unreachable
9001:	from all unreachable
9002:	from all unreachable
32766:	from all lookup main
`
	got := parseLeftoverRulePrefs(input)
	want := []int{9000, 9001, 9002}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestParseLeftoverRulePrefs_HandlesEmptyAndJunk(t *testing.T) {
	cases := []string{
		"",
		"\n\n",
		"junk\nmore junk\n",
		"no colons here at all",
	}
	for _, in := range cases {
		got := parseLeftoverRulePrefs(in)
		if len(got) != 0 {
			t.Errorf("input %q should yield empty prefs; got %v", in, got)
		}
	}
}
