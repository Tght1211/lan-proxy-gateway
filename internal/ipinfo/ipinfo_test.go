package ipinfo

import (
	"testing"
	"unicode/utf8"
)

func TestISPStripAS(t *testing.T) {
	cases := map[string]string{
		"AS13335 Cloudflare, Inc.": "Cloudflare, Inc.",
		"AS7922 Comcast":           "Comcast",
		"Cloudflare":               "Cloudflare",
		"":                         "",
	}
	for in, want := range cases {
		info := &Info{Org: in}
		if got := info.ISP(); got != want {
			t.Errorf("ISP(%q)=%q want %q", in, got, want)
		}
	}
}

func TestISPTruncate(t *testing.T) {
	info := &Info{Org: "AS1 Very Very Very Very Very Long ISP Name Inc."}
	got := info.ISP()
	if n := utf8.RuneCountInString(got); n > 28 {
		t.Errorf("ISP() = %q, too long (%d runes)", got, n)
	}
}
