package source

import (
	"strings"
	"testing"
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
	// Proxy 组（用户自己的）应该还在、不被动
	if strings.Count(frag.YAML, "name: Proxy") != 1 {
		t.Fatalf("user's Proxy group should stay untouched; saw %d occurrences:\n%s",
			strings.Count(frag.YAML, "name: Proxy"), frag.YAML)
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
}
