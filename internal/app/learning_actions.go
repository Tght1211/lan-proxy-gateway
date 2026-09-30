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
	if action == "configure" {
		var settings LearningSettings
		if err := json.Unmarshal([]byte(host), &settings); err != nil {
			return fmt.Errorf("无效的学习设置")
		}
		if settings.Confirmations < 1 || settings.Confirmations > 10 {
			return fmt.Errorf("有效响应次数须为 1 至 10")
		}
		settings = settings.withResponseDefaults()
		if settings.DirectWaitSeconds < 1 || settings.DirectWaitSeconds > 30 || settings.ProxyWaitSeconds < 1 || settings.ProxyWaitSeconds > 30 || settings.MaxDirectWaitSeconds < settings.DirectWaitSeconds || settings.MaxDirectWaitSeconds > 60 || settings.CooldownSeconds < 5 || settings.CooldownSeconds > 600 || settings.MemoryMinutes < 1 || settings.MemoryMinutes > 60 {
			return fmt.Errorf("等待须为 1 至 30 秒，上限须不低于首次等待且不超过 60 秒；冷却为 5 至 600 秒，临时记录为 1 至 60 分钟")
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		previous := l.settings
		l.settings = settings
		if err := l.saveLocked(); err != nil {
			l.settings = previous
			return err
		}
		return nil
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return fmt.Errorf("缺少域名")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	switch action {
	case "accept":
		ready := !l.ignored[host] && len(pruneLearnTimes(l.counts[host], l.now().Add(-fallbackLearnWindow))) >= l.settings.Confirmations
		if !ready {
			return fmt.Errorf("证据不足或建议已过期，请刷新")
		}
		added, err := a.PromoteLearnedProxyRule(host)
		if err != nil {
			return err
		}
		if !added {
			return fmt.Errorf("已有规则覆盖该域名，请在规则编辑器中处理")
		}
		delete(l.counts, host)
		l.saveLocked()
	case "ignore", "restore":
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
		l.ignored[host] = true
		delete(l.counts, host)
		l.saveLocked()
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
	resp, err := c.do(req)
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

// Serialize automatic promotion with ignore/undo so a late response cannot
// immediately re-add a rule the user has just removed.
func (a *App) learnProxyResponse(l *fallbackLearner, host string) (bool, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	l.Record(host)
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.settings.Enabled || l.ignored[host] || !l.settings.AutoSave || len(pruneLearnTimes(l.counts[host], l.now().Add(-fallbackLearnWindow))) < l.settings.Confirmations {
		return false, nil
	}
	added, err := a.PromoteLearnedProxyRule(host)
	if err == nil {
		delete(l.counts, host)
		l.saveLocked()
	}
	return added, err
}
