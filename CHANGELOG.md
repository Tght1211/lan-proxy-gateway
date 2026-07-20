# Changelog

## v4.0.3 - 2026-07-21

### Fixed

- Fixed HTTP CONNECT requests containing duplicate `Host` headers, which caused some mixed HTTP/SOCKS proxy endpoints to close every LAN connection with `unexpected EOF`.
- Fixed successful HTTP CONNECT tunnels being closed through the response body lifecycle when the upstream returned a standard bodyless `200 Connection established` response.
- Added regression coverage for strict CONNECT header handling, bodyless successful responses, and safe loading of configurations left by pre-release UDP experiments.

### Documentation

- Added real-device Fast.com and Nintendo Switch results, including YouTube access through the LAN gateway.
- Clarified that gateway and the external proxy do not require TUN, that QUIC falls back to TCP in proxy mode, and that node selection and routing remain the external proxy software's responsibility.
- Corrected phone and PlayStation setup guidance for DNS, NAT expectations, and external routing rules.

## v4.0.2 - 2026-07-20

### Fixed

- Fixed the relay incorrectly terminating every long-lived TCP connection after two minutes. The drain timeout now starts only after one direction reaches EOF, preventing video streams such as YouTube from being interrupted and reconnected.
- Preserved Go's optimized TCP copy path, including zero-copy `splice` on Linux, by recording transfer totals after each copy direction completes instead of wrapping every write.

## v4.0.1 - 2026-07-20

### Fixed

- Enabled fake-IP by default in proxy mode while keeping DNS hijacking disabled. The relay now passes original domains to Clash/sing-box instead of forwarding potentially polluted local DNS results as destination IPs.
- Fixed sites such as YouTube failing when the external proxy depends on domain-based rules.

## v4.0.0 - 2026-07-20

### Breaking changes

- Rebuilt the project as a focused LAN bypass gateway. This is not an in-place compatible v3 upgrade.
- Removed the bundled mihomo engine, subscriptions, nodes, rule sets, GeoIP, scripts, WebUI, live traffic dashboard, device naming, and Windows support.
- Legacy configuration is backed up and requires v4 onboarding again.

### Gateway

- Added an in-process transparent TCP relay with direct, SOCKS5, and HTTP CONNECT egress.
- Added native macOS pf and Linux iptables rule management.
- Added an IPv4 DNS forwarder. v4.0.1 corrects its proxy-mode defaults so external proxy software receives domains without globally intercepting DNS.
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
