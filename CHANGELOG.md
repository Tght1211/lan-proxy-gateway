# Changelog

All notable changes to this project will be documented here.

## v2.2.7 - 2026-04-03

### Fixed (Windows)

- **Chinese garbled output** — set the console to UTF-8 (code page 65001) and enable ANSI virtual terminal processing at startup, so all Chinese text renders correctly in cmd.exe, PowerShell, and Windows Terminal instead of appearing as mojibake
- **`gateway` not found after install** — `install.ps1` now also refreshes `$env:Path` in the current PowerShell session, so the command is usable immediately without reopening the terminal
- **`IsRunning()` locale bug** — replaced the GBK-unsafe `"没有"` / `"No tasks"` check with a locale-independent `"mihomo.exe"` presence check, so process detection works correctly on Chinese Windows
- **`IsIPForwardingEnabled()` locale bug** — replaced `netsh` output parsing (which emits Chinese text on Chinese Windows) with a direct registry read of `HKLM\...\IPEnableRouter`, making forwarding detection locale-independent
- **`/tmp/` path hardcoded** — `start` and `console` commands now use `os.TempDir()` for the log file path, which resolves to the correct temporary directory on Windows
- **`sudo` in all command hints** — every hint message now uses a runtime-aware helper that outputs `gateway <sub>` on Windows and `sudo gateway <sub>` on Unix, across all commands: `start`, `stop`, `restart`, `console`, `update`, `tun`, `chains`, `switch`, `status`

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
