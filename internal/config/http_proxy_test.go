package config

import "testing"

func TestHTTPProxyValidation(t *testing.T) {
	for _, edit := range []func(*Config){
		func(c *Config) { c.HTTPProxy.Port = 65536 },
		func(c *Config) { c.HTTPProxy.Auth = "unknown" },
		func(c *Config) { c.HTTPProxy.Auth = "basic" },
		func(c *Config) { c.HTTPProxy.Auth = "basic"; c.HTTPProxy.Username = "u:x"; c.HTTPProxy.Password = "p" },
		func(c *Config) { c.HTTPProxy.Enabled = true; c.HTTPProxy.Port = c.Runtime.APIPort },
	} {
		cfg := Default()
		Normalize(cfg)
		edit(cfg)
		if err := Validate(cfg); err == nil {
			t.Fatal("invalid HTTP proxy config accepted")
		}
	}
	cfg, err := Parse([]byte("version: 4\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPProxy.Enabled || cfg.HTTPProxy.Auth != "none" || cfg.HTTPProxy.Port != 17894 {
		t.Fatalf("unexpected migration defaults: %+v", cfg.HTTPProxy)
	}
}
