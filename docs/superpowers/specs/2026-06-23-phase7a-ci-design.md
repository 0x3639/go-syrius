# Phase 7a — CI foundation design

**Date:** 2026-06-23
**Branch:** `phase-7a-ci`
**Scope:** A GitHub Actions PR gate (test + lint + security + per-OS build) — the first sub-project of Phase 7 (plan.md §3). Ledger (Phase 6) is deferred; the remaining Phase 7 sub-projects (release matrix, signing, auto-update, a11y/telemetry, security pass + docs) are separate, later cycles.

## Context

The repo currently has **no CI** (`.github/workflows` is empty). CLAUDE.md's security rules already promise "CI runs `govulncheck` and `gosec`; deps pinned with `go.sum`" — this sub-project makes that real. With Phase 5 (NoM features) complete and merged, the highest-leverage next step is a gate that catches regressions and enforces the security scanners on every change, before any release/packaging work (7b+). It needs no secrets or certificates, so it is cleanly self-contained.

Toolchain (from the repo): Go 1.24.0 (`go.mod`), Wails v2.10.1 (`go.mod`), pnpm 10.17.1 (`frontend/package.json` `packageManager`), `frontend/pnpm-lock.yaml` present. Frontend scripts: `check` (svelte-check), `test` (vitest run), `build` (vite build).

## Architecture

One workflow, `.github/workflows/ci.yml`, triggered on `pull_request` targeting `main` and `push` to `main`. It defines three parallel jobs that separate OS-agnostic checks from the per-OS build/test matrix; the gate is green only if all jobs pass.

Two environment facts shape the workflow:
- **Integration tests are excluded.** The live-node smokes in `internal/spike/*_integration_test.go` are behind `//go:build integration` and require `ZNN_NODE_URL`. Plain `go test ./...` does not compile or run them, so CI must NOT pass `-tags integration`.
- **No `GOWORK=off` in CI.** `GOWORK=off` is a local-only workaround for a parent `go.work` on the developer's machine. CI checks out the repo standalone (no parent workspace), so the flag is unnecessary and must not be added.

Requiring the check as a merge gate is a one-time GitHub branch-protection setting (repo Settings → Branches) — out of scope for this workflow file; noted as a follow-up the repo owner performs.

## Jobs

### Job `frontend` (ubuntu-latest, runs once — OS-agnostic)
- Checkout.
- Enable corepack; pin pnpm 10.17.1 (honors `packageManager`).
- `actions/setup-node` with `cache: pnpm` (cache keyed on `frontend/pnpm-lock.yaml`).
- `pnpm install --frozen-lockfile` (working dir `frontend/`).
- `pnpm run check` — svelte-check, must report 0 errors.
- `pnpm test` — vitest run (currently 40 tests / 19 files).

### Job `security` (ubuntu-latest, runs once — Go-source analysis)
- Checkout; `actions/setup-go` with `go-version-file: go.mod` (module cache on).
- **govulncheck (hard-fail, with allowlist):** `go install golang.org/x/vuln/cmd/govulncheck@latest` then a gate script that runs `govulncheck ./...` and fails on any code-affecting vuln ID **except** an allowlist. As of 2026-06-23 a clean scan surfaces 19 code-affecting vulns: **14 standard-library** (all in the Go toolchain, fixed by patches up to go1.25.11) and **5 in `github.com/ethereum/go-ethereum@v1.13.15`** (devp2p/RLPx p2p DoS/handshake; reachable only via the opt-in embedded node; no key/fund exposure). Resolution: bump the Go toolchain to **go1.25.11** (clears all 14 stdlib vulns — verified), and **allowlist the 5 go-ethereum IDs** (GO-2026-4314/-4315/-4507/-4508/-4511) as deferred pending go-zenon's upstream libp2p migration (they cannot be bumped without breaking go-zenon's build — verified against go-ethereum v1.16.8/v1.16.9/v1.17.0, and no newer go-zenon exists). The gate still hard-fails on any *new* vuln. The `@latest` pin is acceptable for a scanner (we want new-CVE signal).
- **gosec (hard-fail, tuned):** `go install github.com/securego/gosec/v2/cmd/gosec@latest` then `gosec -conf .gosec.json ./...`. A committed `.gosec.json` carries any rule excludes; code-level suppressions use inline `// #nosec G### <justification>`.

### Job `build-test` (matrix: ubuntu-latest, macos-latest, windows-latest)
- Checkout; `actions/setup-go` (cache) + `setup-node` + corepack pnpm 10.17.1.
- **Linux only:** `sudo apt-get update && sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev` (matches Wails 2.10 on ubuntu 24.04). macOS/Windows runners already have the needed webview.
- `go vet ./...`
- `go test ./...` (excludes integration-tagged tests by default).
- `go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1`.
- `wails build` — real packaging; wails.json's `frontend:install: pnpm install` drives the frontend build, so pnpm must be on PATH.

## gosec tuning

gosec with defaults will very likely flag false positives in a wallet — notably G304 (file path from variable) in keystore/config path handling and G104 (unhandled errors) in `defer x.Close()`. The gosec task in the plan includes a **triage pass**: run gosec locally, categorize each finding (real → fix the code; false positive → inline `// #nosec G### <reason>` or a `.gosec.json` exclude with a comment), and re-run until clean — so the hard-fail gate lands green on first CI run. `govulncheck` rarely needs tuning; if it fails, it is reporting a real vulnerable dependency to bump.

## Caching

`actions/setup-go` built-in module cache; pnpm store cache via `setup-node` `cache: pnpm`. Keeps repeat runs fast without changing gate semantics.

## Verification

CI only executes on GitHub, so correctness is established in two stages:
1. **Local dry-run of every gate command** on this codebase, confirming each passes before the workflow is trusted: `go vet ./...`, `go test ./...`, (`cd frontend`) `pnpm run check` + `pnpm test`, `govulncheck ./...`, `gosec -conf .gosec.json ./...`, and `wails build`. Any failure is fixed (or, for gosec, tuned) as part of this sub-project.
2. **Live CI run:** push the branch, open a PR, and confirm all jobs — including each `build-test` matrix leg (Linux/macOS/Windows) — go green in the Actions tab. Iterate on YAML/dep issues (e.g. the Linux WebKit package name) until green.

## Out of scope (deferred)

- Branch-protection / required-status-check enabling (repo Settings; owner action).
- Release artifacts, cross-platform installers, code signing/notarization (7b/7c).
- Auto-update (7d), accessibility/keyboard-nav/telemetry (7e), threat model + CONTRIBUTING/build docs (7f).
- CI status badge in a README (optional polish; no top-level README exists yet).

## File structure

- `.github/workflows/ci.yml` (new) — the three-job workflow above.
- `.gosec.json` (new) — gosec config (rule excludes, if any, with comments).
- Inline `// #nosec` suppressions in Go source only where a gosec finding is a justified false positive.
- `plan.md` — Phase 6 marked deferred (already noted on this branch).
