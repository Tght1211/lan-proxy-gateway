# Command Reference

[Chinese version](../commands.md)

## Common Starting Commands

| Command | Purpose | Requires admin |
|---|---|:---:|
| `gateway install` | Initial setup wizard: download `mihomo`, enter subscription info, generate config | No |
| `gateway config` | Interactive config center: proxy source, LAN sharing, rules, extensions | No |
| `gateway config show` | Show the current config summary | No |
| `sudo gateway start` | Start the gateway and enter the menu-driven CLI console | Yes |
| `sudo gateway console` | Re-enter the menu-driven CLI console without restarting the gateway | Yes |
| `sudo gateway stop` | Stop the gateway | Yes |
| `sudo gateway restart` | Restart the gateway and return to the menu-driven CLI console | Yes |
| `gateway status` | Show runtime status, entry node, regular exit, and residential exit | No |

## Runtime Console

After `gateway start` succeeds in an interactive terminal, it enters the menu-driven CLI console. You land on the home menu first, then move into each workbench by number.

Supported actions:

- home menu: runtime status, nodes, subscriptions, networking, rules, extensions, config center, logs, and update notices
- node workbench: shows per-node latency, lets you press `T` to retest, and sorts the list by lower latency first
- subscription and proxy-source workbenches: create, switch, rename, or edit URL / file sources
- networking workbench: toggle TUN and local bypass
- rules workbench: toggle LAN direct, China direct, Apple, Nintendo, global proxy, and ad blocking
- extension workbench: manage `chains / script / off`, chains `rule / global`, residential proxy, and airport group settings
- full config center: still available from the menu through `gateway config`
- the old `--tui` entry has been removed; if you still pass it, the CLI tells you to use the default console

This makes it feel more like a persistent CLI workspace, not a one-shot command that prints and exits.

## Switching and Configuration

| Command | Purpose |
|---|---|
| `gateway switch` | Show the current proxy source and extension mode |
| `gateway switch url` | Switch to subscription URL mode |
| `gateway switch file /path/to/config.yaml` | Switch to a local Clash / mihomo config file |
| `gateway switch extension` | Show the current extension mode |
| `gateway switch extension chains` | Enable built-in chains mode |
| `gateway switch extension script /path/to/script.js` | Enable a JS extension script |
| `gateway switch extension off` | Turn off extension mode |
| `gateway chains` | Open the chains wizard |
| `gateway chains status` | Show the current chains configuration |
| `gateway chains disable` | Disable chains mode |

## LAN Sharing, TUN, and Local Bypass

| Command | Purpose |
|---|---|
| `gateway tun on` | Enable TUN transparent proxy mode |
| `gateway tun off` | Disable TUN mode |

It is now recommended to manage these through `gateway config`:

- `runtime.tun.enabled`
- `runtime.tun.bypass_local`
- runtime ports
- API secret

`bypass_local` is useful when:

- you want the current computer to keep using its normal local network path
- but other LAN devices should still use this computer as the shared gateway

## Proxy Groups and Node Switching

You can switch groups and nodes in two ways:

1. Web panel: `http://<your-lan-ip>:9090/ui`
2. Menu-driven CLI console:
   - start the gateway
   - open `Nodes and Proxy Groups`
   - choose a node group
   - review the per-node latency list; type `T` if you want to retest the whole group
   - pick a node from the latency-sorted list and press Enter

This makes the project feel closer to a CLI workbench for Clash Verge Rev style workflows.

## Health Checks and Maintenance

| Command | Purpose | Requires admin |
|---|---|:---:|
| `sudo gateway health` | Run health checks and try to repair common issues | Yes |
| `sudo gateway update` | Upgrade to the latest version, with mirror-aware download fallback | Yes |
| `sudo gateway update v3.4.3` | Update or roll back to a specific version; `3.4.3` also works | Yes |
| `gateway permission print` | Print the sudoers snippet | No |
| `sudo gateway permission install` | Install passwordless control rules so normal commands can auto-escalate later | Yes |
| `gateway permission status` | Show permission control status | No |

## Service Management

| Command | Purpose | Requires admin |
|---|---|:---:|
| `sudo gateway service install` | Install auto-start; on Windows this uses Task Scheduler | Yes |
| `sudo gateway service uninstall` | Remove auto-start | Yes |

## AI Skill

| Command | Purpose |
|---|---|
| `gateway skill` | Show skill info for AI clients |
| `gateway skill path` | Print the skill directory path |

The skill flow is designed so AI clients can operate the system by scenario, for example:

- enable LAN sharing
- configure chains
- switch proxy groups
- turn on local bypass
- run health checks and inspect logs

## Global Flags

| Flag | Purpose | Default |
|---|---|---|
| `--config <path>` | Specify the config file path | `./gateway.yaml` |
| `--data-dir <path>` | Specify the runtime data directory | `./data` |

Example:

```bash
sudo gateway start --config /etc/gateway/gateway.yaml --data-dir /var/lib/gateway
```
