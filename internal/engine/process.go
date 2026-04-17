package engine

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

type process struct {
	bin     string
	workdir string
	logPath string
	cmd     *exec.Cmd
	pidFile string
}

func newProcess(bin, workdir, logPath string) *process {
	return &process{
		bin:     bin,
		workdir: workdir,
		logPath: logPath,
		pidFile: workdir + "/mihomo.pid",
	}
}

func (p *process) Start() error {
	// If an old PID exists and is still alive, reuse it rather than double-launch.
	if pid, ok := p.readPID(); ok {
		if pidAlive(pid) {
			// Already running; just record.
			return nil
		}
		_ = os.Remove(p.pidFile)
	}

	log, err := os.OpenFile(p.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}

	cmd := exec.Command(p.bin, "-d", p.workdir)
	cmd.Stdout = log
	cmd.Stderr = log
	configureProcAttrs(cmd)
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return fmt.Errorf("start %s: %w", p.bin, err)
	}
	p.cmd = cmd
	if err := os.WriteFile(p.pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	go func() {
		_ = cmd.Wait()
		_ = log.Close()
	}()
	return nil
}

func (p *process) Stop() error {
	if p.cmd != nil && p.cmd.Process != nil {
		terminateProcess(p.cmd.Process)
		done := make(chan error, 1)
		go func() { done <- p.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = p.cmd.Process.Kill()
		}
	} else if pid, ok := p.readPID(); ok {
		if proc, err := os.FindProcess(pid); err == nil {
			terminateProcess(proc)
			time.Sleep(500 * time.Millisecond)
			_ = proc.Kill()
		}
	}
	_ = os.Remove(p.pidFile)
	return nil
}

func (p *process) Alive() bool {
	if p.cmd != nil && p.cmd.Process != nil {
		if pidAlive(p.cmd.Process.Pid) {
			return true
		}
	}
	pid, ok := p.readPID()
	if !ok {
		return false
	}
	return pidAlive(pid)
}

func (p *process) readPID() (int, bool) {
	data, err := os.ReadFile(p.pidFile)
	if err != nil {
		return 0, false
	}
	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, false
	}
	if pid <= 0 {
		return 0, false
	}
	return pid, true
}
