package mihomo

import (
	"errors"
	"testing"
)

func TestIsProxyUnreachable(t *testing.T) {
	// Mimic the Go http error shapes we see when a proxy is dead.
	yes := []error{
		errors.New("Get \"https://cdn.jsdelivr.net/...\": proxyconnect tcp: dial tcp 127.0.0.1:7891: connect: connection refused"),
		errors.New("connect: connection refused"),
		errors.New("proxyconnect tcp: EOF"),
		errors.New("Bad Gateway"),
		errors.New("no route to host"),
	}
	for _, e := range yes {
		if !isProxyUnreachable(e) {
			t.Errorf("expected proxy-unreachable for %q", e)
		}
	}
	no := []error{
		nil,
		errors.New("HTTP 404"),
		errors.New("context deadline exceeded"),
		errors.New("EOF"),
	}
	for _, e := range no {
		if isProxyUnreachable(e) {
			t.Errorf("should NOT be proxy-unreachable: %v", e)
		}
	}
}
