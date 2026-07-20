# Changelog

## v4.0.0 - 2026-07-20

### Breaking changes

- Rebuilt the project as a focused LAN bypass gateway. This is not an in-place compatible v3 upgrade.
- Removed the bundled mihomo engine, subscriptions, nodes, rule sets, GeoIP, scripts, WebUI, live traffic dashboard, device naming, and Windows support.
- Legacy configuration is backed up and requires v4 onboarding again.

### Gateway

- Added an in-process transparent TCP relay with direct, SOCKS5, and HTTP CONNECT egress.
- Added native macOS pf and Linux iptables rule management.
- Added an IPv4 DNS forwarder with `fake_ip: true` and `hijack: false`: external proxy software receives domains without globally intercepting DNS.
- Proxy mode blocks QUIC so browsers fall back to TCP; other UDP remains direct.
- Added detached daemon lifecycle, hot configuration reload, status API, and precise firewall cleanup.

### Proxy configuration

- Added `gateway system-proxy status|on|off`.
- On macOS, the command updates HTTP/HTTPS or SOCKS5 settings through `networksetup` and uses the same endpoint for LAN gateway traffic.
- On Linux, the endpoint configures LAN gateway egress without modifying host desktop proxy settings.
- Proxy nodes and traffic rules remain the responsibility of external Clash, Mihomo, or sing-box software.

### Interface and distribution

- Replaced the live dashboard with a one-level terminal control panel: start/stop, proxy, device settings, and recent logs.
- Kept `gateway update` with a mandatory migration notice and explicit confirmation.
- Limited builds to macOS and Linux on amd64/arm64.
- Rewrote README and operational documentation around low-power Mac mini and small Linux hosts.
