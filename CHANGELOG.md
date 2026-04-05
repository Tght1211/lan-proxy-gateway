# Changelog

All notable changes to this project will be documented here.

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
