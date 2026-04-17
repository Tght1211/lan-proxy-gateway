package engine

import (
	"net"
	"os/exec"
	"testing"
	"time"
)

func TestCheckPortsReturnsStructuredError(t *testing.T) {
	// Occupy a random port, then CheckPorts against it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	err = CheckPorts([]PortCheck{
		{Label: "test", Port: port, Bind: "127.0.0.1"},
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	pce, ok := err.(*PortConflictError)
	if !ok {
		t.Fatalf("expected *PortConflictError, got %T", err)
	}
	if len(pce.Conflicts) != 1 {
		t.Fatalf("want 1 conflict, got %d", len(pce.Conflicts))
	}
	// HasStaleMihomo should be false — we're sitting on the port, not mihomo.
	if pce.HasStaleMihomo() {
		t.Error("the Go test process is not mihomo")
	}
}

func TestIsMihomo(t *testing.T) {
	cases := map[string]bool{
		"mihomo":       true,
		"Mihomo":       true,
		"mihomo-debug": true,
		"clash":        false,
		"":             false,
	}
	for input, want := range cases {
		if got := isMihomo(input); got != want {
			t.Errorf("isMihomo(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestKillPIDSendsSignal(t *testing.T) {
	// Spawn a short-lived `sleep` we can own and kill.
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Skip("sleep binary unavailable:", err)
	}
	pid := cmd.Process.Pid
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	if err := KillPID(pid); err != nil {
		t.Fatalf("KillPID: %v", err)
	}
	select {
	case <-done:
		// success — process exited
	case <-time.After(2 * time.Second):
		t.Fatal("process did not die within 2s")
	}
}
