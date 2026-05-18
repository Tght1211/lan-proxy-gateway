# legacy/ — archive of historical implementations

This directory exists for git-history reference only. **Nothing here is
maintained, tested, or supported.** Do not import from `legacy/v1/`, do
not invoke the shell scripts in `legacy/`, and do not link to anything
in this directory from public documentation.

## What's in here

- **`legacy/*.sh`, `legacy/com.lan-proxy-gateway.plist`, `legacy/lib/`,
  `legacy/config/`** — the original shell-script-based implementation
  that predated the Go binary. macOS launchd plist + bash scripts
  (`install.sh`, `start.sh`, `stop.sh`, `status.sh`, `switch.sh`,
  `setup-autostart.sh`) that turned a Mac into a gateway by editing
  system configuration directly.
- **`legacy/v1/`** — the first Go rewrite. It has its own `go.mod`
  (`module gateway`) and is independent from the current top-level
  module (`github.com/tght/lan-proxy-gateway`). Replaced by the
  current `cmd/` + `internal/` tree.

## Why kept

`git log` is the source of truth for project history, but having the
files reachable from the current `HEAD` makes pre-refactor behavior
comparison fast (e.g. when investigating regressions a user reports
against an older release). Tagging and dropping the directory is
acceptable future work; until then, this README marks it as inert.
