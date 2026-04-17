package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// waitForSignal blocks until SIGINT or SIGTERM.
func waitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
