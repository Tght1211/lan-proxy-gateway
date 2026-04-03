# Command Reference

[Chinese version](../commands.md)

## Common Starting Commands

| Command | Purpose | Requires admin |
|---|---|:---:|
| `gateway install` | Initial setup wizard: download `mihomo`, enter subscription info, generate config | No |
| `gateway config` | Interactive config center: proxy source, LAN sharing, rules, extensions | No |
| `gateway config show` | Show the current config summary | No |
| `sudo gateway start` | Start the gateway and enter the runtime TUI | Yes |
| `sudo gateway start --simple` | Start the gateway and enter the plain command console | Yes |
| `sudo gateway console` | Re-enter the runtime TUI without restarting the gateway | Yes |
| `sudo gateway console --simple` | Re-enter the plain command console without restarting the gateway | Yes |
| `sudo gateway stop` | Stop the gateway | Yes |
| `sudo gateway restart` | Restart the gateway | Yes |
| `gateway status` | Show runtime status, entry node, regular exit, and residential exit | No |

## Runtime Console

After `gateway start` succeeds in an interactive terminal, it enters a runtime TUI console. After leaving it, you can return with `gateway console`.

Supported actions:

- slash commands: `/status` `/summary` `/config` `/config open` `/proxy` `/tun` `/bypass` `/rules` `/rule` `/extension` `/chain` `/chains` `/nodes` `/speed` `/logs` `/help`
- top tabs: `Esc` back to the header, `←/→` switch sections, `↓ / Enter` return to the action list
- the right content area clearly marks each page as `info / action / confirm`
- node workbench: `Ctrl+P` opens it; once inside, press `T` to test the current node latency
- config workbenches: you can now change proxy source, TUN, local bypass, rule switches, chains mode, and residential-proxy settings directly inside the TUI
- refresh cue: `R` refreshes the current page and shows a short pulse
- confirmation flow: after `/stop` or `/restart`, type `y` or `n`
- plain command mode: `sudo gateway start --simple` or `sudo gateway console --simple`, with the same core config actions

This makes it feel more like a persistent CLI workspace, not a one-shot command that prints and exits.

Common plain-command examples:

- `proxy source url`
- `proxy url https://example.com/sub`
- `tun on`
- `bypass off`
- `rule ads off`
- `extension chains`
- `chain mode global`
- `chain airport Auto`

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
2. Runtime TUI:
   - start the gateway
   - press `Ctrl+P`
   - choose a node group
   - choose a node and press Enter

This makes the project feel closer to a CLI workbench for Clash Verge Rev style workflows.

## Health Checks and Maintenance

| Command | Purpose | Requires admin |
|---|---|:---:|
| `sudo gateway health` | Run health checks and try to repair common issues | Yes |
| `sudo gateway update` | Upgrade to the latest version, with mirror-aware download fallback | Yes |
| `gateway permission print` | Print the sudoers snippet | No |
| `sudo gateway permission install` | Install passwordless control rules so normal commands can auto-escalate later | Yes |
| `gateway permission status` | Show permission control status | No |

## Service Management

| Command | Purpose | Requires admin |
|---|---|:---:|
| `sudo gateway service install` | Install an auto-start system service | Yes |
| `sudo gateway service uninstall` | Remove the auto-start service | Yes |

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
