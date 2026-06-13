package aiagent

import (
	"context"
	"fmt"

	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Result 是一个动作执行后的回灌内容。Observation 会作为下一轮 user 消息喂回 agent。
type Result struct {
	Observation string
	Err         error
	Done        bool // finish 动作 → 结束本轮
}

// ConfirmFunc 渲染计划卡并返回用户是否批准。
type ConfirmFunc func(plan string) bool

// Executor 执行动作：读 inline，写先 Confirm。
type Executor struct {
	ctrl    Controller
	confirm ConfirmFunc
}

func NewExecutor(ctrl Controller, confirm ConfirmFunc) *Executor {
	return &Executor{ctrl: ctrl, confirm: confirm}
}

func (e *Executor) Execute(ctx context.Context, a Action) Result {
	if a.Action == "finish" {
		return Result{Observation: a.Summary, Done: true}
	}
	// 读操作：inline
	switch a.Action {
	case "get_status":
		s := e.ctrl.Status()
		return Result{Observation: fmt.Sprintf(
			"当前状态: 运行=%v 模式=%s TUN=%v 去广告=%v 网关模式=%s 源=%s",
			s.Running, s.Mode, s.TUN, s.Adblock, s.GatewayMode, s.Source)}
	case "get_health":
		return Result{Observation: e.ctrl.HealthText()}
	}
	// 写操作：确认式
	if a.IsWrite() {
		plan := e.planCard(a)
		if !e.confirm(plan) {
			return Result{Observation: "用户拒绝了这个操作，请换个方案或询问用户。"}
		}
	}
	return e.runWrite(ctx, a)
}

func (e *Executor) runWrite(ctx context.Context, a Action) Result {
	var err error
	switch a.Action {
	case "set_mode":
		err = e.ctrl.SetMode(ctx, a.Mode)
	case "set_gateway_mode":
		err = e.ctrl.SetGatewayMode(ctx, a.Mode)
	case "toggle_tun":
		if a.Enabled == nil || *a.Enabled != e.ctrl.Status().TUN {
			err = e.ctrl.ToggleTUN(ctx)
		}
	case "toggle_adblock":
		if a.Enabled == nil || *a.Enabled != e.ctrl.Status().Adblock {
			err = e.ctrl.ToggleAdblock(ctx)
		}
	case "set_source":
		err = e.ctrl.SetSource(ctx, e.sourceFromAction(a))
	case "add_rule":
		err = e.ctrl.AddRule(ctx, a.Verdict, a.Rule)
	case "start":
		err = e.ctrl.Start(ctx)
	case "restart":
		// App.Start 在网关已运行时是 no-op，故 restart 必须先 Stop 再 Start，
		// 否则会静默不重启却回灌「执行成功」误导用户/agent。
		if err = e.ctrl.Stop(); err == nil {
			err = e.ctrl.Start(ctx)
		}
	case "stop":
		err = e.ctrl.Stop()
	default:
		return Result{Observation: fmt.Sprintf("未知动作 %q，请只用约定的动作。", a.Action),
			Err: fmt.Errorf("unknown action %q", a.Action)}
	}
	if err != nil {
		return Result{Observation: "执行失败: " + err.Error(), Err: err}
	}
	return Result{Observation: "执行成功。"}
}

func (e *Executor) sourceFromAction(a Action) config.SourceConfig {
	src := config.SourceConfig{Type: a.Type}
	switch a.Type {
	case "subscription":
		src.Subscription = config.SubscriptionSource{URL: a.URL, Name: "subscription"}
	case "file":
		src.File = config.FileSource{Path: a.Path}
	case "external":
		src.External = config.ExternalProxy{Server: a.Server, Port: a.Port, Kind: a.Kind, Name: "外部代理"}
	case "remote":
		src.Remote = config.RemoteProxy{Server: a.Server, Port: a.Port, Kind: a.Kind, Name: "远程代理"}
	}
	return src
}

func (e *Executor) planCard(a Action) string {
	switch a.Action {
	case "set_source":
		return fmt.Sprintf("设置代理源 → type=%s url=%s path=%s server=%s:%d", a.Type, a.URL, a.Path, a.Server, a.Port)
	case "set_mode":
		return "切换分流模式 → " + a.Mode
	case "set_gateway_mode":
		return "切换网关模式 → " + a.Mode
	case "toggle_tun":
		return fmt.Sprintf("设置 TUN → %v", a.Enabled)
	case "toggle_adblock":
		return fmt.Sprintf("设置去广告 → %v", a.Enabled)
	case "add_rule":
		return fmt.Sprintf("新增规则 → [%s] %s", a.Verdict, a.Rule)
	case "start", "restart":
		return "启动/重启网关"
	case "stop":
		return "停止网关"
	}
	return a.Action
}
