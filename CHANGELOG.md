# Changelog

All notable changes to this project will be documented here.

## v3.4.2 - 2026-05-12

### Changed

- Bumped pinned mihomo from `v1.18.6` to `v1.19.24`. Adds support for the `anytls` proxy type that landed in mihomo v1.19.3 (resolves [issue #4](https://github.com/Tght1211/lan-proxy-gateway/issues/4): subscriptions with `type: anytls` nodes were rejected with `unsupport proxy type: anytls`). v1.19.24 also brings vless reality fixes, hysteria2 stability, and several CVE patches for net/http. New users get this automatically; existing users on v1.18.6 must run the new `gateway install --reinstall-mihomo` flag (see Added) — `gateway install` alone won't re-download because it skips the step when mihomo is already on disk.
- Mihomo SIGTERM → SIGKILL grace bumped from **3s to 8s** on Unix. mihomo's TUN strict-route shutdown work (deleting high-pref ip rules, flushing custom routing table, tearing down the tun device) sometimes didn't fit in 3s; the SIGKILL escalation cut it short, leaving the rules behind (root cause of [issue #5](https://github.com/Tght1211/lan-proxy-gateway/issues/5)). 8s is generous for the normal case (clean exit < 2s) but gives slow / busy hosts real headroom.

### Added

- `gateway install --reinstall-mihomo` flag forces re-download of mihomo even if a binary already exists on disk, overwriting it with the version pinned to the current gateway release. Primary use: lifting v3.4.1 (mihomo v1.18.6) installs to v3.4.2 (mihomo v1.19.24) so subscriptions with anytls nodes start working
- New `Platform.PostStopCleanup()` hook on the OS abstraction. No-op on darwin / windows; on Linux it scrubs leftover mihomo TUN strict-route ip rules whose pref looks like mihomo's signature (`9000-9999` + `unreachable` action). Defends against the case where mihomo got SIGKILL'd mid-cleanup or crashed, leaving stale `pref 9000 from all unreachable` rules behind that could break Docker DNAT for non-port-preserving port mappings (`host:container 2228:2283` style — root cause of [issue #5](https://github.com/Tght1211/lan-proxy-gateway/issues/5)). Wired into `gateway.Disable()` so it always runs on `gateway stop` after mihomo terminates and after the MASQUERADE rule has been removed

### Tests

- `internal/platform/cleanup_parser_test.go` — 7 cases covering the leftover-rule parser: detects mihomo signature, ignores clean systems, refuses to touch admin's own high-pref non-unreachable rules, refuses pref outside [9000,9999], handles multiple leftovers, handles empty / junk input. Parser lives in a portable file (`cleanup_parser.go`) so darwin / windows CI also runs the tests.
- `internal/gateway/gateway_test.go` — asserts `Gateway.Disable()` calls `UnconfigureNAT` → `DisableIPForward` → `PostStopCleanup` in that order, and that `PostStopCleanup` still runs when no interface was detected (defense-in-depth — the cleanup is independent of NAT teardown).
- `internal/mihomo/download_test.go::TestPinnedMihomoVersionSatisfiesAnytls` — version-pin guardrail. Refuses any future bump back below mihomo v1.19, where anytls support starts. Catches accidental regressions during cherry-picks / merges.

## v3.4.1 - 2026-05-06

### Fixed

- `gateway install` now generates a systemd unit with `After=network-online.target` + `Wants=network-online.target` instead of the old `After=network.target`. On Debian 13 with DHCP, the previous unit fired before the interface had an IPv4 lease, causing `gateway start` to fail with `detect network: no IPv4 address on enp0s1` and noisy auto-restart loops in `journalctl`. Reported by @lingbaoboy in [issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2). Reinstall the service (`sudo gateway install`) to pick up the new unit
- `traffic.auto_groups` now also injects the appended `Auto` / `Fallback` group names + `DIRECT` into the user's `Proxy` select group (appended at the tail, never reordering the head). v3.4.0 added `Auto` / `Fallback` to the proxy-groups list but the user's `Proxy` group did not reference them, so they showed up in the mihomo UI but could not actually be selected from the main entrypoint group ("看得见用不着"). DIRECT injection also gives a last-resort escape hatch when every node is down, so the host doesn't lose basic connectivity. Behavior change is gated on `auto_groups: true` only — users who never opted in see no difference

### Tests

- `internal/source/source_autogroups_test.go` — three new scenarios: existing url-test/fallback groups (named `🚀 自动` / `🛡 兜底`) get injected into Proxy, the auto-synthesized fallback Proxy group also receives Auto/Fallback/DIRECT, and `auto_groups=false` keeps Proxy's proxies list bit-identical to the subscription
- Existing `AutoGroupsAppendsBoth` updated to assert the Proxy group now contains `n1, n2, n3, Auto, Fallback, DIRECT` with `n1` still at index 0 (preserving mihomo's default selection)

## v3.4.0 - 2026-04-24

### Added

- `traffic.auto_groups` config (default `false`) plus a toggle in `[M] → 2 分流 & 规则 → 5 策略组自动补全`. When enabled, subscription / file sources get two extra groups injected only when missing: `Auto` (`type: url-test`) and `Fallback` (`type: fallback`), both referencing every proxy in the subscription. The detection is **by group type, not name**, so custom-named subscriptions (`🚀 节点选择`, `智能选择`, etc.) are recognized and no duplicate groups appear. Restores the v2.x template behavior that was lost in the v3.0 rewrite. Resolves the "no fallback when a directly selected node goes down" report in [issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2).

### Tests

- `internal/source/source_autogroups_test.go` — five scenarios: off, append both when subscription only has `select`, skip Auto when `url-test` exists, skip both when both types exist, rename to `Auto2` on name clash

## v3.3.2 - 2026-04-24

### Fixed

- Embedded mihomo template no longer forces `ipv6: false` and `dns.ipv6: false`; both default to `true` again, matching v2.x behavior. The hard-off was inherited from the v1 legacy template during the v3.0 rewrite and broke dual-stack targets (ChatGPT / Cloudflare / Google) for IPv6-preferring clients. The TUN block intentionally still does **not** hijack IPv6 routes, so the host's IPv6 reachability stays intact (avoids re-triggering the Linux-side IPv6 ping issue from [issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2))

## v3.3.1 - 2026-04-24

### Fixed

- Dashboard no longer duplicates "🛫 起飞 / 🛬 落地" as identical lines when there is no chain residential configured; single-hop setups now render one `🌐 出口节点` line, with the ipinfo egress row below providing the ground-truth location
- Added a `mixed` port health probe to the dashboard refresh: when the mihomo API is reachable but the LAN-facing mixed port is not (e.g. reload half-failed, TUN strict-route stole the port, or firewall blocks LAN ingress), the dashboard now prints a red "代理端口不通：LAN 设备连不上" warning with a repair hint instead of silently showing "● 运行中" while phones time out

## v3.3.0 - 2026-04-24

### Added

- Dashboard shows the real egress IP, location, and ISP under the 🛬 landing line by querying `https://ipinfo.io/json` through the local mixed port — gives an accurate read for chain-proxy / residential-IP setups where the flag emoji in a node name can lie
- `ls` now renders the full node list as a Linux-style multi-column grid (column-major, fastest in the first column, CJK/emoji width-aware) and `ll` renders the detailed single-column table; terminal width adapts via `COLUMNS`
- Log tail (readable mode) folds consecutive duplicate warnings with a `⋮ 上面那一行又重复 N 次（最近 HH:MM:SS）` summary, fixing the 15-second direct-timeout spam that used to swamp the screen
- `gateway.yaml` gained `gateway.device_labels: {ip: name}` so the dashboard device table can show user-assigned names over reverse-DNS fallback
- New `internal/ipinfo/`, `internal/geoip/`, and `internal/devices/` packages with unit tests

### Fixed

- `subscription` reload no longer self-bites under active TUN/fake-ip: `engine.Reload` fetches through the **old** mixed port before stopping and starting, avoiding `read: can't assign requested address` on 198.18.0.0/15
- No-op edits in "代理 & 订阅" (subscription URL/name, file path, script screen) no longer trigger `Save` + `Reload` when the user just presses Enter on defaults
- `gateway install` generates a systemd unit that includes `Environment=HOME=…` / `Environment=XDG_CONFIG_HOME=…/.config`, so Debian/Linux users who did `sudo gateway install` no longer end up with the service reading `/root/.config/…` and ignoring the real config ([issue #2](https://github.com/Tght1211/lan-proxy-gateway/issues/2))
- macOS `SetLocalDNSToLoopback` now also turns off system HTTP / HTTPS / SOCKS proxy state, so the method-3 DNS takeover isn't bypassed by a lingering system proxy
- Subscription / file content normalization handles UTF-8 BOM, `#!MANAGED-CONFIG` headers, and base64-encoded Clash YAML; fallback `Proxy` group is emitted as structured YAML instead of a malformed string

### Changed

- The "启动 / 重启 / 停止" menu now shows a calm hint when gateway is already running ("重启/停止 通常不需要 sudo") instead of the old blanket "启动/停止/清理会失败" warning — matches actual runtime behavior

## v2.2.10 - 2026-04-06

### Fixed

- Reworked `install.sh` so release downloads no longer sit on a slow direct GitHub asset connection for minutes before trying the next source
- Added fast candidate probing, low-speed cutover, and retry rotation across direct GitHub plus known mirrors for the Unix install script
- Forced the Unix install script to prefer `curl --http1.1` by default, which improves release-asset download stability on affected macOS networks

### Documentation

- Updated the Chinese and English README install sections to explain the new automatic download-source probing and when `GITHUB_MIRROR` is still the better manual fallback
- Updated the install success hints so `gateway start` / `gateway start --tui` match the current default simple-mode startup behavior

## v2.2.9 - 2026-04-04

### Changed

- Switched the default interactive startup for `gateway start`, `gateway console`, and `gateway restart` back to the plain command console so day-to-day use no longer drops users into the still-maturing TUI first
- Added an explicit `--tui` flag on the same commands, while keeping `--simple` as an accepted explicit alias for the new default mode
- Updated the simple-console help text so switching into the TUI is described as entering the TUI workbench instead of the old "default TUI" wording

### Added

- Added shared console-mode resolution logic plus tests to keep the default-mode behavior, restart flow, and conflicting `--simple` / `--tui` flag handling consistent

### Documentation

- Updated the Chinese and English README files, command references, advanced guides, and install-complete hints to explain that simple mode is now the default and TUI is opt-in

## v2.2.8 - 2026-04-04

### Changed

- Replaced the old runtime-console home page with a Clash-style dashboard that surfaces subscription usage, current node, TUN status, traffic, IP path, and common-site latency in one screen
- Simplified the top-level TUI layout into three tabs: Home, Proxy, and Subscription
- Removed the old bottom command bar and moved confirmations plus parameter entry into centered modal overlays
- Kept `R` refreshes on the current page instead of bouncing guide/info pages back to the home tab

### Added

- Added persistent subscription profiles via `proxy.profiles` and `current_profile`, so multiple URL/file subscriptions can be saved and switched cleanly
- Added simple-console subscription commands: `subscription`, `subscription add url|file <name> <value>`, and `subscription use <name>`
- Added richer left-side preview cards with "current summary" and "next step" hints for the focused workspace

### Documentation

- Updated the Chinese README and release notes for the dashboard redesign and subscription workspace flow

## v2.2.7 - 2026-04-03

### Fixed (Windows)

- **Chinese garbled output** — set the console to UTF-8 (code page 65001) and enable ANSI virtual terminal processing at startup, so all Chinese text renders correctly in cmd.exe, PowerShell, and Windows Terminal instead of appearing as mojibake
- **`gateway` not found after install** — `install.ps1` now also refreshes `$env:Path` in the current PowerShell session, so the command is usable immediately without reopening the terminal
- **Administrator detection** — replaced the fragile `net session` probe with a Windows token membership check, so admin detection no longer depends on optional services
- **`IsRunning()` locale bug** — replaced the GBK-unsafe `"没有"` / `"No tasks"` check with a locale-independent `"mihomo.exe"` presence check, so process detection works correctly on Chinese Windows
- **`IsIPForwardingEnabled()` locale bug** — replaced `netsh` output parsing (which emits Chinese text on Chinese Windows) with a direct registry read of `HKLM\...\IPEnableRouter`, making forwarding detection locale-independent
- **Default NIC detection** — Windows now resolves the active interface from the default route table instead of guessing the first active adapter, which avoids picking VPN / virtualization adapters by mistake
- **Auto-start implementation** — Windows `gateway service install` now installs an auto-start Task Scheduler task instead of trying to register the CLI as a native `sc.exe` service
- **Self-update on Windows** — `gateway update` now uses a detached updater script so the running `.exe` can be replaced and restarted correctly even while the current binary is locked
- **`/tmp/` path hardcoded** — `start` and `console` commands now use `os.TempDir()` for the log file path, which resolves to the correct temporary directory on Windows
- **Unix-only hints leaking into Windows** — runtime hints now use `gateway <sub>` / `Get-Content -Wait` on Windows instead of `sudo ...` and `tail -f ...`
- **`~\...` local config paths** — file-path expansion now supports Windows-style home shortcuts in setup and switching flows

## v2.2.6 - 2026-04-03

### Fixed

- When focus stays in the left navigation list, `←/→` no longer switches top tabs by mistake
- Added a short red boundary feedback in the navigation list, with an explicit `Esc` hint for returning to the top tab area
- Added a whole-page refresh pulse for `R`, so refreshes now feel visible instead of silent
- Stopped preview pages from repeating the same title and hint text in the right-side pane
- Enabled mouse-wheel scrolling when the main detail area is focused and content overflows
- Updated the `gateway install` completion screen to show both startup modes and the `console` re-entry commands

### Changed

- Turned the runtime console from a mostly read-only TUI into a real workbench with action pages for:
  - proxy source
  - TUN / local bypass
  - recommended rule switches
  - extension mode
  - residential proxy fields
- Brought the plain command mode closer to feature parity with the TUI, so core config changes can also be done with commands such as `proxy`, `tun`, `rule`, `extension`, and `chain`

### Documentation

- Updated the Chinese and English README files plus command reference docs for the new console workbenches and plain-command examples

## v2.2.5 - 2026-04-03

### Fixed

- Fixed Windows `gateway install` so it can extract the official `mihomo-windows-amd64-compatible-*.zip` package and rename the extracted kernel to local `mihomo.exe`
- Fixed Windows `gateway update` so it downloads `gateway-windows-amd64.exe` instead of a missing extensionless asset
- Re-verified the pinned official `mihomo v1.19.8` Windows asset URL and archive structure against the upstream MetaCubeX release

### Documentation

- Updated the Chinese and English READMEs plus release notes to clarify the Windows kernel download path

## v2.2.4 - 2026-04-03

### Fixed

- Reworked the runtime TUI so right-side pages clearly indicate whether they are info pages, action pages, or confirm pages
- Added real detail-area focus, so pressing Enter moves focus into the right pane and `Esc` returns to the left menu
- Turned the node entry into a node workbench with clearer action hints and built-in current-node latency testing via `T`
- Replaced the TUI device and extension pages with native TUI rendering for more consistent layout

### Documentation

- Updated the Chinese and English READMEs plus command docs to explain the new node-workbench and page-type flow

## v2.2.3 - 2026-04-03

### Fixed

- `R` in the runtime TUI now refreshes the current page instead of replacing it with a generic refresh message
- Added a short visual refresh pulse so refresh actions feel visible in the terminal

### Documentation

- Updated the Chinese and English READMEs plus command docs to describe the refresh behavior

## v2.2.2 - 2026-04-03

### Fixed

- Replaced the TUI `功能导航` page with native TUI rendering so it no longer picks up broken separator lines or duplicated section titles
- Added a real header focus state so `Esc` from the navigation list now returns to the top tabs as expected
- Made the top tab area visibly focusable, with `←/→` to switch sections and `↓ / Enter` to go back into the action list

### Documentation

- Updated the Chinese and English READMEs with clearer runtime-console controls
- Updated the command reference docs to explain the new header-focus navigation flow

## v2.2.1 - 2026-04-03

### Fixed

- Reworked the runtime TUI so navigation no longer re-runs heavy status probes on every arrow key press
- Removed the distracting pet panel and reduced redraw pressure with a lighter two-column layout
- Fixed TUI focus feedback so the active area is highlighted more clearly
- Replaced captured CLI output in the TUI status and config pages with native TUI rendering
- Added `gateway console` and `gateway console --simple` so users can reopen the workspace without restarting the gateway
- Improved plain-command help text and renamed the main node-switching entry to `nodes` while keeping `groups` compatible

### Changed

- `Q` in the TUI now asks for confirmation instead of exiting immediately
- `R` in the TUI now shows a visible refresh result and timestamp
- `config` inside the TUI now stays in the TUI first; `config open` is the explicit path to the full interactive config center
- Updated the Chinese and English READMEs plus command reference docs for the new console flow

## v2.2.0 - 2026-04-03

### Added

- Added a full-screen runtime TUI console after `gateway start`
- Added slash-command interaction and in-terminal strategy group / node switching
- Added `gateway skill` for AI-client skill discovery
- Added `gateway permission` to help configure passwordless control
- Added `runtime.tun.bypass_local` so the host machine can stay closer to direct access while LAN devices still use the gateway
- Added cached update notices on the CLI home page, start summary, and TUI console

### Changed

- Unified config structure into `proxy`, `runtime`, `rules`, and `extension`
- Kept backward compatibility for legacy flat config fields
- Moved rules into a structured rules system instead of hardcoded template blocks
- Improved chains / egress reporting to show entry node, normal exit, and residential exit more clearly
- Improved config-center summaries and config-path visibility
- Improved install and usage messaging for mainland China users

### Documentation

- Rewrote the main Chinese README
- Added `README_EN.md`
- Expanded command and advanced guides
- Added release notes and release-process documentation
