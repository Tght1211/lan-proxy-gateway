package aiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

type fakeController struct {
	status   app.Status
	setMode  string
	tunCalls int
	srcSet   *config.SourceConfig
}

func (f *fakeController) Status() app.Status                           { return f.status }
func (f *fakeController) HealthText() string                           { return "健康" }
func (f *fakeController) SetMode(_ context.Context, m string) error    { f.setMode = m; return nil }
func (f *fakeController) ToggleTUN(context.Context) error              { f.tunCalls++; return nil }
func (f *fakeController) ToggleAdblock(context.Context) error          { return nil }
func (f *fakeController) SetGatewayMode(context.Context, string) error { return nil }
func (f *fakeController) SetSource(_ context.Context, s config.SourceConfig) error {
	f.srcSet = &s
	return nil
}
func (f *fakeController) AddRule(context.Context, string, string) error { return nil }
func (f *fakeController) Start(context.Context) error                   { return nil }
func (f *fakeController) Stop() error                                   { return nil }

func TestExecuteReadActionInline(t *testing.T) {
	f := &fakeController{status: app.Status{Mode: "rule", Running: true}}
	ex := NewExecutor(f, func(string) bool { t.Fatal("读操作不应请求确认"); return false })
	res := ex.Execute(context.Background(), Action{Action: "get_status"})
	if res.Err != nil {
		t.Fatalf("get_status 不应报错: %v", res.Err)
	}
	if !strings.Contains(res.Observation, "rule") {
		t.Fatalf("观测应含当前模式: %q", res.Observation)
	}
}

func TestExecuteWriteRequiresConfirm(t *testing.T) {
	f := &fakeController{}
	denied := NewExecutor(f, func(string) bool { return false })
	res := denied.Execute(context.Background(), Action{Action: "set_mode", Mode: "global"})
	if f.setMode != "" {
		t.Fatal("用户拒绝时不应执行")
	}
	if !strings.Contains(res.Observation, "拒绝") {
		t.Fatalf("应回灌用户拒绝: %q", res.Observation)
	}

	f2 := &fakeController{}
	ok := NewExecutor(f2, func(string) bool { return true })
	ok.Execute(context.Background(), Action{Action: "set_mode", Mode: "global"})
	if f2.setMode != "global" {
		t.Fatalf("确认后应执行 set_mode，得到 %q", f2.setMode)
	}
}

func TestToggleTUNIdempotent(t *testing.T) {
	// 当前 TUN 已开，要求 enabled:true → 不应再 toggle
	f := &fakeController{status: app.Status{TUN: true}}
	tru := true
	ex := NewExecutor(f, func(string) bool { return true })
	ex.Execute(context.Background(), Action{Action: "toggle_tun", Enabled: &tru})
	if f.tunCalls != 0 {
		t.Fatalf("目标态已满足不应 toggle，调用了 %d 次", f.tunCalls)
	}
}
