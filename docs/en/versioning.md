# Versioning Notes

[Chinese version](../versioning.md)

## Current Strategy

The public version line stays as it is.

That means:

- already-published `v1.x` and `v2.x` releases are not renumbered
- historical tags are not moved
- the focus is on improving release titles, descriptions, and asset completeness

This avoids breaking:

- upgrade paths for existing users
- public GitHub release links
- current version checks in `gateway update`

## Historical Versions

### v1.0.0

Positioning: initial public release.  
Focus: establish the main LAN transparent gateway flow, device setup docs, install scripts, and auto-release basics.

### v1.1.0

Positioning: install and stability improvements.  
Focus: one-command install, China-friendly mirror fallback, automatic `mihomo` download, health checks, self-healing, and basic management.

### v1.1.1

Positioning: install and upgrade fixes.  
Focus: repair the install and release upgrade path.

### v1.2.0

Positioning: first release of chains mode.  
Focus: add a residential proxy chain aimed at AI usage scenarios.

### v1.3.0

Positioning: proxy mode expansion.  
Focus: add switching between multiple proxy modes.

### v2.0.0

Positioning: extension script architecture.  
Focus: introduce global extension scripts so advanced customization is no longer hardcoded in templates.

### v2.1.0

Positioning: TUN control commands.  
Focus: make LAN transparent proxy toggles more direct.

### v2.1.1

Positioning: TUN startup experience fixes.  
Focus: improve TUN startup output and behavior.

### v2.2.0

Positioning: unified console and config architecture.  
Focus: runtime TUI, proxy group switching, unified config areas, fuller release assets, and a clearer release-note system.

### v2.2.1

Positioning: console experience fixes.  
Focus: fix TUI sluggishness, clean up status and config-summary rendering, add a `console` re-entry flow, and tighten the handoff between plain-command mode and TUI mode.

### v2.2.2

Positioning: TUI focus and navigation polish.  
Focus: fix the `guide` page layout, add proper header-focus handoff, and make the top tabs and left action list work together more cleanly.

### v2.2.3

Positioning: TUI refresh feedback polish.  
Focus: make `R` refresh the current page and add a visible but restrained refresh cue.

### v2.2.4

Positioning: TUI interaction-structure polish.  
Focus: clearly separate info pages from action pages, add real right-pane focus, and merge node switching with built-in latency testing into a node workbench.

## Rules for Future Versions

### Patch

`vX.Y.Z`

Use for:

- bug fixes
- documentation corrections
- CLI copy refinements
- backfilling release descriptions

### Minor

`vX.Y.0`

Use for:

- new backward-compatible features
- new command entry points or console capabilities
- rule system upgrades
- improvements to chains, skills, or LAN sharing

### Major

`vX.0.0`

Use only when:

- the config structure changes in a breaking way
- old command behavior is no longer compatible
- the runtime model changes substantially

## How Historical Releases Are Maintained

To avoid disrupting existing users, historical versions are handled this way:

1. keep published tag targets unchanged
2. improve release titles, descriptions, and missing asset notes
3. rebuild from the original tag only when missing binaries need to be uploaded

This keeps the history easier to understand without breaking upgrade behavior.
