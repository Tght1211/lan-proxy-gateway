---
name: lan-proxy-gateway-ops
description: Use when operating or configuring the lan-proxy-gateway CLI (the `gateway` binary) non-interactively — setting the proxy source/subscription, switching nodes, toggling TUN/adblock/traffic mode, adding routing rules, reading status, or starting/stopping the LAN gateway from a script or AI agent without driving the interactive TUI.
---

# LAN Proxy Gateway Ops

## Overview

`lan-proxy-gateway` (binary: `gateway`) turns a Mac/Linux/Windows machine into a LAN transparent proxy gateway on top of mihomo. It can be driven **entirely from the command line** — no interactive TUI required. Every `config` write saves `gateway.yaml` and **hot-reloads mihomo if it is running**, so changes apply live.

This skill is the command reference for operating it headlessly.

## Workflow

1. **Read** current state: `gateway status --json` and `gateway config show --json`.
2. **Change** config with `gateway config ...` (or `gateway node ...` at runtime).
3. **Verify** by re-reading status/config.

All read + `config` commands work **without root**. Only `start`/`stop`/`restart` need root.

## Command Reference

### Read state (no root, machine-readable with `--json`)
- `gateway status --json` — running, mode, TUN, adblock, source type, ports
- `gateway config show --json` — full config incl. source url/path/server, custom rules
- `gateway node list --json` — proxy groups, their nodes, and the current pick *(needs the gateway running)*

`--json` output uses stable **snake_case** keys (`running`, `gateway_mode`, `tun`, …). `node`/`config show` errors are printed to stderr with a non-zero exit code (e.g. `网关未运行，先 gateway start`).

### Set the proxy source
- `gateway config source --type subscription --url <URL>`
- `gateway config source --type file --path <clash.yaml>`
- `gateway config source --type external --server 127.0.0.1 --port 7890 --kind http`  *(chain behind a local Clash/Verge)*
- `gateway config source --type remote --server <host> --port <p> --kind socks5 --user <u> --pass <pw>`
- `gateway config source --type none`  *(all direct)*

### Toggle behavior
- `gateway config mode <rule|global|direct>`
- `gateway config tun <on|off>`
- `gateway config adblock <on|off>`
- `gateway config gateway-mode <tun|forward>`  *(restarts mihomo)*

### Custom routing rules
- `gateway config rule add <direct|proxy|reject> <RULE>` — `<RULE>` is any mihomo rule body: `DOMAIN-SUFFIX,openai.com`, `DOMAIN,api.foo.com`, `IP-CIDR,10.0.0.0/8`, `PROCESS-NAME,Cursor`, `GEOIP,CN`, etc.
- `gateway config rule list --json`
- `gateway config rule rm <direct|proxy|reject> <index>` — index comes from `rule list`

### Switch nodes at runtime (needs the gateway running)
- `gateway node list`
- `gateway node switch "<group>" "<node>"` — quote names; groups/nodes contain spaces & emoji

### Lifecycle
- `gateway install` — first-run wizard: downloads mihomo + GeoIP, guides initial setup
- `gateway start` / `gateway stop` / `gateway restart` — **needs root** (TUN, IP forwarding, firewall)
- `gateway service install|uninstall|status` — OS service for auto-start on boot

## Privileges

`start`/`stop`/`restart` change the host network stack (TUN device, IP forwarding, pf/iptables) and need root. `status`, `config *`, and `node *` do **not**. If passwordless `sudo` is available, run `sudo gateway start` directly; otherwise tell the user to run it themselves. Never assume the machine should proxy its own traffic — check `gateway config show` (`tun`, `gateway_mode`) first.

## Common Mistakes

- Driving the interactive TUI (`gateway` with no args) by piping keystrokes — fragile. Use the headless `config`/`node` commands above instead.
- Running `node list/switch` when the gateway is stopped — they need mihomo's running API; start first.
- Forgetting to quote group/node names in `node switch` — they contain spaces and emoji.
- Editing `gateway.yaml` by hand while the gateway runs — prefer `config` commands so the change hot-reloads cleanly.

## Scenarios

For end-to-end recipes (LAN onboarding, point to a subscription then pick a node, local-machine bypass, health/recovery), read [references/scenarios.md](references/scenarios.md).
