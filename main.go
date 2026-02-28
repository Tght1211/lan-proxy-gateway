package main

import "github.com/tght/lan-proxy-gateway/cmd"

// version is set at build time via ldflags.
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
