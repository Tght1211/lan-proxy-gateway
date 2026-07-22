# macOS App / Observability Handoff

## Branch and goal

- Branch: `feat/macos-observability-app`
- Goal: keep the existing CLI and add a native macOS DMG App in the same repository.
- Architecture: one Go gateway daemon, two control surfaces (CLI and SwiftUI App). The frontends may be open at the same time; they never create two gateway data-plane processes.

## Implemented

### Shared core and CLI

- Added `gateway init` for non-interactive default configuration.
- `gateway start` now reports when it connected to an already-running daemon.
- Kept the existing CLI behavior and release artifacts.

### Core telemetry

- Bounded recent connection history (240 records, memory only).
- Five-second traffic samples (360 points, about 30 minutes, memory only).
- Per-device and per-service upload/download/connection aggregates.
- Domain-based service classification for common services such as YouTube, Netflix, Apple, Nintendo, PlayStation, Steam, TikTok, Bilibili, GitHub and others.
- Egress latency, jitter, availability and bounded probe history (10-second probe interval).
- No telemetry is persisted or transmitted off-host.

### Native macOS App

- SwiftUI + Swift Charts, minimum macOS 13.
- Dark network operations UI with these pages:
  - Network overview
  - Live traffic
  - Service analysis
  - Device insights
  - Stability
  - Connection history/search
  - Egress configuration
  - CLI/service/log settings
- Administrator authorization is requested only for privileged gateway, system proxy and LaunchDaemon operations.
- The App bundles the same Go `gateway` executable and can install it to `/usr/local/bin/gateway`.
- First-run guidance takes users from their third-party proxy address to a running gateway.
- Older running cores are decoded defensively and shown with a one-click current-core restart prompt.
- Telemetry schema v2 adds cached egress public IP/region/ISP identity and improved DNS-assisted service labels.
- LaunchDaemon installation first places the bundled core at `/usr/local/bin/gateway`, so it never depends on a mounted DMG or movable App path.

### Build and release

- `make build-app`: native Swift release build.
- `make dmg VERSION=<version>`: App bundle, icon, ad-hoc or Developer ID signing, and DMG.
- Full Xcode builds universal SwiftUI + universal Go core.
- Command Line Tools-only machines build a host-architecture SwiftUI shell with a universal Go core.
- GitHub Release now runs on `macos-15`, keeps all CLI archives and adds the DMG.

## Verification completed

```text
go test ./...                                      PASS
go test -race ./internal/relay ./internal/app      PASS
swift build -c debug                               PASS
VERSION=v0.1.0-dev ./macos/build-dmg.sh            PASS (arm64 host)
```

Latest local artifact (ignored by git):

```text
dist/LANProxyGateway-v4.1.0-dev-macos-arm64.dmg
```

## Important behavior and limitations

1. "Service/App" identification is domain-level network service recognition. A gateway cannot see the process name inside an iPhone, console or TV. Do not label this as exact process-level App detection.
2. Domain visibility is best in proxy mode with fake-IP DNS enabled. Direct-mode traffic may only expose destination IPs and is labeled as unidentified traffic.
3. Telemetry is session-local and resets when the daemon restarts. Persistence and retention configuration are not implemented.
4. The loopback API is read/status oriented. The App currently executes privileged CLI commands through macOS administrator authorization rather than adding privileged write endpoints.
5. The local build is ad-hoc signed. Public distribution still needs an Apple Developer ID certificate and notarization credentials.
6. Native visual QA through Computer Use was blocked because Accessibility and Screen Recording permissions were not granted. Compilation and DMG packaging passed, but every page should still be inspected manually on a Mac.

## Recommended next tasks

1. Run the App with a real gateway daemon and several LAN devices; validate charts with active and completed traffic.
2. Inspect all pages at 1080x700, 1280x820 and a large desktop window; fix any clipping or table sizing.
3. Verify the `macos-15` universal build path in GitHub Actions.
4. Add Developer ID signing and notarization (`notarytool`) to Release secrets/workflow.
5. Decide whether to persist hourly/daily telemetry in SQLite. Add retention and privacy controls before doing so.
6. Add optional device aliases based on user-entered names; do not guess identities from IP addresses.
7. Evolve the current `schema_version: 2` contract deliberately when adding incompatible telemetry fields.

## Key files

```text
internal/relay/tracker.go                  telemetry and service classification
internal/app/supervisor.go                 stability probes and metrics
internal/app/api.go                        loopback status API
macos/Sources/LANProxyGatewayApp/          native App
macos/build-dmg.sh                         App/DMG build
.github/workflows/release.yml              release pipeline
```
