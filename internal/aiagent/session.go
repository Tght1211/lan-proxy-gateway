package aiagent

import (
	"context"
	"errors"
)

const maxTurns = 8

// Session 是一次「用户说一句话 → agent 多轮执行 → 给最终答复」的对话。
type Session struct {
	llm  LLMClient
	exec *Executor
	hist []Message
}

func NewSession(llm LLMClient, exec *Executor) *Session {
	return &Session{llm: llm, exec: exec}
}

// Handle 处理一句用户输入，跑完动作循环，返回 agent 的最终自然语言回复。
// onDelta 把流式文本增量回调给 UI（可 nil）。
func (s *Session) Handle(ctx context.Context, userInput string, onDelta func(string)) (string, error) {
	if len(s.hist) == 0 {
		s.hist = append(s.hist, Message{Role: "system", Content: systemPrompt(s.exec.ctrl)})
	}
	s.hist = append(s.hist, Message{Role: "user", Content: userInput})

	var lastReply string
	for turn := 0; turn < maxTurns; turn++ {
		reply, err := s.llm.Chat(ctx, s.hist, onDelta)
		if err != nil {
			return "", err
		}
		s.hist = append(s.hist, Message{Role: "assistant", Content: reply})
		lastReply = reply

		act, ok := ParseAction(reply)
		if !ok {
			return lastReply, nil // 没有动作 = 纯聊天回复，结束
		}
		res := s.exec.Execute(ctx, act)
		if res.Done {
			return lastReply, nil
		}
		// 把执行观测回灌为 user 消息，继续下一轮
		s.hist = append(s.hist, Message{Role: "user", Content: "[执行结果] " + res.Observation})
	}
	return lastReply, errors.New("AI 助手步骤过多已中止，请用菜单手动操作或换个说法")
}
