package aiagent

import "testing"

func TestParseActionFromFencedBlock(t *testing.T) {
	reply := "好的，我来帮你设置订阅源。\n\n```gateway-action\n" +
		`{"action":"set_source","type":"subscription","url":"https://e.com/s"}` +
		"\n```\n确认后即可生效。"
	act, ok := ParseAction(reply)
	if !ok {
		t.Fatal("应解析出动作")
	}
	if act.Action != "set_source" || act.Type != "subscription" || act.URL != "https://e.com/s" {
		t.Fatalf("动作字段不对: %+v", act)
	}
}

func TestParseActionNoBlock(t *testing.T) {
	if _, ok := ParseAction("纯聊天没有动作块"); ok {
		t.Fatal("无动作块应返回 false")
	}
}

func TestParseActionMalformed(t *testing.T) {
	reply := "```gateway-action\n{不是合法json}\n```"
	if _, ok := ParseAction(reply); ok {
		t.Fatal("非法 JSON 应返回 false")
	}
}

func TestActionIsWrite(t *testing.T) {
	if (Action{Action: "get_status"}).IsWrite() {
		t.Fatal("get_status 应是读")
	}
	if !(Action{Action: "set_mode"}).IsWrite() {
		t.Fatal("set_mode 应是写")
	}
}
