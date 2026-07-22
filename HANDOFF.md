# macOS App Handoff

Last updated: 2026-07-22

This document describes the actual state of `feat/macos-observability-app`. Do not treat a successful build as UI acceptance. The latest UI changes compile, but have not received complete visual QA or user approval.

## 1. Repository state

- Repository: `https://github.com/Tght1211/lan-proxy-gateway`
- Branch: `feat/macos-observability-app`
- Current commit: `b6ba83e5792e4349b8373e47c9edcdfd775d0c82`
- Commit: `feat(macos): add routing policies and device insights`
- Remote branch is pushed at the same commit.
- Working tree was clean when this handoff was updated.
- No pull request, tag, or release has been created for this branch.

Latest local preview:

```text
dist/LANProxyGateway-v4.1.0-ui-preview9-macos-arm64.dmg
SHA-256: 103e4a89b2bf7ae9bfae92ff0b9f5f2a029bc2590ec725fed2a2beb478538248
```

Do not publish this DMG as a stable release until the UI has been visually inspected and accepted.

## 2. Product boundary

The product turns an always-on Mac mini or Linux mini PC into a lightweight LAN proxy gateway.

The intended flow is:

```text
Switch / PS5 / Apple TV / phone
  -> static IP + gateway IP + DNS
  -> LAN Proxy Gateway
  -> ordered routing rules
  -> external Clash/Mihomo/sing-box HTTP or SOCKS5 proxy, direct, or reject
```

Hard boundaries:

- Do not bundle a proxy engine.
- Do not require TUN in this program or the third-party proxy application.
- Reuse a separately installed HTTP or SOCKS5 proxy.
- macOS and Linux only for now.
- No Windows or Docker support for now.
- This is not intended to become a full OpenWrt replacement.
- Keep the daemon and UI lightweight, stable, and understandable to non-technical users.

## 3. User's UI requirements

Treat these as acceptance criteria, not suggestions:

- Use a light, restrained, polished macOS visual style. Avoid an all-black/neon dashboard.
- Keep hierarchy clear and avoid disconnected or nested cards.
- Remove duplicate pages and metrics. The overview should not be repeated as another menu item.
- Only the network overview may scroll as a whole page.
- Other pages should fit the window and use internal scrolling only where necessary.
- Hide visible scrollbars, including the live-log view.
- Live logs should automatically follow the newest entry.
- Startup state and control should be one native sliding toggle.
- Settings should open without a noticeable pause.
- The topology lines must be straight, aligned, and animated to indicate live flow.
- Show device label alongside its static IP wherever a device is displayed.
- Support persistent device remarks/labels and a clear area listing labeled devices.
- Provide beginner-oriented setup instructions and recommended static IP choices.
- Assume a small household deployment, usually no more than five client devices.
- Service analysis must support all-device and per-device filtering.
- Show each device's top three services plus an aggregate service list.
- Connection history needs quick state/path/device/unresolved filters.
- Show the active proxy clearly, including public egress IP, region, and ISP when available.
- Show stability history visually, not only a current latency number.
- The network overview topology should show:

```text
LAN devices -> gateway -> rule decision -> PROXY / DIRECT / REJECT
```

## 4. Implemented in the Go core

- Existing gateway daemon and CLI are preserved.
- Bounded recent connection history and traffic samples.
- Per-device and per-service byte/connection aggregation.
- DNS-assisted domain/service classification.
- Persistent fake-IP mappings across daemon restarts.
- Egress latency, jitter, availability, and bounded probe history.
- Cached public egress IP, region, and ISP identity.
- API schema version is `3`.
- Ordered, first-match-wins routing rules with hot reload.
- Connections rejected by routing rules are recorded in recent history with `rejected: true` (excluded from usage aggregates).

Supported rule match types:

- `domain`
- `domain-suffix`
- `ip-cidr` (matches the real destination IP; never matches fake-IP targets where only the domain is observable)

Supported actions:

- `proxy`
- `direct`
- `reject`

Example configuration:

```yaml
routing:
  rules:
    - type: domain-suffix
      value: alibaba.com
      action: direct
```

CLI example:

```bash
gateway routing set --rules-json '[{"type":"domain-suffix","value":"alibaba.com","action":"direct"}]'
```

`PROCESS-NAME` is intentionally unsupported. A gateway cannot observe process names on a remote iPhone, console, or TV.

Core files:

```text
cmd/routing.go
internal/config/schema.go
internal/config/load.go
internal/relay/routing.go
internal/relay/relay.go
internal/relay/tracker.go
internal/app/app.go
internal/app/daemon.go
internal/app/api.go
```

## 5. Implemented in the macOS source

The native App is SwiftUI + Swift Charts, minimum macOS 13. The current source attempts to provide:

