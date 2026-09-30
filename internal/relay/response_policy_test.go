package relay

import (
	"testing"
	"time"
)

func TestResponsePolicyBackoff(t *testing.T) {
	s := New(Options{})
	now := time.Now()
	d, ok := s.responsePlan("a.test", now)
	if d != 5*time.Second || !ok {
		t.Fatal(d, ok)
	}
	for _, want := range []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second, 30 * time.Second} {
		s.responseResult("a.test", true, false, now)
		d, ok = s.responsePlan("a.test", now)
		if d != want || ok {
			t.Fatal(d, ok)
		}
		now = now.Add(31 * time.Second)
	}
	s.responseResult("a.test", false, true, now)
	d, _ = s.responsePlan("a.test", now)
	if d != 30*time.Second {
		t.Fatal("lost slow direct budget", d)
	}
	s.responseResult("a.test", true, true, now)
	d, ok = s.responsePlan("a.test", now)
	if d != 5*time.Second || !ok {
		t.Fatal(d, ok)
	}
}

func TestResponsePolicyUsesConfiguredBudgets(t *testing.T) {
	policy := ResponsePolicy{DirectWait: 2 * time.Second, ProxyWait: 3 * time.Second, MaxDirectWait: 8 * time.Second, Cooldown: time.Minute, MemoryTTL: 2 * time.Minute}
	s := New(Options{ResponsePolicy: func() ResponsePolicy { return policy }})
	now := time.Now()
	if d, ok := s.responsePlan("a.test", now); d != 2*time.Second || !ok {
		t.Fatal(d, ok)
	}
	s.responseResult("a.test", true, false, now)
	if d, ok := s.responsePlan("a.test", now.Add(31*time.Second)); d != 4*time.Second || ok {
		t.Fatal(d, ok)
	}
	if d, ok := s.responsePlan("a.test", now.Add(61*time.Second)); d != 4*time.Second || !ok {
		t.Fatal(d, ok)
	}
	s.responseResult("a.test", true, false, now)
	s.responseResult("a.test", true, false, now)
	if d, _ := s.responsePlan("a.test", now); d != 8*time.Second {
		t.Fatal("cap", d)
	}
	if d, ok := s.responsePlan("a.test", now.Add(121*time.Second)); d != 2*time.Second || !ok {
		t.Fatal("expiry", d, ok)
	}
}
