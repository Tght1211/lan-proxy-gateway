package source

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 三个节点 + 一个 select 类型的 Proxy 组。订阅里既没 url-test 也没 fallback。
// autoGroups=true 时应该追加 Auto (url-test) + Fallback (fallback)；
// autoGroups=false 时只补 v3.0 起就有的 Proxy 兜底，不应冒出 Auto/Fallback。
const yamlThreeProxiesOneSelectGroup = `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
  - name: n2
    type: socks5
    server: 2.2.2.2
    port: 443
  - name: n3
    type: socks5
    server: 3.3.3.3
    port: 443
proxy-groups:
  - name: Proxy
    type: select
    proxies: [n1, n2, n3]
rules:
  - MATCH,Proxy
`

func TestInlineUserYAML_AutoGroupsOff(t *testing.T) {
	frag, err := inlineUserYAML([]byte(yamlThreeProxiesOneSelectGroup), false)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	if strings.Contains(frag.YAML, "type: url-test") {
		t.Fatalf("autoGroups=false should NOT add url-test group:\n%s", frag.YAML)
	}
	if strings.Contains(frag.YAML, "type: fallback") {
		t.Fatalf("autoGroups=false should NOT add fallback group:\n%s", frag.YAML)
	}
}

func TestInlineUserYAML_AutoGroupsAppendsBoth(t *testing.T) {
	frag, err := inlineUserYAML([]byte(yamlThreeProxiesOneSelectGroup), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	// 只断言 "type: xxx" 和节点名，因为 yaml.Marshal 对关键字名字会加引号
	for _, want := range []string{"type: url-test", "type: fallback", "n1", "n3"} {
		if !strings.Contains(frag.YAML, want) {
			t.Fatalf("autoGroups=true missing %q in output:\n%s", want, frag.YAML)
		}
	}
	if !strings.Contains(frag.YAML, "name: Auto") && !strings.Contains(frag.YAML, `name: "Auto"`) {
		t.Fatalf("Auto group name missing:\n%s", frag.YAML)
	}
	if !strings.Contains(frag.YAML, "name: Fallback") && !strings.Contains(frag.YAML, `name: "Fallback"`) {
		t.Fatalf("Fallback group name missing:\n%s", frag.YAML)
	}
	// Proxy 组本身仍然只有一个 (没新增重名组)，但 v3.4.1 起它的 proxies
	// 列表会被注入 Auto / Fallback / DIRECT 用作可选项 (尾部追加，不动头部默认)
	if strings.Count(frag.YAML, "name: Proxy") != 1 {
		t.Fatalf("Proxy group should appear exactly once; saw %d:\n%s",
			strings.Count(frag.YAML, "name: Proxy"), frag.YAML)
	}
	proxyMembers := proxyGroupMembers(t, frag.YAML, "Proxy")
	for _, want := range []string{"n1", "n2", "n3", "Auto", "Fallback", "DIRECT"} {
		if !containsString(proxyMembers, want) {
			t.Fatalf("Proxy group should contain %q after auto_groups; got %v\n%s",
				want, proxyMembers, frag.YAML)
		}
	}
	// 用户原有的头三项必须保持原顺序，新项追加在尾部
	if got := proxyMembers[0]; got != "n1" {
		t.Fatalf("Proxy group head should stay n1 (default selection); got %q", got)
	}
}

func TestInlineUserYAML_AutoGroupsSkipsWhenUrlTestExists(t *testing.T) {
	// 订阅已有一个 url-test（叫 "🚀 自动"）但没 fallback —— 应该只加 Fallback
	yamlHasUrlTest := `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
proxy-groups:
  - name: "🚀 自动"
    type: url-test
    proxies: [n1]
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: Proxy
    type: select
    proxies: ["🚀 自动", n1]
`
	frag, err := inlineUserYAML([]byte(yamlHasUrlTest), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	// 不应该再多加一个 url-test
	if strings.Count(frag.YAML, "type: url-test") != 1 {
		t.Fatalf("should not duplicate url-test group; saw %d:\n%s",
			strings.Count(frag.YAML, "type: url-test"), frag.YAML)
	}
	if !strings.Contains(frag.YAML, "type: fallback") {
		t.Fatalf("Fallback group should still be appended:\n%s", frag.YAML)
	}
	// yaml.Marshal 对跟类型关键字同名的字符串会加引号（"Fallback"），所以两种都可能
	if !strings.Contains(frag.YAML, "name: Fallback") && !strings.Contains(frag.YAML, `name: "Fallback"`) {
		t.Fatalf("Fallback group name missing:\n%s", frag.YAML)
	}
}

func TestInlineUserYAML_AutoGroupsSkipsWhenBothExist(t *testing.T) {
	yamlHasBoth := `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
proxy-groups:
  - name: AutoX
    type: url-test
    proxies: [n1]
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: FbX
    type: fallback
    proxies: [n1]
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: Proxy
    type: select
    proxies: [AutoX, FbX, n1]
`
	frag, err := inlineUserYAML([]byte(yamlHasBoth), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	if strings.Count(frag.YAML, "type: url-test") != 1 {
		t.Fatalf("should not add another url-test:\n%s", frag.YAML)
	}
	if strings.Count(frag.YAML, "type: fallback") != 1 {
		t.Fatalf("should not add another fallback:\n%s", frag.YAML)
	}
	// 新增的 "Auto" / "Fallback" 名字都不该出现
	if strings.Contains(frag.YAML, "name: Auto\n") {
		t.Fatalf("no Auto group should be added when url-test exists:\n%s", frag.YAML)
	}
}

func TestInlineUserYAML_AutoGroupsRenameOnNameClash(t *testing.T) {
	// 用户已有一个叫 "Auto" 的 **select** 组（不是 url-test）。此时 autoGroups=true
	// 应该追加 url-test 组，但名字要避冲突改成 "Auto2"。
	yamlAutoIsSelect := `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
proxy-groups:
  - name: Auto
    type: select
    proxies: [n1, DIRECT]
  - name: Proxy
    type: select
    proxies: [Auto, n1]
`
	frag, err := inlineUserYAML([]byte(yamlAutoIsSelect), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	if !strings.Contains(frag.YAML, `name: "Auto2"`) && !strings.Contains(frag.YAML, "name: Auto2") {
		t.Fatalf("name clash should rename to Auto2:\n%s", frag.YAML)
	}
	if !strings.Contains(frag.YAML, "type: url-test") {
		t.Fatalf("url-test group should still be added:\n%s", frag.YAML)
	}
	// 重命名场景下注入的应该是新组的实际名 (Auto2)，而不是原始 baseName Auto。
	// 用户已有的 select 组 Auto 是直选组，注入它没意义 (作用 = 直选 n1/DIRECT)，
	// 但代码逻辑只看 type=url-test/fallback，所以 Auto (select) 不会被注入；Auto2 才会
	members := proxyGroupMembers(t, frag.YAML, "Proxy")
	if !containsString(members, "Auto2") {
		t.Fatalf("Proxy group should contain renamed Auto2; got %v\n%s", members, frag.YAML)
	}
	if !containsString(members, "DIRECT") {
		t.Fatalf("Proxy group should contain DIRECT; got %v\n%s", members, frag.YAML)
	}
}

// auto_groups=true，订阅本来就有 url-test (🚀 自动) + fallback (🛡 兜底)，
// Proxy 组 proxies 里只列了节点 n1。增强后 Proxy 组应该把这两个已存在的组
// 名 + DIRECT 都注入进去，让用户能直接在 Proxy 里选自动组。
func TestInlineUserYAML_AutoGroupsInjectsExistingSmartGroupsIntoProxy(t *testing.T) {
	yamlExistingSmartGroups := `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
  - name: n2
    type: socks5
    server: 2.2.2.2
    port: 443
proxy-groups:
  - name: "🚀 自动"
    type: url-test
    proxies: [n1, n2]
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: "🛡 兜底"
    type: fallback
    proxies: [n1, n2]
    url: http://www.gstatic.com/generate_204
    interval: 300
  - name: Proxy
    type: select
    proxies: [n1, n2]
rules:
  - MATCH,Proxy
`
	frag, err := inlineUserYAML([]byte(yamlExistingSmartGroups), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	members := proxyGroupMembers(t, frag.YAML, "Proxy")
	for _, want := range []string{"n1", "n2", "🚀 自动", "🛡 兜底", "DIRECT"} {
		if !containsString(members, want) {
			t.Fatalf("Proxy group missing %q after augment; got %v\n%s",
				want, members, frag.YAML)
		}
	}
	// 头部仍是用户原本的 n1，不被插队
	if members[0] != "n1" {
		t.Fatalf("Proxy default selection (head) should stay n1; got %q", members[0])
	}
}

// auto_groups=true 但订阅压根没 Proxy 组：inlineUserYAML 会先补一个兜底
// Proxy (select, 包含 fallback 名 + DIRECT)，appendAutoFallbackGroups 再追加
// Auto/Fallback 组，最后 augmentProxyGroupOptions 把 Auto/Fallback 也注入到
// 兜底 Proxy 组里。验证这条链路完整可用。
func TestInlineUserYAML_AutoGroupsAugmentsFallbackProxyGroup(t *testing.T) {
	yamlNoProxyGroup := `proxies:
  - name: n1
    type: socks5
    server: 1.1.1.1
    port: 443
proxy-groups:
  - name: "🚀 节点选择"
    type: select
    proxies: [n1]
`
	frag, err := inlineUserYAML([]byte(yamlNoProxyGroup), true)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	members := proxyGroupMembers(t, frag.YAML, "Proxy")
	for _, want := range []string{"DIRECT", "Auto", "Fallback"} {
		if !containsString(members, want) {
			t.Fatalf("synthesized Proxy group missing %q; got %v\n%s",
				want, members, frag.YAML)
		}
	}
}

// auto_groups=false 时绝对不能改 Proxy 组的 proxies 列表 (零侵入升级)
func TestInlineUserYAML_AutoGroupsOff_ProxyGroupUntouched(t *testing.T) {
	frag, err := inlineUserYAML([]byte(yamlThreeProxiesOneSelectGroup), false)
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	members := proxyGroupMembers(t, frag.YAML, "Proxy")
	want := []string{"n1", "n2", "n3"}
	if len(members) != len(want) {
		t.Fatalf("Proxy group should keep original 3 members; got %v", members)
	}
	for i, w := range want {
		if members[i] != w {
			t.Fatalf("Proxy member [%d] = %q, want %q", i, members[i], w)
		}
	}
}

// proxyGroupMembers 从渲染出的 YAML 里抽指定 name 的组的 proxies 列表。
// 用真正的 yaml.Unmarshal 解析，避免对字符串格式 (引号/缩进/emoji) 做脆弱假设。
func proxyGroupMembers(t *testing.T, fragYAML, groupName string) []string {
	t.Helper()
	var doc struct {
		ProxyGroups []struct {
			Name    string   `yaml:"name"`
			Type    string   `yaml:"type"`
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal([]byte(fragYAML), &doc); err != nil {
		t.Fatalf("parse fragment yaml: %v\n%s", err, fragYAML)
	}
	for _, g := range doc.ProxyGroups {
		if g.Name == groupName {
			return g.Proxies
		}
	}
	t.Fatalf("proxy-group %q not found in:\n%s", groupName, fragYAML)
	return nil
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
