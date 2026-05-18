# Codebase cleanup — design

- **Status:** Draft, awaiting user review
- **Date:** 2026-05-19
- **Posture:** Conservative — keep compat paths where files have public references
- **Hard invariant:** No functional behavior change. Every commit either touches build/docs only, or is a pure refactor protected by existing tests. The branch is rejected before merge if `go test ./...` or any verification step regresses.
- **Out of scope:** Splitting `internal/console/console.go` (2042 lines) and `internal/app/webui_adapter.go` (967 lines). Each is its own refactor epic.

## Context

User reported the project structure felt "脏" / "乱" while finishing the `gateway update` fix. Scoping conversation classified the cleanup into four categories, all in scope:

1. Local working-tree clutter (everything gitignored).
2. Root-directory tracked clutter (scripts and example files).
3. Tracked-but-dead code (`legacy/` directory).
4. Active Go code quality (SonarQube findings in `cmd/update.go`).

## Constraints discovered during exploration

- `install.sh` and `install.ps1` are referenced from `README.md`, `README_EN.md`, and the release notes for v2.2.7 / v3.3.0 / v3.3.1 / v3.4.0 / v3.4.1 / v3.4.4 via the public install URL `https://raw.githubusercontent.com/Tght1211/lan-proxy-gateway/main/install.sh`. They must stay at repo root.
- `dev.sh` / `dev.ps1` are documented in `README_EN.md` as `./dev.sh build|test|run|start`. Keep at root for developer ergonomics.
- `install-mihomo.sh`, `download-mihomo.sh`, `script-demo.js` have zero references in `*.go`, `*.md`, or `*.sh` (verified by grep). Safe to relocate without wrappers.

## Plan — branch `chore/housekeeping`, four atomic commits

### Commit 1 — `chore(make): expand clean target, wipe local cruft`

Existing `clean:` only removes `dist/` and the `gateway` binary. Expand to cover the rest of the gitignored cruft we accumulated locally:

```make
clean:
	rm -rf $(DIST_DIR)/ $(BINARY)
	rm -rf .tmp/ .cache/ .try/
	rm -f logs/*.log
	find . -name '.DS_Store' -delete
```

Then run `make clean` once locally. Diff = `Makefile` only; the deletions affect only gitignored paths so git itself sees no file removals.

`handoff_unfinished_tasks.txt` is intentionally **not** removed by `clean`: it is a hand-off / recovery document (gitignored but human-meaningful), not a build artifact. The user can delete it manually when no longer needed.

### Commit 2 — `chore(legacy): document archive status`

Add `legacy/README.md`:

- `legacy/v1/` is the standalone pre-refactor Go module.
- The shell scripts at `legacy/` root are the original shell-only implementation (predates the Go binary).
- Kept for git-history reference. Not maintained. Do not import or invoke.

No other changes — `legacy/` keeps its contents.

### Commit 3 — `refactor: relocate stand-alone helper scripts`

Moves:

| From | To | Compat wrapper at original path? |
|---|---|---|
| `install-mihomo.sh` | `scripts/install-mihomo.sh` | Yes — 2-line shell wrapper |
| `download-mihomo.sh` | `scripts/download-mihomo.sh` | Yes — 2-line shell wrapper |
| `script-demo.js` | `examples/extension.js` | No (no callers; rename also intentional) |

Wrapper content (executable, retains original `#!/usr/bin/env bash` shebang for portability):

```bash
#!/usr/bin/env bash
# Moved to scripts/. This shim is kept for compatibility with any
# documentation or muscle-memory still pointing at the repo root.
exec "$(dirname "$0")/scripts/$(basename "$0")" "$@"
```

Pre-move verification (already done while drafting this spec): `grep -rn 'install-mihomo\|download-mihomo\|script-demo' .github/ Makefile scripts/ embed/ internal/ cmd/ main.go README*.md docs/` → zero hits. The wrappers are belt-and-suspenders only.

Stay at root (external/documented references): `install.sh`, `install.ps1`, `dev.sh`, `dev.ps1`.

`scripts/` already exists with `rebuild-tag-assets.sh` and `sync-release-notes.sh`; the new arrivals fit the existing convention. `examples/` will be created.

### Commit 4 — `refactor(update): satisfy lint, extract helpers`

Pure refactor of `cmd/update.go`. No behavior change. Addresses the five SonarQube findings inherited from the prior fix/refactor commits:

1. **Repeated string literals (S1192).** Define module-level constants:

   ```go
   const (
       updateUserAgentHeader = "User-Agent"
       updateUserAgentValue  = "lan-proxy-gateway"
       updateErrCandidateFmt = "%s: %v"
   )
   ```

   Verified exact sites to replace:
   - `"%s: %v"` at lines 247, 276, 412.
   - `"User-Agent"` and `"lan-proxy-gateway"` together at lines 288, 334, 402.

2. **Cognitive complexity (S3776).** Extract two helpers:

   - `prepareUpdateBinary(ctx, requested) (tag, tmpPath string, err error)` — pulls the `resolveUpdateTag` → `gatewayReleaseAsset` → `downloadUpdateAsset` → `chmod` → `--version` print block out of `runUpdate`. Brings `runUpdate` from 18 → ≤15.
   - `restartGatewayAfterUpdate(ctx, a, wasRunning, localDNSWasLoopback)` — pulls the tail-end `a.Start` + `SetLocalDNSToLoopback` block out of `installUpdateBinary`. Brings `installUpdateBinary` from 24 → ≤15.

`cmd/update_test.go` continues to cover behavior (all functions extracted have no test-visible signature change).

## Verification

Per commit:

- `go build ./...` clean.
- `go test ./...` clean.

Commit-specific checks:

- **C1:** run `make clean` against a dirty tree, confirm it removes the listed paths and leaves tracked files alone.
- **C2:** read-only — no checks beyond the build.
- **C3:**
  - `bash -n scripts/install-mihomo.sh`, `bash -n scripts/download-mihomo.sh`, `bash -n install-mihomo.sh`, `bash -n download-mihomo.sh` — syntax-check both targets and both wrappers.
  - `./install-mihomo.sh --help 2>&1 | head -2` and `./scripts/install-mihomo.sh --help 2>&1 | head -2` should produce identical output (wrapper invariant). Same for `download-mihomo.sh`.
  - Confirm `install.sh` and `install.ps1` are still at repo root with original content (unchanged).
  - `git grep -n 'install-mihomo\|download-mihomo\|script-demo'` shows references **only** inside `scripts/`, the wrappers, and `examples/extension.js`.
- **C4:** `go test ./cmd/...` green. `gateway update --help` still parses (cobra hasn't lost any flags). `golangci-lint` (if available) clean on `cmd/update.go`.

Whole-branch pre-merge:

- `go build ./...` and `go test ./...`.
- `gateway --version` runs from a fresh build of the branch HEAD.

## Rollback

Each commit is atomic and independently revertible.

- `git revert <sha>` on any single commit restores the prior state for that scope alone.
- The branch `chore/housekeeping` lives separately until merge; abandoning the entire effort is `git branch -D chore/housekeeping`.
- The most failure-prone commit is C3 (moves files). If any downstream tooling we missed depends on the moved scripts, `git revert` restores them at root within seconds.

## Future work (explicitly out of scope here)

- Split `internal/console/console.go` (2042 lines) into per-tab files.
- Split `internal/app/webui_adapter.go` (967 lines).
- Decide eventual fate of `legacy/` — delete after the v1 shell installation path is fully retired in user-facing docs.
- Address any cognitive-complexity hotspots SonarQube flags outside `cmd/update.go`.
