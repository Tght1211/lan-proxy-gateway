package console

import (
	"fmt"
	"io"
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
	formatted, _, _ := humanizeMihomoLineWithKey(line)
	return formatted
}

// humanizeMihomoLineWithKey 同 humanizeMihomoLine，额外返回去掉时间戳后的 dedup key
// 和提取出来的 HH:MM:SS。dedup key 相同就是"本质上重复的同一条告警"，调用方
// （lineDeduper）据此折叠日志刷屏。
//
// key 设计：
//   - 能解析的 dial 行：proto|route|matchDesc|dst|reason — 这四段决定一条告警的"身份"
//   - 其他可解析行：level|msg —— msg 本身已经去掉时间戳
//   - 无法解析的行：原始 line —— 原样返回不折叠，避免把无结构行误合并
func humanizeMihomoLineWithKey(line string) (formatted, key, timeHMS string) {
	t, level, msg, ok := parseMihomoLine(line)
	if !ok {
		return line, line, ""
	}
	icon := levelIcon(level)

	if m := dialLineRE.FindStringSubmatch(msg); m != nil {
		proto := m[1]
		route := describeRoute(m[2])
		matchDesc := describeMatch(m[3])
		dst := m[5]
		reason := describeReason(m[6])
		formatted = fmt.Sprintf("%s %s  %s %s %s %s  → %s",
			icon, t, proto, route, dst, matchDesc, reason)
		key = proto + "|" + route + "|" + matchDesc + "|" + dst + "|" + reason
		return formatted, key, t
	}

	// 非 dial 行（info / 配置加载 / rule 重载等），只做最基本的时间+等级+原 msg
	formatted = fmt.Sprintf("%s %s  %s", icon, t, msg)
	key = level + "|" + msg
	return formatted, key, t
}

// lineDeduper 把连续重复的同 key 行折叠成计数，避免长时间只有一两个告警在
// 每 15 秒重复的日志把终端刷成流瀑布。
// 约定：
//   - 连续同 key → 只打印第一条，累计计数
//   - key 切换 → 先打印计数摘要（"⋮ 上面那一行又重复 N 次（最近 HH:MM:SS）"）再打新行
//   - 流结束 → 调用 Flush 把尾部未 flush 的计数落盘
//   - rawMode（不 humanize）下不折叠 —— rawMode 的用途就是"原样看"
type lineDeduper struct {
	w        io.Writer
	lastKey  string
	dupCount int
	lastTime string
}

func newLineDeduper(w io.Writer) *lineDeduper { return &lineDeduper{w: w} }

// Write 接收 humanize 后的一行，根据 key 决定是打印还是累计。
// 空 key 视为不可折叠，直接打印并切断前一段折叠。
func (d *lineDeduper) Write(formatted, key, timeHMS string) {
	if key != "" && key == d.lastKey {
		d.dupCount++
		d.lastTime = timeHMS
		return
	}
	d.Flush()
	d.lastKey = key
	d.lastTime = timeHMS
	fmt.Fprintln(d.w, formatted)
}

// Flush 把累计的重复计数（如果有）写成一行摘要并清零。
// 在一次日志渲染结束、切换模式、退出跟随时必须调用。
func (d *lineDeduper) Flush() {
	if d.dupCount > 0 {
		suffix := ""
		if d.lastTime != "" {
			suffix = fmt.Sprintf("（最近 %s）", d.lastTime)
		}
		fmt.Fprintf(d.w, "   ⋮ 上面那一行又重复 %d 次%s\n", d.dupCount, suffix)
	}
	d.dupCount = 0
	d.lastKey = ""
	d.lastTime = ""
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
