package console

import (
	"bytes"
	"strings"
	"testing"
)

func TestHumanizeMihomoLine_CommonCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // 期望输出里应包含的关键词
	}{
		{
			name: "DIRECT + GeoIP/cn + timeout",
			in:   `time="2026-04-21T01:27:55.642635000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:50931 --> 115.190.130.195:21114 error: connect failed: dial tcp 115.190.130.195:21114: i/o timeout"`,
			want: []string{"🟡", "01:27:55", "直连", "115.190.130.195:21114", "GeoIP=cn", "目标无响应"},
		},
		{
			name: "Proxy + Match + 本机代理拒绝",
			in:   `time="2026-04-21T01:25:53.652976000+08:00" level=warning msg="[UDP] dial Proxy (match Match/) 198.18.0.1:55131 --> 17.248.216.69:443 error: 127.0.0.1:6578 connect error: connect failed: dial tcp 127.0.0.1:6578: connect: connection refused"`,
			want: []string{"🟡", "01:25:53", "走代理", "17.248.216.69:443", "兜底", "本机代理拒绝"},
		},
		{
			name: "Proxy + DomainSuffix + DNS 失败",
			in:   `time="2026-04-21T01:05:41.491676000+08:00" level=warning msg="[UDP] dial Proxy (match DomainSuffix/google.com) 192.168.12.108:49427 --> android.apis.google.com:443 error: can't resolve ip: all DNS requests failed, first error: requesting https://8.8.8.8:443/dns-query: xxx"`,
			want: []string{"🟡", "走代理", "android.apis.google.com:443", "域名=google.com", "域名解析失败"},
		},
		{
			name: "UDP bind 资源不够",
			in:   `time="2026-04-21T01:28:56.056420000+08:00" level=warning msg="[UDP] dial Proxy (match Match/) 198.18.0.1:56030 --> 17.253.150.10:443 error: listen udp :0: bind: resource temporarily unavailable"`,
			want: []string{"17.253.150.10:443", "socket 资源"},
		},
		{
			name: "不认识的行原样返回",
			in:   `this is a plain line with no mihomo structure`,
			want: []string{"this is a plain line"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := humanizeMihomoLine(tc.in)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("expected %q to contain %q\ngot: %s", tc.name, w, got)
				}
			}
		})
	}
}

func TestLineDeduperFoldsRepeats(t *testing.T) {
	// 三条几乎相同的超时行（时间戳不同，dst/reason 相同）+ 一条不同的 → 期望折叠成
	// "第1行 + 摘要 + 第4行"。
	sameA := `time="2026-04-21T02:37:57.000000000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:50931 --> 115.190.130.195:21114 error: connect failed: dial tcp 115.190.130.195:21114: i/o timeout"`
	sameB := `time="2026-04-21T02:38:12.000000000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:50932 --> 115.190.130.195:21114 error: connect failed: dial tcp 115.190.130.195:21114: i/o timeout"`
	sameC := `time="2026-04-21T02:38:27.000000000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:50933 --> 115.190.130.195:21114 error: connect failed: dial tcp 115.190.130.195:21114: i/o timeout"`
	other := `time="2026-04-21T02:44:20.000000000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:60001 --> pantanal-intelli-cn.allawntech.com:443 error: connect failed: dial tcp: i/o timeout"`

	var buf bytes.Buffer
	d := newLineDeduper(&buf)
	for _, ln := range []string{sameA, sameB, sameC, other} {
		formatted, key, t := humanizeMihomoLineWithKey(ln)
		d.Write(formatted, key, t)
	}
	d.Flush()

	out := buf.String()
	// 前 3 条只应打印第 1 条
	if n := strings.Count(out, "115.190.130.195:21114"); n != 1 {
		t.Fatalf("repeat lines should fold to a single printed line; got %d occurrences:\n%s", n, out)
	}
	if !strings.Contains(out, "又重复 2 次") {
		t.Fatalf("expected fold summary with count=2:\n%s", out)
	}
	if !strings.Contains(out, "最近 02:38:27") {
		t.Fatalf("expected latest timestamp in fold summary:\n%s", out)
	}
	// 后面的异型行必须独立出现
	if !strings.Contains(out, "pantanal-intelli-cn.allawntech.com") {
		t.Fatalf("different line should be printed separately:\n%s", out)
	}
}

func TestLineDeduperFlushAtEnd(t *testing.T) {
	// 两条相同行 → 最后 Flush 时应输出摘要（而不是等下一条切换触发）
	ln := `time="2026-04-21T02:37:57.000000000+08:00" level=warning msg="[TCP] dial DIRECT (match GeoIP/cn) 198.18.0.1:1 --> 1.2.3.4:443 error: connect failed: dial tcp 1.2.3.4:443: i/o timeout"`

	var buf bytes.Buffer
	d := newLineDeduper(&buf)
	for i := 0; i < 2; i++ {
		formatted, key, t := humanizeMihomoLineWithKey(ln)
		d.Write(formatted, key, t)
	}
	d.Flush()

	if !strings.Contains(buf.String(), "又重复 1 次") {
		t.Fatalf("Flush should emit summary for tail duplicates:\n%s", buf.String())
	}
}
