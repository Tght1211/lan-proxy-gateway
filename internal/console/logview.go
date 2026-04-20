package console

import (
	"fmt"
	"regexp"
	"strings"
)

// mihomo warning/error 行里最常见的格式：
//   [TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:xxx --> 1.2.3.4:443 error: ...
//   [UDP] dial Proxy  (match Match/)   LAN 源        --> 域名:443 error: ...
// 这个正则抓出 proto、route、match、src、dst、reason。
var dialLineRE = regexp.MustCompile(`^\[(TCP|UDP)\] dial (\S+) \(match ([^)]+)\) (\S+) --> (\S+) error: (.*)$`)

// humanizeMihomoLine 把一行 mihomo 日志翻译成中文简要版。
// 识别不了的行原样返回，保证信息不丢。
func humanizeMihomoLine(line string) string {
	t, level, msg, ok := parseMihomoLine(line)
	if !ok {
		return line
	}
	icon := levelIcon(level)

	if m := dialLineRE.FindStringSubmatch(msg); m != nil {
		proto := m[1]
		route := describeRoute(m[2])
		matchDesc := describeMatch(m[3])
		dst := m[5]
		reason := describeReason(m[6])
		return fmt.Sprintf("%s %s  %s %s %s %s  → %s",
			icon, t, proto, route, dst, matchDesc, reason)
	}

	// 非 dial 行（info / 配置加载 / rule 重载等），只做最基本的时间+等级+原 msg
	return fmt.Sprintf("%s %s  %s", icon, t, msg)
}

// parseMihomoLine 解析 `time="..." level=... msg="..."` 三段。
// 只取时分秒，方便肉眼读。
func parseMihomoLine(line string) (timeHMS, level, msg string, ok bool) {
	ti := strings.Index(line, `time="`)
	li := strings.Index(line, "level=")
	mi := strings.Index(line, `msg="`)
	if ti < 0 || li < 0 || mi < 0 {
		return
	}

	// time="2026-04-21T01:28:56.xxx+08:00" → 01:28:56
	if raw := extractQuoted(line[ti+5:]); raw != "" {
		if tIdx := strings.IndexByte(raw, 'T'); tIdx >= 0 {
			raw = raw[tIdx+1:]
		}
		if dot := strings.IndexByte(raw, '.'); dot > 0 {
			raw = raw[:dot]
		}
		timeHMS = raw
	}

	// level=warning → warning
	{
		s := line[li+6:]
		if sp := strings.IndexAny(s, " \t"); sp > 0 {
			level = s[:sp]
		} else {
			level = s
		}
	}

	// msg="..." 可能含转义引号，找未转义的闭合引号。
	{
		s := line[mi+5:]
		end := len(s)
		for i := 0; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				end = i
				break
			}
		}
		msg = s[:end]
		msg = strings.ReplaceAll(msg, `\"`, `"`)
		msg = strings.ReplaceAll(msg, `\\`, `\`)
	}
	ok = true
	return
}

// extractQuoted 从 `"xxx"...` 里取 xxx。
func extractQuoted(s string) string {
	if !strings.HasPrefix(s, `"`) {
		return ""
	}
	s = s[1:]
	if end := strings.IndexByte(s, '"'); end > 0 {
		return s[:end]
	}
	return ""
}

func levelIcon(level string) string {
	switch level {
	case "error", "fatal", "panic":
		return "🔴"
	case "warning", "warn":
		return "🟡"
	case "info":
		return "ℹ️"
	case "debug":
		return "·"
	default:
		return "·"
	}
}

func describeRoute(r string) string {
	switch r {
	case "DIRECT":
		return "直连"
	case "REJECT", "REJECT-DROP":
		return "拒绝"
	}
	// Proxy / Auto / 具体节点名都归类为「走代理」，后面跟上原名方便对照
	return "走代理[" + r + "]"
}

func describeMatch(m string) string {
	switch {
	case m == "Match/":
		return "（兜底）"
	case strings.HasPrefix(m, "GeoIP/"):
		return fmt.Sprintf("（GeoIP=%s）", strings.TrimPrefix(m, "GeoIP/"))
	case strings.HasPrefix(m, "DomainSuffix/"):
		return fmt.Sprintf("（域名=%s）", strings.TrimPrefix(m, "DomainSuffix/"))
	case strings.HasPrefix(m, "DomainKeyword/"):
		return fmt.Sprintf("（关键字=%s）", strings.TrimPrefix(m, "DomainKeyword/"))
	case strings.HasPrefix(m, "Domain/"):
		return fmt.Sprintf("（精确域=%s）", strings.TrimPrefix(m, "Domain/"))
	case strings.HasPrefix(m, "IPCIDR/"):
		return fmt.Sprintf("（IP 段=%s）", strings.TrimPrefix(m, "IPCIDR/"))
	case strings.HasPrefix(m, "ProcessName/"):
		return fmt.Sprintf("（进程=%s）", strings.TrimPrefix(m, "ProcessName/"))
	}
	return "（" + m + "）"
}

// describeReason 把 mihomo 常见错误翻译成人话。
// 顺序很重要：更具体的 pattern 要放前面。
func describeReason(reason string) string {
	switch {
	case strings.Contains(reason, "127.0.0.1") && strings.Contains(reason, "connection refused"):
		return "本机代理拒绝连接（检查代理软件是否启动）"
	case strings.Contains(reason, "connection refused"):
		return "目标主动拒绝（端口上没服务）"
	case strings.Contains(reason, "i/o timeout"):
		return "目标无响应（超时）"
	case strings.Contains(reason, "network is unreachable"):
		return "目标网络不可达"
	case strings.Contains(reason, "no route to host"):
		return "没有路由到目标"
	case strings.Contains(reason, "can't resolve ip"):
		if strings.Contains(reason, "1.1.1.1") || strings.Contains(reason, "8.8.8.8") {
			return "域名解析失败（fallback DoH 没走代理？）"
		}
		return "域名解析失败（DNS 不通）"
	case strings.Contains(reason, "resource temporarily unavailable"):
		return "本机 socket 资源暂时不足（重启 mihomo 清一下）"
	case strings.Contains(reason, "context deadline exceeded"):
		return "超时"
	case strings.Contains(reason, "EOF"):
		return "连接被对端关闭"
	}
	// 实在不认识，保留原文前 120 字符
	if len(reason) > 120 {
		return reason[:117] + "..."
	}
	return reason
}
