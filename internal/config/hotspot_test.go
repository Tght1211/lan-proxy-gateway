package config

import "testing"

func TestHotspotConfigRequiresIndependentDNS(t *testing.T) {
	c := Default()
	Normalize(c)
	c.Gateway.AccessMode = "hotspot"
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	c.DNS.Port = 53
	if Validate(c) == nil {
		t.Fatal("port 53 competes with Internet Sharing DNS")
	}
	c.DNS.Port = 1053
	c.DNS.Enabled = false
	if Validate(c) == nil {
		t.Fatal("hotspot requires DNS")
	}
	c.DNS.Enabled = true
	c.Gateway.AccessMode = "typo"
	if Validate(c) == nil {
		t.Fatal("unknown mode must not capture default LAN")
	}
}
