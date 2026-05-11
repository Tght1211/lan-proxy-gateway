package platform

import (
	"strconv"
	"strings"
)

// parseLeftoverRulePrefs scans `ip rule list` output for entries that look
// like mihomo TUN strict-route leftovers and returns their pref values.
//
// Pure function — kept in a portable file (no build tag) so the unit tests
// can run on darwin / windows CI without needing iproute2.
//
// Match rule:
//   - line starts with "<pref>:"
//   - pref is in [9000, 9999] (mihomo's typical range; admin pref usually < 32766)
//   - line contains "unreachable" (mihomo strict-route signature)
//
// Example match: `9000:	from all unreachable`
// Example skip:  `0:      from all lookup local`  (admin rule, low pref)
// Example skip:  `9001:   from all lookup main`   (high pref but not unreachable)
func parseLeftoverRulePrefs(output string) []int {
	var prefs []int
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		pref, err := strconv.Atoi(line[:colon])
		if err != nil || pref < 9000 || pref > 9999 {
			continue
		}
		if !strings.Contains(line, "unreachable") {
			continue
		}
		prefs = append(prefs, pref)
	}
	return prefs
}
