# Changelog

All notable changes to this project will be documented here.

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
