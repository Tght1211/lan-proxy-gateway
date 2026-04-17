package cmd

import "strings"

var subscriptionHintPrefixes = []struct {
	kind   string
	prefix string
}{
	{kind: "remaining", prefix: "剩余流量："},
	{kind: "remaining", prefix: "剩余流量:"},
	{kind: "expiry", prefix: "套餐到期："},
	{kind: "expiry", prefix: "套餐到期:"},
	{kind: "website", prefix: "最新网址："},
	{kind: "website", prefix: "最新网址:"},
	{kind: "reset", prefix: "距离下次重置剩余："},
	{kind: "reset", prefix: "距离下次重置剩余:"},
	{kind: "reset", prefix: "距离重置剩余："},
	{kind: "reset", prefix: "距离重置剩余:"},
	{kind: "reset", prefix: "下次重置剩余："},
	{kind: "reset", prefix: "下次重置剩余:"},
	{kind: "website", prefix: "官网："},
	{kind: "website", prefix: "官网:"},
	{kind: "website", prefix: "官方网站："},
	{kind: "website", prefix: "官方网站:"},
	{kind: "support", prefix: "客服："},
	{kind: "support", prefix: "客服:"},
	{kind: "telegram", prefix: "Telegram："},
	{kind: "telegram", prefix: "Telegram:"},
	{kind: "telegram", prefix: "TG群："},
	{kind: "telegram", prefix: "TG群:"},
	{kind: "telegram", prefix: "TG频道："},
	{kind: "telegram", prefix: "TG频道:"},
	{kind: "telegram", prefix: "通知频道："},
	{kind: "telegram", prefix: "通知频道:"},
}

func parseSubscriptionHintItem(item string) (kind, value string, ok bool) {
	trimmed := strings.TrimSpace(item)
	if trimmed == "" {
		return "", "", false
	}
	for _, hint := range subscriptionHintPrefixes {
		if strings.HasPrefix(trimmed, hint.prefix) {
			return hint.kind, strings.TrimSpace(strings.TrimPrefix(trimmed, hint.prefix)), true
		}
	}
	return "", "", false
}
