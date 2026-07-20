# lan-proxy-gateway v4.0.0

v4 is a complete rewrite focused on one job: using an always-on macOS or Linux computer as a lightweight LAN bypass gateway while reusing an existing Clash, Mihomo, or sing-box proxy endpoint.

## Important upgrade notice

This release is not configuration-compatible with v3.

- The bundled mihomo engine, subscriptions, nodes, rule sets, WebUI, traffic dashboard, scripts, and Windows build are removed.
- Existing `gateway.yaml` is backed up as `gateway.yaml.pre-v4.bak*`; run the v4 initialization flow again.
- Proxy nodes and routing rules must live in external proxy software.
- Back up `~/.config/lan-proxy-gateway/` and note the HTTP/SOCKS5 endpoint before updating.

`gateway update` displays this migration notice and requires confirmation before downloading or replacing the binary.

## Highlights

- Transparent IPv4 TCP relay with direct, SOCKS5, and HTTP CONNECT egress.
- Native macOS pf and Linux iptables integration.
- Built-in DNS forwarder with proxy-mode fake-IP and DNS hijacking disabled: domains reach external proxy rules without intercepting other resolvers.
- macOS system proxy configuration through `networksetup`, synchronized with LAN gateway egress.
- Linux LAN proxy endpoint configuration without desktop integration.
- One-level terminal UI for start/stop, proxy configuration, device parameters, and recent logs.
- Detached daemon, hot reload, status API, and cleanup of owned firewall state.

## Supported platforms

- macOS amd64 / arm64
- Linux amd64 / arm64

Windows, Docker, consumer routers/OpenWrt, IPv6 transparent routing, and UDP proxying are not supported. In proxy mode UDP/443 is rejected so browsers can fall back from QUIC to TCP; other UDP traffic remains direct.
