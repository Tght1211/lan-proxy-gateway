package main

import "github.com/tght/lan-proxy-gateway/cmd"

// version is injected via -ldflags at build time.
var version = "dev"

func main() {
	cmd.Version = version
	cmd.Execute()
}