- Network overview with device-to-egress topology.
- Animated proxy/direct/reject topology branches.
- Device IPs and local labels in topology and connection rows.
- Routing rule editor and per-branch rule counts.
- Traffic and stability visualizations.
- Last 60 stability probes in the relevant views.
- All-device and per-device service analysis.
- Per-device top three services.
- Connection filters for state, device, proxy/direct path, and unresolved destinations.
- Persistent device labels through App-local `UserDefaults`.
- Suggested static IP choices `.201` through `.205` and setup guidance.
- Public egress IP, region, and ISP display.
- GitHub project link in settings.
- Log auto-following with rendering limited to the last 120 lines.
- Hidden scroll indicators.
- LaunchDaemon startup represented by a SwiftUI toggle.
- Compatibility prompt when an old daemon reports a schema older than version 3.

Primary UI files:

```text
macos/Sources/LANProxyGatewayApp/ContentView.swift
macos/Sources/LANProxyGatewayApp/AppModel.swift
macos/Sources/LANProxyGatewayApp/GatewayClient.swift
macos/Sources/LANProxyGatewayApp/Models.swift
```

`ContentView.swift` grew by roughly 950 lines in the last commit. Review its structure carefully; do not keep appending large views without first extracting coherent components.

## 6. Terminology and observability limits

The former label `IP 地址流量` was changed to `未解析域名`.

An unresolved destination can occur when:

- A device connects directly to an IP address.
- A fake-IP/DNS mapping expired or is unavailable.
- The protocol does not expose a domain observable by the gateway.

The gateway can reliably record source device, destination IP/port, time, transferred bytes, and final route. It cannot infer the user's exact operation inside encrypted HTTPS/TLS traffic. Do not claim exact application or action recognition.

## 7. Verification already completed

These checks passed against the latest committed source:

```bash
go test ./...
go test -race ./internal/relay ./internal/app ./internal/config
swift build --package-path macos -c release
git diff --check
codesign --verify --deep --strict "dist/LAN Proxy Gateway.app"
```

An isolated CLI test also passed for `gateway init`, `gateway routing set`, config persistence, and `gateway status --json`.

This proves compilation and core behavior only. It does not prove that the macOS layouts satisfy the screenshots or fit all supported window sizes.

## 8. Unverified or incomplete items

These are the main reasons the current branch must not be called finished:

1. The latest UI was not visually inspected after the last commit.
2. The dynamic topology may be too wide, misaligned, or clipped at smaller window sizes.
3. If default egress is direct, a `proxy` rule may have no active proxy dialer and fall back to direct.
4. Stability probes measure the configured default egress, not proxy and direct branches independently.
5. Device labels live only in macOS `UserDefaults`; the CLI and Linux host do not share them.
6. Suggested `.201-.205` addresses are only candidates. The program does not inspect DHCP leases or detect conflicts.
7. Hidden scrollbar and fixed-page behavior must be rechecked on every page.
8. Settings latency needs measurement with a large real log file.
9. The routing topology and editor need real `DIRECT`, `PROXY`, and `REJECT` traffic tests.
10. The current menu/page organization still needs comparison with every user screenshot to confirm redundant pages are gone.
11. The final visual polish, spacing, typography, empty states, and card grouping have not been approved by the user.

Fixed in code but not yet visually or manually verified:

- Routing editor rows can be drag-reordered (List + onMove); needs hands-on check.
- The editor now waits for asynchronous save validation and stays open with an inline error on failure.
- Rejected connections appear in connection history with a `拒绝`/`REJECT` state and a dedicated route filter.
- `ip-cidr` rules are supported end to end (config validation, relay matching against the real destination IP, editor picker); needs real-traffic testing.

## 9. Required next workflow

1. Build the App from current `HEAD`.
2. Quit every previously running copy before opening the new build.
3. Confirm `/api/stats` reports `schema_version: 3`.
4. Restart the gateway daemon only after warning the user because it interrupts LAN forwarding.
5. Visually inspect every page at approximately 1440x900, 1200x800, and the minimum window size.
6. Compare each page directly with the screenshots and requirements in the originating Codex task.
7. Fix clipping, topology alignment, scrollbar behavior, duplicated information, and unclear active states.
8. Exercise real traffic for one `DIRECT`, one `PROXY`, and one `REJECT` domain rule.
9. Verify device labels, all connection filters, service filters, and setup guidance with multiple clients.
10. Re-run Go tests, race tests, Swift release build, codesign verification, and DMG packaging.
11. Commit and push follow-up fixes only after visual verification.
12. Do not create a stable release until the user explicitly accepts the UI.

## 10. Operational notes

- Building or opening the App is safe.
- Restarting the daemon interrupts connected LAN devices; ask before doing it.
- If direct network access fails, retry with:

```bash
HTTP_PROXY=http://127.0.0.1:7897 \
HTTPS_PROXY=http://127.0.0.1:7897 \
<command>
```

- Never add the residential proxy credentials previously pasted in chat to source, examples, logs, or documentation. They should be rotated because they were exposed in conversation.

## 11. Useful commands

```bash
git status --short --branch
git log -3 --oneline
go test ./...
go test -race ./internal/relay ./internal/app ./internal/config
swift build --package-path macos -c release
make dmg VERSION=v4.1.0-ui-preview
curl -s http://127.0.0.1:<api-port>/api/stats
```
