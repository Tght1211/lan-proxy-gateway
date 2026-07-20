// Package procutil holds generic process/port utilities used by the gateway
// daemon: startup port preflight, port-owner lookup, and log tailing.
package procutil

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// PortCheck describes one port the daemon wants to bind at startup.
type PortCheck struct {
	Label string // e.g. "relay", "api", "dns"
	Port  int
	Bind  string // "0.0.0.0" or "127.0.0.1"
}

// PortOwner describes the process currently holding a TCP port.
type PortOwner struct {
	Name string
	PID  int
}

// PortConflict is one entry from a preflight check.
type PortConflict struct {
	Check PortCheck
	Owner *PortOwner // nil if we couldn't identify it
	Err   error      // raw net.Listen error
}

// PortConflictError is returned by CheckPorts so callers can print a helpful
// recovery message naming the occupying process.
type PortConflictError struct {
	Conflicts []PortConflict
}

func (e *PortConflictError) Error() string {
	var b strings.Builder
	b.WriteString("端口冲突：\n")
	for _, c := range e.Conflicts {
		fmt.Fprintf(&b, "  • %s 端口 %d 被占用", c.Check.Label, c.Check.Port)
		if c.Owner != nil {
			fmt.Fprintf(&b, " 占用者: %s (PID %d)", c.Owner.Name, c.Owner.PID)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n解决：在配置文件里调整对应端口，或停止占用端口的进程后重试")
	return b.String()
}

// CheckPorts returns a *PortConflictError describing any port conflicts, or nil.
func CheckPorts(checks []PortCheck) error {
	var conflicts []PortConflict
	for _, c := range checks {
		addr := fmt.Sprintf("%s:%d", c.Bind, c.Port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			conflicts = append(conflicts, PortConflict{
				Check: c,
				Owner: LookupPortOwner(c.Port),
				Err:   err,
			})
			continue
		}
		_ = ln.Close()
	}
	if len(conflicts) == 0 {
		return nil
	}
	return &PortConflictError{Conflicts: conflicts}
}

// LookupPortOwner names the process holding a port using lsof.
func LookupPortOwner(port int) *PortOwner {
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+fmt.Sprint(port), "-sTCP:LISTEN").Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return nil
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return nil
	}
	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil
	}
	return &PortOwner{Name: fields[0], PID: pid}
}

// KillPID sends SIGTERM then SIGKILL as fallback.
func KillPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err == nil {
		// give it a moment to shut down cleanly before we escalate
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			if !PIDAlive(pid) {
				return nil
			}
		}
	}
	return proc.Kill()
}

// TailLog returns the last N lines of a log file, for post-mortem on startup failure.
func TailLog(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	return strings.Join(lines[start:], "\n")
}

// WaitForFile polls a path until it exists or timeout.
func WaitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
