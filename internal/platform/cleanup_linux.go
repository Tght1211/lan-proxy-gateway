//go:build linux

package platform

import (
	"fmt"
	"os/exec"
)

// PostStopCleanup scrubs leftover TUN strict-route ip rules that mihomo
// sometimes leaves behind on Linux when it gets SIGKILL after the
// SIGTERM grace timeout, crashes, or panics.
//
// Why this matters (issue #5): mihomo TUN with strict-route adds high-pref
// `ip rule` entries (typically pref >= 9000) with action `unreachable` to
// force every non-TUN egress to fail, sealing the egress to the TUN device.
// On clean shutdown mihomo removes them. On unclean shutdown they linger,
// which can break Docker port mapping for non-port-preserving DNAT
// (e.g. host:container 2228:2283) — packets to the host port match the
// stale unreachable rule before docker's PREROUTING DNAT runs.
//
// Approach: enumerate `ip rule list` (and v6 too), find rows that look like
// mihomo's signature (pref 9000-9999 + unreachable action) via the portable
// parseLeftoverRulePrefs() helper, delete them by pref. Conservative —
// admin rules typically use pref < 32766 and rarely use unreachable action.
func (linuxPlatform) PostStopCleanup() error {
	var firstErr error
	for _, ipv6 := range []bool{false, true} {
		out, err := listIPRules(ipv6)
		if err != nil {
			continue // ip command missing → nothing to clean
		}
		for _, pref := range parseLeftoverRulePrefs(out) {
			if delErr := deleteIPRule(ipv6, pref); delErr != nil && firstErr == nil {
				firstErr = delErr
			}
		}
	}
	return firstErr
}

func listIPRules(ipv6 bool) (string, error) {
	args := []string{"rule", "list"}
	if ipv6 {
		args = append([]string{"-6"}, args...)
	}
	cmd := exec.Command("ip", args...)
	out, err := cmd.Output()
	return string(out), err
}

func deleteIPRule(ipv6 bool, pref int) error {
	args := []string{"rule", "del", "pref", fmt.Sprintf("%d", pref)}
	if ipv6 {
		args = append([]string{"-6"}, args...)
	}
	if _, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		// Best-effort: rule might have been removed concurrently by mihomo's
		// own cleanup (e.g. mihomo SIGTERM cleanup raced with our scan).
		// We swallow EEXIST/ENOENT-equivalents; the only meaningful error
		// here is "ip command missing" which the caller's loop handles.
		return fmt.Errorf("ip rule del pref %d (ipv6=%v): %w", pref, ipv6, err)
	}
	return nil
}
