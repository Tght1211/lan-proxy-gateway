package geoip

import (
	"net"
	"testing"
)

func TestFlagFor(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"HK", "🇭🇰"},
		{"us", "🇺🇸"}, // lowercase ok
		{"CN", "🇨🇳"},
		{"", ""},
		{"U", ""},    // 长度不对
		{"USA", ""},  // 长度不对
		{"12", ""},   // 非字母
	}
	for _, c := range cases {
		got := FlagFor(c.code)
		if got != c.want {
			t.Errorf("FlagFor(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

func TestLookupPrivateIP(t *testing.T) {
	var db *DB // 零值
	tests := []string{"192.168.1.1", "10.0.0.1", "127.0.0.1", "::1"}
	for _, s := range tests {
		_, flag := db.Lookup(net.ParseIP(s))
		if flag != "🏠" {
			t.Errorf("%s want 🏠 got %q", s, flag)
		}
	}
}

func TestLookupNilDB(t *testing.T) {
	var db *DB
	// 非私有 IP + 零值 DB → 空
	c, f := db.LookupString("8.8.8.8")
	if c != "" || f != "" {
		t.Errorf("nil DB expected empty, got %q/%q", c, f)
	}
}

func TestLookupZeroValueDB(t *testing.T) {
	db := &DB{} // r == nil
	c, f := db.LookupString("8.8.8.8")
	if c != "" || f != "" {
		t.Errorf("zero DB expected empty, got %q/%q", c, f)
	}
	if err := db.Close(); err != nil {
		t.Errorf("Close on zero DB: %v", err)
	}
}
