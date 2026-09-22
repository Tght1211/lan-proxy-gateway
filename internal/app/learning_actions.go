package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/tght/lan-proxy-gateway/internal/config"
	"io"
	"net/http"
	"sort"
	"strings"
)

func (l *fallbackLearner) Ignored() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	hosts := make([]string, 0, len(l.ignored))
	for host := range l.ignored {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}
func (a *App) applyLearningAction(l *fallbackLearner, action, host string) error {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return fmt.Errorf("缺少域名")
	}
	switch action {
	case "accept":
		ready := false
		for _, candidate := range l.Snapshot() {
			if candidate.Host == host && candidate.Count >= fallbackLearnThreshold {
				ready = true
			}
		}
		if !ready {
			return fmt.Errorf("证据不足或建议已过期，请刷新")
		}
		added, err := a.PromoteLearnedDirectRule(host)
		if err != nil {
			return err
		}
		if !added {
			return fmt.Errorf("已有规则覆盖该域名，请在规则编辑器中处理")
		}
		l.mu.Lock()
		delete(l.counts, host)
		l.saveLocked()
		l.mu.Unlock()
	case "ignore", "restore":
		l.mu.Lock()
		defer l.mu.Unlock()
		if action == "ignore" {
			l.ignored[host] = true
			delete(l.counts, host)
		} else {
			delete(l.ignored, host)
		}
		l.saveLocked()
	case "undo":
		a.cfgMu.Lock()
		next := *a.Cfg
		next.Routing.Rules = make([]config.RoutingRule, 0, len(a.Cfg.Routing.Rules))
		removed := false
		for _, rule := range a.Cfg.Routing.Rules {
			if rule.Learned && rule.Value == host {
				removed = true
				continue
			}
			next.Routing.Rules = append(next.Routing.Rules, rule)
		}
		if !removed {
			a.cfgMu.Unlock()
			return fmt.Errorf("该学习规则已不存在")
		}
		if err := config.Save(&next, a.Paths.ConfigFile); err != nil {
			a.cfgMu.Unlock()
			return err
		}
		a.Cfg = &next
		a.cfgMu.Unlock()
	default:
		return fmt.Errorf("未知操作")
	}
	return nil
}

func (c *APIClient) Learning(ctx context.Context, action, host string) error {
	data, _ := json.Marshal(map[string]string{"action": action, "host": host})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/learning", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("规则操作失败: %s", body)
	}
	return nil
}
