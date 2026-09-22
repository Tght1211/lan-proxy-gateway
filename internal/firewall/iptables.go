package firewall

import (
	"strings"
)

// iptablesManager converges iptables to the desired rule set. It is
// platform-neutral (exec seams injected) so the diff logic is testable
// without root or Linux; only newPlatformManager wires it to real commands.
type iptablesManager struct {
	run   func(args ...string) error
	save  func(table string) (string, error)
	run6  func(args ...string) error         // ip6tables
	save6 func(table string) (string, error) // ip6tables-save
}

func (m *iptablesManager) Apply(c Config) (Report, error) {
	nat, filter := renderLinuxRules(c)
	if err := m.syncTable("nat", nat); err != nil {
		return Report{}, err
	}
	if err := m.syncTable("filter", filter); err != nil {
		return Report{}, err
	}
	if m.run6 != nil {
		ipv6Filter := renderLinuxIPv6Rules(c)
		if err := m.syncTable6("filter", ipv6Filter); err != nil {
			return Report{}, err
		}
	}
	return Report{}, nil
}

func (m *iptablesManager) Remove() error {
	for _, table := range []string{"nat", "filter"} {
		if err := m.syncTable(table, nil); err != nil {
			return err
		}
	}
	if m.run6 != nil {
		if err := m.syncTable6("filter", nil); err != nil {
			return err
		}
	}
	return nil
}

// syncTable converges one table: rules tagged ours that aren't desired are
// removed (full-spec -D, never rule numbers), desired ones missing are added.
func (m *iptablesManager) syncTable(table string, desired [][]string) error {
	current, err := m.taggedRules(table)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, rule := range desired {
		want[strings.Join(rule, " ")] = true
	}
	for _, cur := range current {
		if !want[cur] {
			args := append([]string{"-t", table, "-D"}, strings.Fields(cur)...)
			if err := m.run(args...); err != nil {
				return err
			}
		}
	}
	have := map[string]bool{}
	for _, cur := range current {
		have[cur] = true
	}
	for _, rule := range desired {
		key := strings.Join(rule, " ")
		if !have[key] {
			args := append([]string{"-t", table, "-A"}, rule...)
			if err := m.run(args...); err != nil {
				return err
			}
		}
	}
	return nil
}

// taggedRules returns our rules in one table as "-A"-less spec strings,
// discovered via iptables-save and filtered by the comment tag.
func (m *iptablesManager) taggedRules(table string) ([]string, error) {
	return parseTaggedRules(m.save, table)
}

// syncTable6 is the ip6tables equivalent of syncTable.
func (m *iptablesManager) syncTable6(table string, desired [][]string) error {
	current, err := parseTaggedRules(m.save6, table)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, rule := range desired {
		want[strings.Join(rule, " ")] = true
	}
	for _, cur := range current {
		if !want[cur] {
			args := append([]string{"-t", table, "-D"}, strings.Fields(cur)...)
			if err := m.run6(args...); err != nil {
				return err
			}
		}
	}
	have := map[string]bool{}
	for _, cur := range current {
		have[cur] = true
	}
	for _, rule := range desired {
		key := strings.Join(rule, " ")
		if !have[key] {
			args := append([]string{"-t", table, "-A"}, rule...)
			if err := m.run6(args...); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseTaggedRules(save func(string) (string, error), table string) ([]string, error) {
	out, err := save(table)
	if err != nil {
		return nil, err
	}
	var rules []string
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A ") || !strings.Contains(line, CommentTag) {
			continue
		}
		rules = append(rules, strings.TrimPrefix(line, "-A "))
	}
	return rules, nil
}
