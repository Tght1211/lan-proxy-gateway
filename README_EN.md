# LAN Proxy Gateway

[![Release](https://img.shields.io/github/v/release/Tght1211/lan-proxy-gateway)](https://github.com/Tght1211/lan-proxy-gateway/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)]()
[![License](https://img.shields.io/github/license/Tght1211/lan-proxy-gateway)](LICENSE)

Turn an always-on Mac mini or small Linux host into a LAN gateway. Switch, PS5, Apple TV, phones, and TVs can reuse an existing Clash, Mihomo, sing-box, or HTTP/SOCKS5 proxy by pointing their gateway and DNS at this host. No proxy app is needed on the client device.

> [!IMPORTANT]
> This project provides no proxy nodes, subscriptions, or streaming unlocks. Service availability depends on your external proxy software and node.

[中文](README.md) · [Latest release](https://github.com/Tght1211/lan-proxy-gateway/releases/latest) · [Documentation](docs/README.md)

<p align="center">
  <img src="docs/images/app-network-overview.png" alt="Network overview with gateway status, traffic topology, and network quality" width="49%">
  <img src="docs/images/app-devices-services.png" alt="Connected devices and service traffic" width="49%">
  <img src="docs/images/app-access-log.png" alt="Live connections and result filters" width="49%">
  <img src="docs/images/app-settings.png" alt="Themes, updates, and runtime options" width="49%">
</p>

## Features

- **LAN gateway:** route client IPv4 TCP traffic directly or through one HTTP/SOCKS5 upstream.
- **Original-domain forwarding:** fake-IP preserves domains for Clash/sing-box routing without requiring TUN mode.
- **Lightweight routing:** ordered domain, domain-suffix, and IP-CIDR rules can select proxy, direct, or reject behavior, with automatic learning from successful direct fallbacks.
- **Native macOS app:** inspect traffic topology, clients, services, connection outcomes, and network quality; manage rules, proxy settings, and the gateway core.
- **Native networking:** `pf` on macOS and `iptables` on Linux, with precise cleanup of rules and system state on shutdown.

LAN Proxy Gateway is designed for an always-on computer with an existing proxy application or HTTP/SOCKS5 endpoint. It does not target Windows, consumer routers/OpenWrt, IPv6 transparent proxying, or proxied game/voice UDP. Proxy mode rejects QUIC (UDP/443) so clients fall back to TCP; other UDP remains direct.

## Quick start

### 1. Install

On macOS, download the DMG from [GitHub Releases](https://github.com/Tght1211/lan-proxy-gateway/releases/latest). The app and CLI share the same `gateway` core.

For the CLI on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh | bash
sudo gateway install
```

Administrator access is required to bind DNS, enable IP forwarding, and configure the firewall. Routine status and configuration commands do not require persistent root access after service installation.

### 2. Configure egress

Set the proxy in the app, or run `sudo gateway` and choose **Set proxy**. A proxy on the same host commonly looks like this; use the actual port shown by your proxy software:

```text
Type     HTTP
Address  127.0.0.1
Port     7897
```

Direct egress is also supported when you only need a LAN gateway.

### 3. Connect a device

Run `gateway status`, then enter the reported values on the phone, TV, or console:

| Device setting | Value |
|---|---|
| IP configuration | Manual / static |
| IP address | An unused address in the same subnet; unique per device |
| Subnet mask | `255.255.255.0`; usually prefix length `24` on Android |
| Gateway / router | Gateway host's LAN IP |
| DNS 1 | Gateway host's LAN IP |
| DNS 2 | Same value, or blank if duplicates are rejected |
| Device proxy | None / disabled |

Reconnect Wi-Fi and run `gateway status` again. Only the device IP differs between clients; gateway and DNS always use the gateway host's IP.

Detailed device guides are available from the [documentation index](docs/README.md). Troubleshooting is covered in the [FAQ](docs/faq.md).

## Documentation

| Topic | Links |
|---|---|
| Device setup | [Overview](docs/device-setup.md) · [Phone](docs/phone-setup.md) · [Switch](docs/switch-setup.md) · [PS5](docs/ps5-setup.md) · [Apple TV](docs/appletv-setup.md) · [TV](docs/tv-setup.md) |
| Operation | [macOS app](docs/app.md) · [Commands](docs/commands.md) · [Configuration](docs/advanced.md) · [Scenarios](docs/scenarios.md) |
| Internals | [Architecture](docs/architecture.md) · [Real-device results](docs/real-device-results.md) · [FAQ](docs/faq.md) |
| Upgrade and automation | [v3 migration](docs/migration-v4.md) · [AI-assisted setup](docs/ai-setup.md) · [Changelog](CHANGELOG.md) |

Most detailed guides are currently written in Chinese. Start from [docs/README.md](docs/README.md).

## Build from source

```bash
make build       # CLI
make test        # Go tests
make build-app   # macOS app
```

See the [app guide](docs/app.md) and [architecture](docs/architecture.md) for packaging and repository structure.

## License

[MIT](LICENSE) © 2025-2026 [Tght1211](https://github.com/Tght1211)
