// Package rulesets exposes the embedded rule data as parsed slices.
package rulesets

import (
	_ "embed"
	"strings"
)

//go:embed adblock.txt
var adblockRaw string

//go:embed china_direct.txt
var chinaDirectRaw string

//go:embed apple.txt
var appleRaw string

//go:embed nintendo.txt
var nintendoRaw string

//go:embed global.txt
var globalRaw string

//go:embed lan_direct.txt
var lanDirectRaw string

func parse(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// Adblock returns the REJECT rule body (without trailing ",REJECT").
func Adblock() []string { return parse(adblockRaw) }

// ChinaDirect returns the DIRECT rule body for China-only services.
func ChinaDirect() []string { return parse(chinaDirectRaw) }

// Apple returns the DIRECT rule body for Apple services.
func Apple() []string { return parse(appleRaw) }

// Nintendo returns the Proxy rule body for Nintendo services.
func Nintendo() []string { return parse(nintendoRaw) }

// Global returns the Proxy rule body for global services that typically need a proxy.
func Global() []string { return parse(globalRaw) }

// LANDirect returns the DIRECT rule body for private LAN ranges.
func LANDirect() []string { return parse(lanDirectRaw) }
