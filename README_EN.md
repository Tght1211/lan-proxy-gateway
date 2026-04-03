# LAN Proxy Gateway

[Chinese README](README.md)

[![Release](https://img.shields.io/github/v/release/Tght1211/lan-proxy-gateway)](https://github.com/Tght1211/lan-proxy-gateway/releases)
[![Stars](https://img.shields.io/github/stars/Tght1211/lan-proxy-gateway?style=social)](https://github.com/Tght1211/lan-proxy-gateway/stargazers)
[![License](https://img.shields.io/github/license/Tght1211/lan-proxy-gateway)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

Turn your computer into a LAN-wide transparent proxy gateway.  
No router flashing, no extra soft router, just change the gateway and DNS on your `Switch / PS5 / Apple TV / smart TV / phone`.

This project is built on `mihomo` and focuses on two things:

- `LAN sharing`: devices that cannot install proxy apps can still use transparent proxying
- `Chains`: `Claude / ChatGPT / Codex / Cursor` can use a cleaner residential exit

> Fully open source. Chinese-first project, but the networking model is globally applicable.

```mermaid
graph TD
    Internet(("🌐 Internet"))
    Router["🔲 Router<br/>192.168.x.1"]
    Mac["🖥 Gateway Computer<br/>running mihomo · 192.168.x.2"]
    Switch["🎮 Switch<br/>YouTube · eShop"]
    ATV["📺 Apple TV<br/>Netflix · Disney+"]
    PS5["🕹 PS5 / Xbox<br/>PSN · Online Play"]
    TV["📡 Smart TV<br/>Streaming"]
    Phone["📱 Phone / Computer<br/>Normal Internet"]

    Internet <--> Router
    Router <--> Mac
    Router <--> Phone
    Mac -- "Gateway + DNS -> gateway IP" --> Switch
    Mac -- "Gateway + DNS -> gateway IP" --> ATV
    Mac -- "Gateway + DNS -> gateway IP" --> PS5
    Mac -- "Gateway + DNS -> gateway IP" --> TV

    style Mac fill:#2d9e2d,color:#fff,stroke:#1a7a1a
    style Internet fill:#4a90d9,color:#fff,stroke:#2a6ab9
    style Router fill:#f5a623,color:#fff,stroke:#d4891a
    style Switch fill:#e60012,color:#fff,stroke:#b8000e
    style ATV fill:#555,color:#fff,stroke:#333
    style PS5 fill:#006fcd,color:#fff,stroke:#0055a0
    style TV fill:#8e44ad,color:#fff,stroke:#6c3483
    style Phone fill:#95a5a6,color:#fff,stroke:#7f8c8d
```

## Core Capabilities

### 1. LAN-wide transparent sharing

- Devices join by changing only gateway and DNS
- Supports `Switch / PS5 / Apple TV / smart TV / phone / tablet`
- Supports `TUN` mode and `bypass local host`

### 2. Chains mode

```text
Your device -> airport node -> residential proxy -> Claude / ChatGPT / Codex / Cursor
```

Useful for:

- Claude / ChatGPT signup and usage
- Codex / Cursor style AI coding tools
- keeping daily traffic on airport nodes while AI traffic uses residential exit

### 3. Runtime console

By default, `gateway start` opens the plain command console. It is the more compatible mode, and it is the safer default while the runtime TUI is still being refined.

Common plain-command actions include:

- `proxy source url`
- `tun on`
- `bypass off`
- `rule ads off`
- `extension chains`
- `chain mode global`

If you want the full runtime TUI workbench, use:

```bash
sudo gateway start --tui
```

The TUI workspace gives you:

- tabs, arrow-key navigation, and Enter-to-run actions
- the right side clearly labels each page as `info / action / confirm`
- a command bar at the bottom for direct commands
- `Ctrl+P` opens the node workbench for switching nodes and pressing `T` to test the current node latency
- `Esc` back to the top tabs, `←/→` to switch sections, and `↓` back into the action list
- `R` refreshes the current page and gives a short visual refresh cue

If you are already in simple mode, you can also type `tui` to switch into it later.

### 4. Rule system

Built-in defaults include:

- LAN and reserved address direct access
- common mainland-China services direct access
- Apple / Nintendo related rules
- ad and tracking domain blocking
- proxy rules for overseas sites and AI services

## 3-Minute Quick Start

### Step 1: Install

Install `gateway` first. For users in mainland China, the CDN entry is usually easier.

#### macOS / Linux

Recommended:

```bash
curl -fsSL https://cdn.jsdelivr.net/gh/Tght1211/lan-proxy-gateway@main/install.sh | bash
```

Fallback:

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | bash
```

#### Windows PowerShell

Recommended:

```powershell
irm https://cdn.jsdelivr.net/gh/Tght1211/lan-proxy-gateway@main/install.ps1 | iex
```

Fallback:

```powershell
irm https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.ps1 | iex
```

If GitHub is unstable from your network, you can also force a mirror:

```bash
GITHUB_MIRROR=https://hub.gitmirror.com/ bash install.sh
```

### Step 2: Initialize

```bash
gateway install
```

The setup wizard will:

1. automatically download the official `mihomo` kernel (on Windows x86_64 it downloads the official zip and installs it locally as `mihomo.exe`)
2. ask for a subscription URL or local config file
3. generate `gateway.yaml`

If you just want the fastest path, only fill these:

- proxy source
- subscription URL or local config file
- subscription name

### Step 3: Start the gateway

**macOS / Linux:**

```bash
sudo gateway start
```

**Windows (run in an Administrator terminal):**

```powershell
gateway start
```

Extra notes:

- `gateway update` now uses a background replacement flow on Windows, so the current `.exe` can be swapped after exit and the gateway is restarted automatically
- `gateway service install` uses Task Scheduler on Windows instead of pretending the CLI itself is a native `sc.exe` service

By default, startup enters the plain command console and shows:

- the config file path in use
- your LAN gateway IP
- runtime mode
- egress summary
- the simple-mode command prompt
- direct commands for common settings

The default workspace now handles these common tasks directly:

- proxy source and subscription info
- TUN and local bypass
- recommended rule switches such as China direct and ad blocking
- `chains / script / off`
- `rule / global` for chains mode
- residential proxy and airport group settings

The most important thing here is your LAN IP.

If you want the runtime TUI workbench:

```bash
# macOS / Linux
sudo gateway start --tui

# Windows (Administrator terminal)
gateway start --tui
```

The TUI provides the `Home / Proxy / Subscription` tabs, node switching, config workbenches, and the Dashboard view. If you still want to spell out the default, `gateway start --simple` is still accepted.

If you leave the console, you can re-enter it at any time:

```bash
# macOS / Linux
sudo gateway console
sudo gateway console --tui

# Windows (Administrator terminal)
gateway console
gateway console --tui
```

### Step 4: Connect another device

Change the device's:

- `Gateway` to your computer's LAN IP
- `DNS` to the same IP

If you want a quick first test, start with:

- [iPhone / Android](docs/en/phone-setup.md)
- [Nintendo Switch](docs/en/switch-setup.md)
- [PS5](docs/en/ps5-setup.md)
- [Apple TV](docs/en/appletv-setup.md)
- [Smart TV](docs/en/tv-setup.md)

### Step 5: Verify

```bash
gateway status
```

You will see:

- current node
- entry node
- regular exit
- residential exit, if `chains` is enabled

## Common Commands

> **Windows users:** commands listed with `sudo` must be run in an **Administrator terminal** without the `sudo` prefix — e.g. `sudo gateway start` → `gateway start`.

| Command | Purpose |
|---|---|
| `gateway install` | Initial setup wizard |
| `gateway config` | Interactive config center |
| `sudo gateway start` | Start gateway and open the default plain command console |
| `sudo gateway start --tui` | Start gateway and open the runtime TUI workbench |
| `sudo gateway console` | Re-enter the plain command console without restarting the gateway |
| `sudo gateway console --tui` | Re-enter the runtime TUI workbench without restarting the gateway |
| `gateway tun on` | Enable TUN transparent proxy mode |
| `gateway tun off` | Disable TUN transparent proxy mode |
| `gateway status` | Show runtime and egress status |
| `gateway chains` | Chains / residential proxy wizard |
| `gateway switch` | Switch proxy source and extension mode |
| `gateway skill` | Show AI skill info |
| `gateway permission install` | Install passwordless control rule (macOS/Linux only) |
| `sudo gateway service install` | Install auto-start (implemented with Task Scheduler on Windows) |
| `sudo gateway update` | Upgrade to the latest version |

Full command reference: [docs/en/commands.md](docs/en/commands.md)

## How It Works

```mermaid
flowchart LR
    Device["📱 LAN Device"] --> Mac["🖥 Gateway Computer<br/>IP Forwarding"]
    Mac --> TUN["mihomo<br/>TUN Virtual NIC"]
    TUN --> Rules{"Routing Rules"}
    Rules -- "China traffic" --> Direct["🇨🇳 Direct"]
    Rules -- "Overseas traffic" --> Proxy["🌐 Proxy Node"]
    Rules -- "Ads" --> Block["🚫 Block"]

    style Mac fill:#2d9e2d,color:#fff,stroke:#1a7a1a
    style TUN fill:#3498db,color:#fff,stroke:#2980b9
    style Rules fill:#f39c12,color:#fff,stroke:#d68910
    style Direct fill:#27ae60,color:#fff,stroke:#1e8449
    style Proxy fill:#8e44ad,color:#fff,stroke:#6c3483
    style Block fill:#e74c3c,color:#fff,stroke:#c0392b
```

1. The computer enables IP forwarding and becomes the LAN gateway
2. `mihomo` captures traffic in TUN mode
3. The rule system decides direct, proxy, or block
4. In `chains` mode, AI traffic can continue to a residential exit

## Documentation

- [Command Reference](docs/en/commands.md)
- [Advanced Guide](docs/en/advanced.md)
- [FAQ](docs/en/faq.md)
- [Versioning Notes](docs/en/versioning.md)
- [Switch Setup](docs/en/switch-setup.md)
- [PS5 Setup](docs/en/ps5-setup.md)
- [Apple TV Setup](docs/en/appletv-setup.md)
- [Smart TV Setup](docs/en/tv-setup.md)
- [Phone Setup](docs/en/phone-setup.md)

## How It Differs from Clash Verge LAN Access

| Item | Clash Verge LAN Access | LAN Proxy Gateway |
|---|---|---|
| Proxy layer | Application-level proxy | Network-level transparent proxy |
| Device setup | Fill proxy server address | Change gateway and DNS |
| Switch / Apple TV / PS5 | Limited in some cases | Better for full-device transparent takeover |
| App proxy awareness | Often detectable | Closer to a real gateway |
| Typical use | Per-device proxy | Whole-home shared gateway |

## Open Source

This project is mainly for:

- networking and proxy learning
- home LAN gateway practice
- TUN / transparent proxy / routing rule experiments
- AI client and CLI / TUI interaction design

Use it only where legal and appropriate.

## License

[MIT](LICENSE)
