package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// forwarder resolves queries by racing all configured upstreams and taking
// the first successful answer. UDP first, TCP retry on truncation.
type forwarder struct {
	mu       sync.RWMutex
	upstream []string
	timeout  time.Duration
}

func newForwarder(upstreams []string) *forwarder {
	f := &forwarder{timeout: 3 * time.Second}
	f.Set(upstreams)
	return f
}

// Set replaces the upstream list (normalized to host:port).
func (f *forwarder) Set(upstreams []string) {
	var norm []string
	for _, u := range upstreams {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(u); err != nil {
			u = net.JoinHostPort(u, "53")
		}
		norm = append(norm, u)
	}
	f.mu.Lock()
	f.upstream = norm
	f.mu.Unlock()
}

func (f *forwarder) list() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]string{}, f.upstream...)
}

type raceResult struct {
	msg *dns.Msg
	err error
}

// exchange races the query against all upstreams; first RcodeSuccess wins.
func (f *forwarder) exchange(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	upstreams := f.list()
	if len(upstreams) == 0 {
		return nil, fmt.Errorf("未配置 DNS 上游")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan raceResult, len(upstreams))
	for _, up := range upstreams {
		go func(server string) {
			msg, err := f.queryOne(ctx, server, req)
			results <- raceResult{msg, err}
		}(up)
	}
	var lastErr error
	for range upstreams {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-results:
			if r.err == nil && r.msg != nil && r.msg.Rcode == dns.RcodeSuccess {
				cancel()
				return r.msg, nil
			}
			if r.err != nil {
				lastErr = r.err
			} else if r.msg != nil && lastErr == nil {
				lastErr = fmt.Errorf("上游返回 %s", dns.RcodeToString[r.msg.Rcode])
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("所有 DNS 上游均无可用应答")
	}
	return nil, lastErr
}

func (f *forwarder) queryOne(ctx context.Context, server string, req *dns.Msg) (*dns.Msg, error) {
	c := &dns.Client{Net: "udp", Timeout: f.timeout}
	msg, _, err := c.ExchangeContext(ctx, req, server)
	if err != nil {
		return nil, err
	}
	if msg.Truncated {
		c.Net = "tcp"
		msg, _, err = c.ExchangeContext(ctx, req, server)
	}
	return msg, err
}
