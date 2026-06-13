package script

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestApply_ScriptTimeout 验证死循环脚本被超时中断而不是挂死渲染管线。
func TestApply_ScriptTimeout(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "loop.js")
	_ = os.WriteFile(sp, []byte(`function main(c){ while(true){} return c }`), 0o644)

	done := make(chan error, 1)
	go func() {
		_, err := applyWithTimeout(sp, []byte("proxies: []"), 200*time.Millisecond)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("死循环脚本应返回超时错误")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("超时看门狗没生效，脚本把渲染挂死了")
	}
}

// TestApply_ScriptThrows 验证脚本主动抛错被转成普通 error，不杀进程。
func TestApply_ScriptThrows(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "throw.js")
	_ = os.WriteFile(sp, []byte(`function main(c){ throw new Error("boom") }`), 0o644)
	_, err := Apply(sp, []byte("proxies: []"))
	if err == nil || !strings.Contains(err.Error(), "main()") {
		t.Fatalf("期望 main() 报错，got %v", err)
	}
}

// TestApply_RecoversFromHostPanic 验证 panic 被隔离成 error。
// 用一个会让 stringify 拿到无法 JSON 序列化值（循环引用）的脚本触发异常路径，
// 确认不会 panic 出整个进程。
func TestApply_CircularReferenceNoCrash(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "circ.js")
	_ = os.WriteFile(sp, []byte(`function main(c){ var o={}; o.self=o; return o }`), 0o644)
	// 不应 panic 出测试进程；返回 error 即可（循环引用 JSON.stringify 会抛错）。
	_, err := Apply(sp, []byte("proxies: []"))
	if err == nil {
		t.Fatal("循环引用应返回错误而非成功")
	}
}

func TestApply_InjectsProxyGroup(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "enhance.js")
	if err := os.WriteFile(scriptPath, []byte(`
function main(config) {
  config["proxy-groups"] = config["proxy-groups"] || [];
  config["proxy-groups"].unshift({
    name: "AI",
    type: "select",
    proxies: ["DIRECT"]
  });
  config.rules = (config.rules || []);
  config.rules.unshift("DOMAIN-SUFFIX,openai.com,AI");
  return config;
}
`), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	input := []byte(`
mode: rule
proxies: []
proxy-groups:
  - name: Proxy
    type: select
    proxies: [DIRECT]
rules:
  - MATCH,Proxy
`)
	out, err := Apply(scriptPath, input)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "name: AI") {
		t.Errorf("期望注入 AI 组，实际输出:\n%s", s)
	}
	if !strings.Contains(s, "DOMAIN-SUFFIX,openai.com,AI") {
		t.Errorf("期望注入优先规则，实际输出:\n%s", s)
	}
	// 原有结构保持
	if !strings.Contains(s, "name: Proxy") {
		t.Errorf("原 Proxy 组丢了:\n%s", s)
	}
}

func TestApply_MissingMainFn(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "bad.js")
	_ = os.WriteFile(sp, []byte(`function notMain(c){return c}`), 0o644)
	_, err := Apply(sp, []byte("proxies: []"))
	if err == nil || !strings.Contains(err.Error(), "main") {
		t.Errorf("期望找不到 main 的错误, got %v", err)
	}
}
