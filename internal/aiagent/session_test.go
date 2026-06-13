package aiagent

import (
	"context"
	"testing"

	"github.com/tght/lan-proxy-gateway/internal/app"
)

// scriptClient 按预设依次返回回复，忽略输入。
type scriptClient struct {
	replies []string
	i       int
}

func (s *scriptClient) Chat(_ context.Context, _ []Message, onDelta func(string)) (string, error) {
	r := s.replies[s.i]
	s.i++
	if onDelta != nil {
		onDelta(r)
	}
	return r, nil
}

func TestSessionRunsActionThenFinishes(t *testing.T) {
	f := &fakeController{status: app.Status{Mode: "rule"}}
	llm := &scriptClient{replies: []string{
		"我先看下状态。\n```gateway-action\n{\"action\":\"get_status\"}\n```",
		"状态正常。\n```gateway-action\n{\"action\":\"finish\",\"summary\":\"已确认状态\"}\n```",
	}}
	ex := NewExecutor(f, func(string) bool { return true })
	sess := NewSession(llm, ex)
	out, err := sess.Handle(context.Background(), "看下状态", nil)
	if err != nil {
		t.Fatalf("Handle 报错: %v", err)
	}
	if out == "" {
		t.Fatal("应有最终回复")
	}
	if llm.i != 2 {
		t.Fatalf("应走两轮（动作 + finish），走了 %d", llm.i)
	}
}

func TestSessionStopsAtMaxTurns(t *testing.T) {
	f := &fakeController{}
	// 永远返回同一个读动作，永不 finish → 应被 maxTurns 截断
	llm := &scriptClient{}
	for i := 0; i < 20; i++ {
		llm.replies = append(llm.replies, "```gateway-action\n{\"action\":\"get_status\"}\n```")
	}
	ex := NewExecutor(f, func(string) bool { return true })
	sess := NewSession(llm, ex)
	_, err := sess.Handle(context.Background(), "循环", nil)
	if err == nil {
		t.Fatal("超过 maxTurns 应报错")
	}
}
