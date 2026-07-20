# go-syrius Security Audit — 2026-07-20

Comprehensive, adversarial security audit of the go-syrius non-custodial Zenon
desktop wallet (Go + Wails v2 + Vue 3), performed against the exact repository
state checked out at the time of review. This is an audit, not a remediation: no
production code was modified. The only file added is this report.

---

## 1. Executive summary

go-syrius is **unusually well hardened for its maturity**. Across an independent
three-track review (Go wallet/transaction/NoM backend; Wails/frontend/WalletConnect
boundary; node/persistence/concurrency/supply-chain) plus lead-auditor validation
of the core signing path, **no Critical or High severity vulnerability was found**
in the funds-critical code. The central invariant — *the WebView is untrusted for
key material; Go exclusively builds, PoWs, signs, and publishes* — holds
everywhere it was tested. Confirm-what-you-sign is enforced on the built block via
`assertMatches`; the mainnet opt-in is re-checked immediately before broadcast
against the **built block's** chain id; publication is serialized; and the
WalletConnect duplicate-protection journal persists the signed block **before**
broadcast with fail-closed capacity and hash-integrity-checked rebroadcast.

The eight findings from the prior WalletConnect audit (`docs/walletconnect-audit-2026-07-17.md`)
were **independently re-verified as remediated in the current code** (WC-01…WC-08),
not merely trusted.

Thirteen findings remain, all Medium or below. The two Medium items are a
defense-in-depth WebView CSP gap (GS-01) and the unsigned/un-notarized release
pipeline (GS-02) — the latter is an explicitly-deferred Phase 7 release risk, not a
code vulnerability. The Low/Informational items are defense-in-depth, availability,
display-accuracy, and supply-chain-pinning hardening.

**Mainnet/release recommendation: Conditionally acceptable** (see §2).

---

## 2. Mainnet / release recommendation

**Conditionally acceptable for the reviewed scope.**

- The signing, key-handling, mainnet-authorization, chain-binding, amount-integrity,
  and WalletConnect duplicate-protection paths are sound and well-tested. There is
  **no Critical/High blocker** in the code that signs and publishes funds.
- **Condition for a broad mainnet / WalletConnect release:** remediate **GS-01**
  (no effective CSP). The WebView exposes the full Wails binding surface and stores
  the WalletConnect relay keypair in `localStorage`; WalletConnect is a mainnet-only
  feature that renders attacker-influenced metadata. Today there is **no demonstrated
  XSS sink** (verified — see §10), so this is a latent blast-radius gap, not an
  active exploit, but it is the primary remaining WebView hardening item.
- **Condition for a signed production distribution:** remediate **GS-02**
  (code-sign + notarize; publish an out-of-band authenticated checksum) and **GS-06**
  (pin CI/release actions to immutable SHAs). These are distribution/supply-chain
  risks, correctly classified as Phase 7b–7f work, not hidden code bugs.

The wallet already gates mainnet sends behind an explicit opt-in
(`AllowMainnetSend`) re-checked at broadcast, and WalletConnect is hard-restricted
to `zenon:1` Bridge `WrapToken`/`RedeemUnwrap`. Nothing in this audit found a path
that signs an incorrect or unauthorized mainnet transaction.

---

## 3. Exact commit, branch, and working-tree state

| Item | Value |
|---|---|
| Commit | `de55384dfd8db4d18e68581194779c53c31edc1d` |
| Branch | `main` |
| `git status --short` | `?? .agents/` and `?? .claude/` only (untracked agent-tooling dirs; **no modified tracked files**) |
| Go toolchain | `go1.25.4 darwin/arm64` (go.mod declares `go 1.25.12`; `GOTOOLCHAIN=auto` used per AGENTS.md) |
| Node | `v22.12.0` |
| pnpm | `10.14.0` (package.json `packageManager` declares `pnpm@10.17.1` — minor mismatch; all frontend commands still ran on the existing lockfile/modules) |
| Wails CLI | not in local PATH (not required for the audit commands; CI builds with `wails@v2.12.0`) |

Key direct dependencies (from `go.mod` / `frontend/package.json`):

| Dependency | Version | Notes |
|---|---|---|
| `github.com/0x3639/znn-sdk-go` | `v0.2.1` | author-owned SDK; pinned tag. **AGENTS.md still says v0.1.19 — stale doc.** |
| `github.com/zenon-network/go-zenon` | `replace`→`github.com/0x3639/go-zenon v0.0.0-20260615011802-81c247408859` | fork pinned to commit `81c247408859`; go.sum H1-verified |
| `github.com/wailsapp/wails/v2` | `v2.12.0` | |
| `github.com/ethereum/go-ethereum` | `v1.13.15` (indirect) | 5 govulncheck IDs allowlisted (see §11) |
| `@walletconnect/sign-client` | `2.23.9` (exact) | |
| `nom-ui` | `github:digitalSloth/nom-ui#63f755a…` | git dep pinned to immutable commit SHA |
| `vue` / `pinia` / `vue-router` | `^3.4` / `^2.2` / `^4.6.4` | |
| `lottie-web` | `^5.13.0` | uses `eval` in its expression renderer (see GS-13) |

---

## 4. Scope, methodology, tools, and limitations

**Scope.** The full working tree at the commit above, including the binding
boundary (`app/`), internal packages (`internal/embeddednode`, `internal/governance`,
`internal/compat`), the Vue frontend (`frontend/src`), generated Wails bindings
(`frontend/wailsjs`), persistence files, CI/release workflows, and dependency
manifests. `plan.md` was treated as the authoritative specification; `AGENTS.md` and
the prior audits as historical context (independently verified, not trusted).

**Binding surface enumerated.** `app/app.go:48-50` binds exactly five structs:
`Config`, `Wallet`, `Node`, `Tx`, `Nom`. Every exported method on those structs is
exposed to the WebView. The frontend agent programmatically cross-checked every
`frontend/wailsjs/go/app/*.d.ts` function against the Go receiver methods — **zero
missing/extra**. The intentional secret-crossing methods are `WalletService.RevealMnemonic`
(password-gated, session-re-checked) and `WalletService.GenerateMnemonic` (create-time
only); both are narrowly scoped.

**Methodology.** Three parallel reviewers (general agents) covered the three tracks,
returning findings in the required format; the lead auditor independently re-read the
core signing/key/WC-journal/persistence files (`tx_service.go`, `wallet_service.go`,
`walletconnect.go`, `walletconnect_journal.go`, `node_service.go`, `config_service.go`,
`tx_effect.go`, `decimals.go`, `dto.go`, `wallet_manifest.go`, `index.html`, `main.go`,
`app.go`) and validated/deduplicated every finding, confirming the cited file:line and
control-flow argument before inclusion.

**Tools run (all on the audited worktree):**

| Command | Result |
|---|---|
| `GOWORK=off GOTOOLCHAIN=auto go mod verify` | **all modules verified** |
| `GOWORK=off GOTOOLCHAIN=auto go vet ./...` | **clean** (exit 0) |
| `GOWORK=off GOTOOLCHAIN=auto go test ./...` | **all packages pass** |
| `GOWORK=off GOTOOLCHAIN=auto go test -race ./...` | **all pass, no data races** (benign `ld` LC_DYSYMTAB + gopsutil/IOKit cgo deprecation warnings only) |
| `pnpm run typecheck` (vue-tsc) | **clean** |
| `pnpm test` (vitest) | **71 files, 398 tests pass** (one non-fatal jsdom `canvas.toDataURL` warning in a passing QR test) |
| `pnpm run build` (Vite) | **builds** (large-chunk + lottie `eval` warnings only) |
| `pnpm audit --prod` | **no known vulnerabilities** |

**Limitations (recorded, not silently omitted):**

- `gosec` and `govulncheck` are **not installed** locally, so `scripts/govulncheck-gate.sh`
  and `gosec -conf .gosec.json ./...` could **not** be reproduced here. Per the audit's
  no-network/no-install constraint they were not installed; **CI runs both**
  (`.github/workflows/ci.yml:50-57`). The govulncheck allowlist was reviewed by reading
  `.github/govulncheck-allow.txt` and the gate script (§11).
- The `wails` CLI is not installed, so a **production `wails build` smoke test could not
  be run** — relevant to verifying CSP tightening (GS-01) and confirming devtools-off in
  prod builds. CI's build-test matrix compiles the app on ubuntu/macOS/windows.
- **No live-node or integration tests were run** (per constraints). Integration tests are
  behind `//go:build integration` and self-skip without a node.
- The pinned **SDK (`znn-sdk-go`) and go-zenon fork were NOT independently source-audited**
  for their crypto/ABI/PoW/keystore internals — only their *usage* and the wallet's
  `assertMatches`/`decodeContractCall` layer above them (see §13).
- Frontend commands ran on the **existing** `node_modules`; `pnpm install --frozen-lockfile`
  was not re-executed (lockfile not modified, per constraints).

---

## 5. Architecture and trust-boundary summary

Single Wails binary. The WebView (Vue 3 + Pinia) sends *intent*; five Go services do
all security-sensitive work:

```
WebView (untrusted for key material)
   │  Wails bindings: Config / Wallet / Node / Tx / Nom  (app/app.go:48-50)
   ▼
Go backend (app/) ── znn-sdk-go (wallet/pow/rpc/contracts) ── go-zenon (types/ABI/embedded node)
   │  WebSocket ws://… | wss://…
   ▼
Remote / Local / Embedded node
```

- **Secrets never cross the binding** except password-gated `RevealMnemonic` and
  create-time `GenerateMnemonic`. `signingKeyPair` (`wallet_service.go:712`) and the
  mnemonic stay backend-only; the transient SDK keystore's derived seed/entropy are
  zeroed after each derivation (`wallet_service.go:731-734`).
- **Prepare-then-publish**: `PrepareSend`/`prepareCall*` build + hold an un-PoW'd
  template and return a preview; `ConfirmPublish(holdId)` PoWs, signs, re-asserts the
  effect on the **built** block, re-checks chain + mainnet opt-in, then publishes
  (`tx_service.go:266-426`). A monotonic, non-zero `holdId` identity gate fails closed.
- **WalletConnect**: Go reconstructs a clean Bridge block from a narrow intent DTO
  (`walletconnect.go:36-118`) — no dapp-supplied frontier/PoW/hash/pubkey/signature can
  enter signing — and journals the signed block before broadcast
  (`walletconnect_journal.go`) for exactly-once semantics across crashes/restarts.
- **Persistence**: settings + WC journal use atomic temp-file + `fsync` + rename in a
  `0700` data dir; keystores are written `0600`.

---

## 6. Threat model and security invariants

**Invariants treated as non-negotiable** (and the audit's checklist): (1) no
keys/mnemonics/decrypted keystores in the WebView; (2) signing only in Go; (3) all
frontend/WC/node/persisted/Wails input untrusted; (4) every state-changing binding
validates args + wallet/node/network state; (5) confirmation shows the exact built-block
effect; (6) approved block == signed block == published block; (7) mainnet writes need
explicit authorization re-checked immediately before broadcast; (8) lock/account/node
switch/cancel/timeout/restart never cause the wrong account/chain/tx to be signed;
(9) amounts stay exact base-unit integers; (10) keystore/crypto behavior matches the
pinned Zenon implementations; (11) no secrets in logs/errors/events/telemetry/persisted
state; (12) publication/retry never produces duplicate financial effects.

**Attackers/failure conditions analyzed:** malicious/compromised remote RPC node;
malicious WalletConnect dapp/relay/peer; malicious frontend content/XSS/compromised
dependency; direct invocation of Wails bindings bypassing the UI; crafted
keystore/config/contact/journal/RPC/ABI/token-metadata/URL/account-block input; local
unprivileged user inspecting/modifying app files; symlink/path-traversal/unsafe-perm/
partial-write/crash-recovery; supply-chain compromise (Go/npm/Actions/release/git deps);
TOCTOU races (lock/unlock, account/node change, mainnet consent, PoW, auto-receive,
publication); crash/timeout/lost RPC/relay failure/duplicate request/replay/restart;
resource exhaustion from oversized remote data; clipboard/QR/external-link/icon/URL
attacks.

**Explicitly OUT of scope / outside the threat model:**
- A **fully compromised WebView process** (e.g. successful XSS) is treated as game-over
  for *that session* regardless of these controls — it controls the UI the user trusts
  and can call the bindings directly. The wallet's defense against this is *preventing*
  injection (no XSS sinks — verified §10; CSP — GS-01) and keeping keys in Go (so even a
  compromised WebView cannot exfiltrate the seed, only request signatures the user is
  tricked into approving). The WC journal is designed for the **dapp/relay/crash**
  duplication threat, not a hostile local UI (see GS-01 discussion).
- Physical access / root / kernel-level attackers; OS-level keyloggers/screen capture.
- The internal correctness of the pinned `znn-sdk-go` / `go-zenon` crypto primitives
  (trusted; usage audited — §13).
- Ledger hardware signing (Phase 6, deferred, not built).

---

## 7. Findings table (sorted by severity)

| ID | Title | Severity | Confidence | CWE | Area |
|---|---|---|---|---|---|
| GS-01 | No effective Content-Security-Policy (no `script-src`/`default-src`/`connect-src`) | **Medium** | High (condition); n/a (no current exploit) | CWE-79 / CWE-1021 | Frontend/WebView |
| GS-02 | Release binaries unsigned/un-notarized; checksums unauthenticated | **Medium** | High | CWE-494 | Supply chain/release |
| GS-03 | Unbounded node-driven pagination loops → client DoS (hang/OOM) | Low | High | CWE-835 / CWE-400 | Node/RPC |
| GS-04 | First-party NoM writes confirm material ABI params from form input, not the built block | Low | High | CWE-345 / CWE-451 | Tx/NoM |
| GS-05 | Wallet manifest: unsynchronized read-modify-write + non-fsync fixed temp file | Low | High | CWE-362 / CWE-667 | Persistence |
| GS-06 | GitHub Actions pinned to mutable major tags (not immutable SHAs) | Low–Medium | High | CWE-829 / CWE-1357 | Supply chain/CI |
| GS-07 | Node-supplied token decimals not range-bounded before display | Low | High | CWE-807 | Node/display |
| GS-08 | Stale prior-session account data displayed between unlock and first refresh | Low | High | CWE-672 | Frontend |
| GS-09 | WalletConnect request-expiry cancellation keys on numeric id only (fail-safe DoS) | Low | Medium | CWE-697 | WalletConnect |
| GS-10 | CI security scanners installed from `@latest`/floating refs | Low | High | CWE-829 / CWE-1357 | Supply chain/CI |
| GS-11 | `SetNodeURL` persists userinfo/query/fragment/path; only the WC path strips them | Informational | High | CWE-200 / CWE-522 | Node/persistence |
| GS-12 | `toBase` silently normalizes malformed decimal input | Informational | High | CWE-20 | Frontend |
| GS-13 | Dependency notes: `lottie-web` eval blocks strict CSP; `nom-ui` git dep lacks attestation | Informational | High | CWE-1104 | Supply chain |

**Totals: 0 Critical, 0 High, 2 Medium, 8 Low, 3 Informational.**

---

## 8. Full finding details

### GS-01 — No effective Content-Security-Policy
- **Severity:** Medium | **Confidence:** High that the condition exists; **no current exploit demonstrated** | **CWE-79 / CWE-1021**
- **Location:** `frontend/index.html:20-23`
- **Invariant:** WebView defense-in-depth (limit blast radius of any future injection).
- **Detail:** The policy is only `object-src 'none'; base-uri 'self'; frame-ancestors 'none'`. There is no `default-src`/`script-src`/`connect-src`/`img-src`, so script and connections fall open. `frame-ancestors` is inert in a `<meta>` policy (correctly noted in the in-file comment). The WebView exposes the entire Wails binding surface and SignClient persists its relay keypair + sessions in WebView `localStorage`, so any future injection primitive would get code execution with the app's whole IPC surface. Tightening is currently blocked by `lottie-web`'s `eval` (GS-13): a `script-src` without `'unsafe-eval'` breaks the splash; with it the anti-XSS value is largely void.
- **Attacker/prereqs:** Requires first introducing an XSS sink; **none exists today** (verified — §10). This is the deliberate, documented state from `docs/security-audit-hardening-2026-07-19.md`, left open pending a production `wails build` smoke test of the WC relay + price feed + Wails runtime.
- **Impact:** Latent. If any XSS sink is ever introduced (e.g. via a dependency or a careless `v-html`), unrestricted script/`fetch` with full IPC access; could trick the user into approving signatures (keys themselves remain in Go and cannot be exfiltrated).
- **Why guards don't prevent it:** Vue auto-escaping is the only layer; there is no second layer.
- **Repro:** n/a (latent); `wails build` then inject a probe via devtools to show unrestricted `fetch`/script.
- **Remediation:** Remove/replace lottie's expression-eval path (a static logo animation needs no expression renderer), then ship a strict CSP (`script-src 'self'`; explicit `connect-src` for the WC relay + Verify/explorer APIs + `api.zenon.info`; `img-src 'self' data:`), validated against a production build as the in-file comment prescribes. Longer-term, proxy the price feed + WalletConnect through Go to remove direct WebView→internet calls.
- **Regression test:** CI job that production-builds and loads the bundle under a CSP-violation listener; assert zero violations and that an injected inline script does not execute.

### GS-02 — Release binaries unsigned/un-notarized; checksums unauthenticated
- **Severity:** Medium (release/distribution risk, **not** a code vuln) | **Confidence:** High | **CWE-494**
- **Location:** `.github/workflows/release.yml` (header comment `:5-7`, asset/checksum body `:105-132`); Phase 7b–7f.
- **Detail:** Binaries are explicitly unsigned/un-notarized (Gatekeeper/SmartScreen warnings; users told to bypass). `SHA256SUMS` is generated **inside the same workflow** that builds the assets and is **not signed/attested** — a pipeline compromise yields both assets and matching checksums.
- **Impact:** A user cannot cryptographically distinguish a genuine release from a tampered one; a release-pipeline compromise distributes malicious binaries with valid-looking checksums.
- **Remediation:** Code-sign + notarize (macOS/Windows); publish an out-of-band signed checksum (Sigstore/cosign or a maintainer-signed `SHA256SUMS.asc`); consider SLSA provenance attestation.
- **Regression test:** Release pipeline check that asserts a detached signature/attestation exists and verifies before publishing.

### GS-03 — Unbounded node-driven pagination loops → client DoS
- **Severity:** Low | **Confidence:** High | **CWE-835 / CWE-400**
- **Locations:** `app/nom_service.go:342-354` (`GetPillarList`), `:866-881` (`GetMyTokens`); `app/nom_accelerator.go:436-446` (`GetVotableItems`), `:516-527` (`GetMyProjects`).
- **Invariant:** Reads against an untrusted node must be bounded.
- **Trace:** Each loop is `for { list := api.GetAll(page, size); append(...); if len(out) >= list.Count || len(list.List)==0 { break }; pageIndex++ }`. Exit depends entirely on node-supplied `Count`/`List`. A malicious node returns `Count = 2^31` and a constant non-empty page forever → unbounded append → OOM/UI freeze. There is no `maxPages`/`maxItems` cap.
- **Why guards don't prevent it:** `SearchTokens` (`nom_service.go:935,940`) **does** cap with `maxPages = 50`, proving the pattern is known; the four loops omit it. The default node is the third-party `wss://my.hc1node.com:35998` (`dto.go:136`).
- **Impact:** Whole-app hang/crash (availability) — a wallet that cannot be operated to move funds is a meaningful DoS; triggerable if the connected node is ever compromised or MITM'd on plaintext `ws://`. No funds movement.
- **Remediation:** Add a hard `maxPages`/`maxItems` ceiling (mirror `SearchTokens`) and/or stop when a page returns fewer than `pageSize`; treat `Count` as untrusted.
- **Regression test:** Stub the SDK API seam to return `Count=huge` + constant non-empty pages; assert the method returns within a small page budget.

### GS-04 — First-party NoM writes confirm material ABI params from form input, not the built block
- **Severity:** Low | **Confidence:** High | **CWE-345 / CWE-451**
- **Locations:** `app/nom_service.go:134,155,259,275,397,409,422,575,588,617,656,673,743,757,769,782,795,1012,1057,1078,1108`; `app/nom_accelerator.go:268,294` (lead-auditor confirmed via grep: these use `prepareCall` with a `Summary` string, not `prepareCallWithEffect`/`decodeContractCall`).
- **Invariant:** Confirmation must show the exact effect derived from the **built** block.
- **Detail:** `decodeContractCall` (`tx_effect.go:44`) exists precisely to render a confirmation from the exact held ABI bytes (fail-closed round-trip), and it IS used for governance, accelerator create/phase, `CollectReward`, and WalletConnect. It is **not** used for the first-party NoM writes, whose *material* parameter often lives entirely inside ABI `Data` (the block's `Amount`/`TokenStandard`/`ToAddress` are a zero-value ZNN carrier): e.g. **Mint** receiver+amount (`:1057`), **UpdateToken** new owner + mint/burn flags (`:1108`, irreversible on-chain), **RegisterPillar/UpdatePillar** producer/reward address + reward % (`:617,656`), **Fuse** plasma beneficiary (`:134`), **IssueToken** full metadata (`:1012`). For these the user sees the material parameter only via a form-derived `Summary`.
- **Why it is not a live hole:** (1) the funds-moving block fields ARE block-derived and enforced — `assertMatches` (`tx_service.go:81-86`) requires the signed block's `ToAddress`/`TokenStandard`/`Amount`/`Data` to byte-equal the held `callExpect` (whose `to`/`zts` are hardcoded per method); a node cannot alter destination/token/amount. (2) The ABI `Data` is built locally by the author-owned SDK from the *same parsed values* the summary renders, so no remote/untrusted party can cause a divergence — it would require a latent bug in the trusted SDK ABI encoder or the wallet's summary formatting.
- **Impact (only if a trusted-code bug existed):** user signs a block whose real effect (mint receiver, token-owner transfer, pillar reward address) differs from the displayed summary → value/rewards directed to an unintended address.
- **Why guards/tests don't prevent it:** `assertMatches` proves signed-block == SDK-template, not that the template's ABI bytes decode to the rendered summary. No test asserts decoded-fields == summary for these methods.
- **Remediation:** Route the remaining NoM writes (prioritize `Mint`, `UpdateToken`, `RegisterPillar`, `UpdatePillar`, `IssueToken`, `Fuse`) through `prepareCallWithEffect` with `decodeContractCall(template.ToAddress, template.Data)` so the dialog renders beneficiary/receiver/owner/duration/percentages from the signed bytes.
- **Regression test:** For each NoM write template, assert `decodeContractCall(...)` field values equal the values the summary renders (mirror `TestDecodeAcceleratorTemplates`/`assertEffectHasValues`, `tx_effect_test.go:200-253`).

### GS-05 — Wallet manifest: unsynchronized read-modify-write + non-fsync fixed temp file
- **Severity:** Low | **Confidence:** High | **CWE-362 / CWE-667 / CWE-367**
- **Location:** `app/wallet_manifest.go:60-74` (`saveManifest`), `:34-58` (`loadManifest`); callers `wallet_service.go:115,176,204,216`. Lead-auditor confirmed: `tmp := p + ".tmp"`, `os.WriteFile(tmp, data, 0o600)` (no `Sync`), then `os.Rename`; no manifest mutex on `WalletService`.
- **Trace:** Two concurrent writers (e.g. an `ImportKeystore` racing a `ListWallets` reconcile) each `loadManifest` from the same baseline → mutate → `saveManifest`. The fixed `.tmp` path can be overwritten by the second writer before the first renames → one update lost; a crash between `WriteFile` and `Rename` can leave a partial `.tmp`. Contrast `setSettingsLocked` (`config_service.go:138-159`) and `wcJournal.saveLocked` (`walletconnect_journal.go:154-172`), which use unique `CreateTemp` + `Sync` + `Rename`.
- **Impact:** Lost wallet **display name** or a freshly-imported manifest entry. **Not funds** — keystores are never touched, and `ListWallets` self-heals by re-registering any on-disk keystore missing from the manifest (`wallet_service.go:153-161`). Cosmetic/recoverable → Low.
- **Why guards don't prevent it:** `go test -race` passes because the race is on shared **file** state, not in-process memory the detector sees; no test hammers concurrent manifest writers.
- **Remediation:** Guard manifest read-modify-write with a dedicated mutex; switch `saveManifest` to unique `CreateTemp` + `Sync` + `Rename`.
- **Regression test:** Concurrent `ImportKeystore`+`RenameWallet` under `-race` asserting every mutation survives and no `.tmp` remains.

### GS-06 — GitHub Actions pinned to mutable major tags
- **Severity:** Low–Medium | **Confidence:** High | **CWE-829 / CWE-1357**
- **Locations:** `.github/workflows/ci.yml:25,26,29,42,67,86,90,93,101,112` and `.github/workflows/release.yml:36,46,49,52,90,101,112` — all third-party actions pinned `@v2`/`@v4`/`@v5` (movable tags), e.g. `actions/checkout@v4`, `softprops/action-gh-release@v2`.
- **Detail:** A moved/compromised tag could alter executed code. Highest stakes in `release.yml`, which runs with `permissions: contents: write` (`release.yml:16`) and passes `GITHUB_TOKEN` to `softprops/action-gh-release@v2`.
- **Mitigations (verified):** CI gate is least-privilege `permissions: contents: read` (`ci.yml:11`); **no secrets** on the `pull_request` trigger (the WC project id is injected only in the tag-gated `release.yml` via `vars`); release is triggered **only** by a maintainer-pushed `v*` tag, so fork "pwn requests" cannot reach write paths or secrets.
- **Remediation:** Pin all third-party actions to full commit SHAs (with a version comment); consider Dependabot for action bumps.
- **Regression test:** CI lint (e.g. `pinact`/`zizmor`) that rejects non-SHA action refs.

### GS-07 — Node-supplied token decimals not range-bounded before display
- **Severity:** Low (display only) | **Confidence:** High | **CWE-807**
- **Location:** `app/decimals.go:74-85` (`clientTokenDecimals`, `int(tok.Decimals)` at `:83` — no range check); used by `walletconnect.go:256` and first-party previews `tx_service.go:224,495`.
- **Detail:** For a custom ZTS, decimals are read straight from the node with no `[0,18]` bound (unlike issuance, which validates `0..18` at `nom_service.go:987`). A node lying about decimals renders a misleading **human** figure in the confirm dialog.
- **Why display-only:** the **signed amount never derives from node metadata** — it comes from user input (`tx_service.go:113`) or the dapp's base-unit field (`walletconnect.go:69`), and base units are shown alongside (`walletconnect.go:328`; `WalletConnectRequest.vue:54-57`). `resolveDecimalsChecked` already fails **closed** when decimals are **missing**; it just trusts the **value** when present.
- **Remediation:** Clamp/reject node decimals outside `[0,18]` before rendering; keep base units as the authoritative figure.
- **Regression test:** Stub lookup returning `decimals=200`/negative; assert rejection or clamp.

### GS-08 — Stale prior-session account data displayed between unlock and first refresh
- **Severity:** Low | **Confidence:** High | **CWE-672**
- **Location:** `frontend/src/stores/wallet.ts:62-70` (teardown clears only wallet state), `frontend/src/components/AppShell.vue:100-109` (async refresh on mount), `frontend/src/stores/balances.ts:10-19`.
- **Detail:** On lock, only the wallet store is cleared; account-scoped stores (`balances`, `txs`, `unreceived`, `plasma`, `pillar`, `accelerator`) retain the previous wallet's data. After unlocking a *different* keystore, until `GetBalances` etc. resolve, the UI briefly renders wallet A's balances/history while wallet B is unlocked.
- **Impact:** Transient display staleness in the same OS-user window (shoulder-surfing/shared-machine). **Authorization remains backend-enforced** — this is display only; no signing uses stale data.
- **Remediation:** Synchronously reset account-scoped stores to empty in `_applyLocked()`/`unlock()` (or before `refresh()`) so the UI shows a loading state.
- **Regression test:** Store test: populate balances, call `_applyLocked()`/unlock, assert `items` empty before any RPC resolves.

### GS-09 — WalletConnect request-expiry cancellation keys on numeric id only
- **Severity:** Low | **Confidence:** Medium | **CWE-697**
- **Location:** `frontend/src/stores/walletconnect.ts:1043-1074` (`handleRequestExpired`).
- **Detail:** SignClient 2.23.9 emits `session_request_expire` as `{ id }` with **no topic**, and the wallet matches that id against requests on *any* topic. A malicious already-approved dapp can send a `znn_send` whose id collides with an honest dapp's in-flight id; it is busy-rejected (`:568-570`) but SignClient keeps it pending until expiry, then the expiry event cancels the honest request's hold and closes its modal (`:1070-1073`) — repeatable DoS against the honest dapp. SignClient itself keys `pendingRequest` by id only, so this is a library-wide property.
- **Direction is fail-safe:** no funds move, no rejection is fabricated for the honest id, first-party transactions are untouched; approval still re-checks `expiryTimestamp` and fails closed (`:795-802`).
- **Remediation:** On `session_request_expire(id)`, consult `client.pendingRequest.getAll()` before acting; only cancel tracked `(topic,id)` pairs actually gone at the relay; when ambiguous, prefer leaving the request displayed.
- **Regression test:** Two topics sharing an id; expire one; assert the other's hold survives.

### GS-10 — CI security scanners installed from `@latest`/floating refs
- **Severity:** Low | **Confidence:** High | **CWE-829 / CWE-1357**
- **Location:** `.github/workflows/ci.yml:52` (`govulncheck@latest`), `:56` (`gosec@v2.27.1`), `:111`/`release.yml:61` (`wails@v2.12.0`).
- **Detail:** The **govulncheck gate itself** depends on `@latest` — whatever version is current at run time. A compromised or behavior-changed `govulncheck@latest` could silently pass vulnerable code (the gate parses IDs from stdout, `scripts/govulncheck-gate.sh:14-21`).
- **Mitigation (verified):** the gate script distinguishes "tool failed" (exit ≠ 0/3) from "no vulns" (`:16-20`), so a *crashing* tool fails loudly — but a *maliciously compliant* tool would not be caught.
- **Remediation:** Pin `govulncheck`/`gosec`/`wails` to an immutable version or SHA.
- **Regression test:** Assert the workflow references pinned scanner versions.

### GS-11 — `SetNodeURL` persists userinfo/query/fragment/path
- **Severity:** Informational (hardening) | **Confidence:** High | **CWE-200 / CWE-522**
- **Location:** `app/node_service.go:413-416` (validation checks only `scheme ∈ {ws,wss}` and `Host != ""`).
- **Detail:** A user may persist `wss://user:pass@host/path?apikey=…#frag` into `settings.json` (0600). The backend never discloses the URL to third parties, and the one cross-trust-boundary disclosure (WC `znn_info`) is sanitized by `publicWalletConnectNodeURL` (`walletconnect.ts:196-210`), which rejects userinfo/query/fragment/non-root path (tests `walletconnect.test.ts:141-152`). So there is **no live third-party leak**. Residual: (a) credentials persist in plaintext `settings.json`; (b) connection errors wrapping the dial URL (`node_service.go:93,99,108`) could echo userinfo back into the WebView. The userinfo is legitimately used as basic-auth to the user's own node.
- **Remediation:** Optionally reject/strip `Userinfo`/`RawQuery`/`Fragment` in `SetNodeURL` and scrub URLs from error strings returned to the frontend.

### GS-12 — `toBase` silently normalizes malformed decimal input
- **Severity:** Informational | **Confidence:** High | **CWE-20**
- **Location:** `frontend/src/lib/format.ts:4-8`.
- **Detail:** `'1.2.3'` → `1.2` (second dot dropped); `'-0.5'` → positive `0.5`. **No confirm-what-you-sign violation:** the amount sent to `PrepareSend` is what the Go-built preview renders exactly (`TxModal.vue:62-70` uses `formatAmountExact(preview.amount)`), the backend re-validates authoritatively, and `AmountInput.vue:19` strips non-`[0-9.]` chars anyway.
- **Remediation:** Reject strings with more than one `.` or a misplaced `-` in `toBase`.

### GS-13 — Dependency notes (lottie-web eval; nom-ui git dep)
- **Severity:** Informational | **Confidence:** High | **CWE-1104**
- **Location:** `frontend/package.json:13-25`.
- **Detail:**
  - `lottie-web ^5.13.0` uses `eval` in its expression renderer (the pre-existing Vite build warning; `dist/assets/lottie-*.js` 307 kB). Only a **bundled local** animation is loaded (`IntroSplash.vue:46-51`), so the eval input is not attacker-controlled — not exploitable today — but it blocks a strict `script-src` (GS-01). Replace with an eval-free/svg renderer.
  - `nom-ui` is pinned to immutable commit SHA `63f755a` (good), but git deps lack registry attestation/provenance — treat that repo as part of the trusted code base and vendor-review on bump.
  - `@walletconnect/sign-client 2.23.9` exact-pinned, no known advisories affecting this usage at audit time; relay key material lives in WebView `localStorage` (GS-01 blast-radius note).
  - `vite ^5.4` dev-server advisories exist but affect only the dev server, not the shipped static bundle.

---

## 9. Previously reported findings independently verified as remediated

All eight findings of `docs/walletconnect-audit-2026-07-17.md` were re-checked in the
**current** code (not trusted):

| Prior ID | Verification evidence in current code |
|---|---|
| **WC-01** publication durability/idempotency | Signed block journaled **before** broadcast (`tx_service.go:374-397`); broadcast error keeps state `signed`/unknown, never a definite rejection (`:404-413`); reconcile queries by hash and rebroadcasts the **exact** stored block with hash-integrity + chain checks (`walletconnect.go:349-428`); fail-closed journal capacity (`walletconnect_journal.go:205-207`); intent-hash conflict fails closed (`walletconnect.go:157-164`); same-intent new-id and cross-topic matching block duplicates (`:168-203`, `walletconnect_journal.go:235-270`). |
| **WC-02** request expiry | `session_request_expire` listener installed (`walletconnect.ts:277`); `handleRequestExpired` cancels the exact hold and never fabricates a response for publishing/unknown states (`:1043-1074`); approval re-checks `expiryTimestamp` and fails closed without the event (`:795-802`). |
| **WC-03** custom-token decimals | Confirmation renders the held block's raw base units + full ZTS as authoritative (`WalletConnectRequest.vue:54-57`); `resolveDecimalsChecked` fails preparation instead of guessing 8 (`decimals.go:46-63`, applied `walletconnect.go:256-259`). Residual accepted: the human line uses node-reported decimals (see GS-07), mitigated by the adjacent exact line. |
| **WC-04** mainnet opt-out race | Final `guardChain(built.ChainIdentifier)` immediately before broadcast (`tx_service.go:367-373`); reconcile re-checks the gate against the block's chain before rebroadcast (`walletconnect.go:412-416`). |
| **WC-05** Verify identity | `verifyContext` captured for proposals (`walletconnect.ts:305`) and requests (`:405`); missing context degrades to UNKNOWN, never trusted (`:157-165`); scam hard-blocks proposal approval, `znn_info` disclosure, and fresh `znn_send` (`:339-342,409-412,577-580`); verified origin shown separately from claimed name in both proposal and approval UI. |
| **WC-06** node URL disclosure | `publicWalletConnectNodeURL` (`walletconnect.ts:196-210`) allows only bare `ws:`/`wss:` origins; rejects userinfo/query/fragment and any non-root path; tests cover path-embedded tokens and percent-encoded paths (`walletconnect.test.ts:138-152`). |
| **WC-07** icon fetching | No `<img>` binds peer metadata anywhere; proposal/session lists render a local `GlobeIcon`; the `icon` field is stored but never rendered (dead data); test asserts zero `img` with hostile icon URLs (`WalletConnect.test.ts:22-49`). |
| **WC-08** proposal expiry | `proposal_expire` listener (`walletconnect.ts:278`); only the matching id is cleared so a late expiry can't wipe a newer proposal (`:308-312`); approval re-checks the deadline (`:332-336`). |

Also re-confirmed from `docs/security-audit-hardening-2026-07-19.md`: transient SDK
keystore seed/entropy zeroing (`wallet_service.go:731-734`), clipboard auto-clear of the
recovery phrase (`Create.vue` `SEED_CLIPBOARD_TTL_MS`), and `secrets/` never committed
(gitignored; no history — re-verified by the infra agent via pickaxe/`git check-ignore`).

---

## 10. Positive security controls observed (with evidence)

- **Binding boundary intact.** No Pinia store/component holds passwords/mnemonics beyond transient refs passed straight into Go and cleared; `RevealMnemonic` is password-gated and its output hidden; generated mnemonics surface once at creation; no secrets in `localStorage` (only theme/splash). Generated bindings cross-checked 1:1 against Go receivers — zero missing/extra.
- **Confirm-what-you-sign on the built block.** `assertMatches` (`tx_service.go:81-86`) re-asserts sender/recipient/token/amount/data on both the template and the **built** block (`:313,359`); non-zero `holdId` identity gate fails closed (`:294-299`); session-gen + active-address re-checks bracket the slow PoW (`:306,354`); `publishMu` serializes confirm/receive/reconcile (`:269-272,524`). Covered by `TestConfirmPublishRejects{AccountSwitch,SenderMismatch,WhenLocked,ChainMismatch,MismatchedHoldID,Concurrent}`.
- **Mainnet/chain guard is node-independent and fails closed.** The built block's `ChainIdentifier` comes from **settings** (`:123-129`, default→mainnet), never the node; `guardChain` keys off the **built block's** chain (`:140-151,371`); chain-match check refuses any mismatch (`:328,363,541`). `chainID 0` (disconnected) fails closed: `disconnectLocked` resets `chainID=0` (`node_service.go:288`) and every path then fails the `template.ChainIdentifier(≥1) != 0` match.
- **WalletConnect intent reconstruction.** No dapp-supplied frontier/PoW/hash/pubkey/signature enters signing (`walletconnect.go:16-19,100-109`); chain pinned to mainnet (`:41`), destination pinned to `BridgeContract` (`:62`), method allowlist `WrapToken`/`RedeemUnwrap` (`:81`), canonical-base64 round-trip (`:74`), Redeem must attach no funds (`:89-99`).
- **ABI canonicality + allowlisting.** `decodeContractCall` enforces known-contract + `MethodById` + arg unpack + **byte-exact re-encode round-trip** (`tx_effect.go:52-66`); `TestDecodeContractCallFailsClosed` proves it refuses unknown destinations, cross-contract selectors, truncated/garbage bytes.
- **Amounts stay exact integers.** All amounts cross the boundary as base-unit decimal strings parsed via `big.Int.SetString(…,10)` with sign checks; no float/JS-number path touches a signed value; `format.ts` is pure BigInt/string math.
- **Governance truly kill-switched, testnet-only, unbypassable.** `governanceFeatureEnabled = false` (`nom_governance.go:50`) checked at every entry point; no production setter (only test flips with cleanup); `requireTestnet` re-asserted at publish via `callExpect.policy`; WC cannot carry a governance block. `TestGovernanceDisabled_AllBoundMethodsBlocked` covers it.
- **TLS certificate verification on `wss://`.** SDK uses `websocket.DefaultDialer`; governance uses go-ethereum `DialWebsocket` with a bare dialer — neither sets `InsecureSkipVerify`. No MITM via disabled TLS validation.
- **Atomic, `0600`/`0700` persistence (settings + journal).** `dataDir`/`walletsDir` are `0700` (`config_service.go:26,36,48`); settings + journal use unique-temp + `Sync` + `Rename`; corrupt manifest is backed up and rebuilt from on-disk keystores (`wallet_manifest.go:47-53`).
- **Path-traversal guards.** `walletPath` rejects `""/./..`/non-`Base` names (`wallet_service.go:98-107`); `ImportKeystore` dst is a random id inside `walletsDir` with `O_EXCL` (`:185-195,796`); `DeleteEmbeddedData` removes a constant suffix and refuses while running (`node_service.go:532-548`).
- **Embedded node hardening.** WS bound to `127.0.0.1` only, HTTP disabled, empty `WSOrigins`, single-instance guard, bounded startup wait (`internal/embeddednode`).
- **No XSS sinks.** Repo-wide grep: zero `v-html`/`innerHTML`/`outerHTML`/`document.write`/`eval(`/`new Function`/`window.open`/`location.href`/`target="_blank"`/`javascript:` URLs; all peer-controlled strings render via Vue text interpolation. The one `:href` is a static internal route with `@click.prevent`.
- **Address/token spoofing surface minimal.** Confirmations show full un-truncated addresses; custom tokens display their **full ZTS** (the symbol map returns `""` for non-ZNN/QSR, `tx_service.go:92-101`), not a node-reported symbol.
- **Event-listener hygiene.** Every `EventsOn` is one-shot guarded; DOM listeners and the price timer are removed on unmount; lock/account/network transitions tear down WC requests/holds by identity without racing in-flight publications.
- **Secrets not committed.** `secrets/` gitignored with no git history; `frontend/.env.local` ignored; release-injected `VITE_WALLETCONNECT_PROJECT_ID` is a non-secret by design.
- **Go dependency integrity.** `go mod verify` passes; the `go-zenon` `replace` is a pseudo-version pinned to commit `81c247408859` with go.sum H1 verification; `nom-ui` pinned to immutable commit SHA.

---

## 11. Scanner / advisory triage

- **`go mod verify`** — all modules verified (go.sum integrity intact, incl. the forked `0x3639/go-zenon` pseudo-version).
- **`go test -race ./...`** — passes; no data races. Caveat: it does not exercise concurrent **manifest** writers, so GS-05's file-level race is invisible to it.
- **`gosec` / `govulncheck`** — **not installed locally → could not be reproduced** (limitation; not installed per no-network constraint). CI runs both (`ci.yml:50-57`).
- **govulncheck allowlist (`.github/govulncheck-allow.txt`)** — 5 IDs (`GO-2026-4314/4315/4507/4508/4511`), all go-ethereum devp2p/RLPx p2p DoS/handshake issues. **Justification is defensible:** reachable only via the opt-in embedded node's p2p stack (default remote/local modes are pure WS RPC clients with no p2p), none touch keys/signing/funds, and they cannot be bumped without breaking the pinned go-zenon build. The gate script fails loudly on tool error and on any non-allowlisted ID (`scripts/govulncheck-gate.sh:16-30`). **Residual:** the embedded node *does* expose this p2p stack when enabled — acceptable for a full node; revisit when go-zenon migrates off go-ethereum p2p.
- **`.gosec.json` is `{}`** — gosec relies on per-line `#nosec` suppressions, each carrying a justification comment (e.g. `config_service.go:26,109`, `wallet_service.go:791,796`, `node_service.go:629`). Reviewed suppressions are scoped and reasonable.
- **`pnpm audit --prod`** — no known vulnerabilities.
- **Generated assets/bindings** — `frontend/wailsjs/` is produced by `wails build` from the Go-bound method set; the CI `security` job stubs `frontend/dist` (scanners analyze Go source, not the bundle). No evidence of hand-edited generated files diverging from reviewed inputs.

---

## 12. Coverage matrix

| Audit area | Covered | Evidence / outcome |
|---|---|---|
| 1. Wails trust boundary | ✅ | All 5 bound structs + every exported method enumerated & cross-checked vs generated bindings; secret-crossing methods (`RevealMnemonic`/`GenerateMnemonic`) narrowly scoped; DTOs use base-unit strings (no ambiguous number deserialization of signed values); CSP gap → GS-01. No sensitive data returned by reads. Concurrent-call invariants hold via `publishMu`/`mu`/`selMu`/`opMu`. |
| 2. Wallet & secret lifecycle | ✅ | Mnemonic gen (256-bit BIP39), import, keystore create/unlock, account derivation, password change (atomic), reveal (password + session re-check), lock/auto-lock, shutdown reviewed. Seed/entropy zeroed; keystore `0600`; path-traversal guarded. SDK keystore/Argon2/AES internals trusted-not-audited (§13). |
| 3. Transaction & NoM operations | ✅ | End-to-end intent→validate→template→effect→hold→PoW→sign→policy→publish→result traced for send/receive/auto-receive and all NoM ops. Confirm-what-you-sign + mainnet/chain re-checks confirmed. Material-ABI-summary gap → GS-04; pagination DoS → GS-03. |
| 4. WalletConnect | ✅ | Pairing/session validation, frozen `zenon:1` namespace, active-address binding, Verify identity, expiry, replay/id-conflict, canonical base64/amounts/methods/destination/attached-funds, durable journal (integrity/atomicity/bounds/DoS), persist-before-broadcast, rebroadcast/reconcile, ack/crash recovery, node-URL disclosure, icon fetching, mainnet revocation, cross-session/topic, metadata XSS — all reviewed. Prior WC-01…08 verified remediated. Residual: GS-09 (expiry id), GS-01 (CSP for metadata). |
| 5. Node modes & RPC | ✅ | URL scheme/host validation, TLS verification, SSRF/userinfo leakage, oversized-response/DoS (GS-03), client-side validation of node data, reconnect/stale-state races (`connGen`/`connectionSnapshot`), embedded-node data-dir safety/lifecycle/permissions/binding, chainID-0 fail-closed. Residual: GS-03, GS-07, GS-11. |
| 6. Persistence & local data | ✅ | Settings/contacts/manifest/node-config/WC-journal reviewed for permissions, atomicity, symlink/traversal, corrupt/oversized input, defaults, sensitive fields, concurrent writers, crash consistency. Residual: GS-05 (manifest). |
| 7. Frontend & WebView | ✅ | Vue/Pinia/router guards/events/WC UI/confirmation/format/deps reviewed; no XSS sinks; BigInt-only amounts; clipboard auto-clear; stale-state (GS-08); CSP (GS-01); `toBase` (GS-12); deps (GS-13). |
| 8. Concurrency & failure recovery | ✅ | `go test -race ./...` clean; mutex coverage & lock ordering reviewed (opMu→mu; selMu→mu; autoMu alone; publishMu; journal mu; config mu); concurrent publish/auto-receive, lock-during-PoW, node reconnect, duplicate publication, crash windows analyzed. Residual: GS-05 (file-level race the detector can't see). |
| 9. Supply chain / CI / build / release | ✅ | go.mod/go.sum, package.json/pnpm-lock, git deps, replace directives, wails config, workflows, scripts, govulncheck allowlist reviewed. Residual: GS-02 (signing), GS-06 (action pinning), GS-10 (scanner pinning), GS-13 (deps). |

---

## 13. Untested or externally trusted components

- **`github.com/0x3639/znn-sdk-go` v0.2.1** — builds every embedded-contract template (`ToAddress`/`TokenStandard`/`Amount` and the ABI `Data` via `PackMethodPanic`), BIP39/BIP44 derivation, keystore Argon2id/AES, PoW, Ed25519 signing, RPC transport. The wallet trusts the SDK to encode parsed inputs into the correct ABI method/argument order and to implement crypto compatibly with go-zenon. **Not independently source-audited here** — only its usage and the wallet's `assertMatches`/`decodeContractCall`/Phase-0 address cross-check above it. This is the component whose latent bug GS-04 would fail to catch for summary-only methods. **Note: AGENTS.md documents v0.1.19; the pinned version is v0.2.1 — stale doc.**
- **`github.com/0x3639/go-zenon` (fork, `v0.0.0-20260615011802-81c247408859`)** — supplies `vm/embedded/definition` ABI definitions (used by both the SDK encode and the wallet's decode, so encode/decode cannot drift), `vm/constants` collateral/supply limits, and the address/ZTS/hash/AccountBlock types. Because the same `definition.ABI*` objects drive encode and decode, the round-trip check is sound by construction. **Not independently source-audited.**
- **`internal/governance`** (in-repo) — proposal payload builders; gated behind the kill switch; exercised by `TestDecodeEveryProposeKind`.
- **WalletConnect SignClient 2.23.9** — relay transport, session/request lifecycle, Verify service. Trusted library; the wallet validates everything it produces (GS-09 notes its id-only `pendingRequest` keying).
- **Live node behavior** — not exercised (no integration tests run). All node-input handling was reviewed statically and via stubbed seams.

---

## 14. Prioritized remediation roadmap

**P0 — before any mainnet-capable release:**
- **GS-01**: Ship an effective CSP (remove lottie eval; `script-src 'self'` + explicit `connect-src` + `img-src 'self' data:`), validated against a production `wails build`. *(No active exploit today, but this is the primary WebView blast-radius control for a mainnet WalletConnect feature.)*

**P1 — before a signed production release:**
- **GS-02**: Code-sign + notarize; publish out-of-band authenticated checksums / SLSA provenance.
- **GS-06**: Pin all third-party GitHub Actions to immutable SHAs.
- **GS-10**: Pin CI security scanners (`govulncheck`/`gosec`/`wails`) to immutable versions.
- **GS-03**: Bound the four pagination loops (availability; trivial fix, mirrors `SearchTokens`).
- **GS-04**: Route high-value NoM writes (`Mint`, `UpdateToken`, pillar register/update, `IssueToken`, `Fuse`) through block-decoded confirmation effects.

**P2 — hardening / follow-up:**
- **GS-05**: Mutex + `CreateTemp`/`Sync` for the manifest writer.
- **GS-07**: Clamp/reject node decimals outside `[0,18]`.
- **GS-08**: Clear account-scoped stores on lock/unlock.
- **GS-09**: Make WC expiry cancellation topic-aware.
- **GS-11**: Strip/scrub userinfo/query/fragment from node URLs + error strings.
- **GS-12**: Reject malformed decimals in `toBase`.
- **GS-13**: Replace lottie eval renderer; vendor-review `nom-ui` on bump.
- **Doc hygiene**: Update AGENTS.md SDK version (v0.1.19 → v0.2.1).
- **Upstream** (author-owned SDK): zero the derived BIP32 child key in `znn-sdk-go` `GetKeyPair` (marginal defense-in-depth, per the 2026-07-19 hardening notes).

---

## 15. Suggested regression tests

1. **CSP enforcement (GS-01):** production-build load under a CSP-violation listener; assert zero violations and that an injected inline script does not execute.
2. **Pagination bounds (GS-03):** stub the SDK list APIs to return `Count=huge` + constant non-empty pages; assert each of the four methods returns within a small page budget.
3. **NoM summary == decoded effect (GS-04):** for each NoM write template, assert `decodeContractCall(template.ToAddress, template.Data)` field values equal the values the summary renders.
4. **Manifest concurrency (GS-05):** concurrent `ImportKeystore`+`RenameWallet` under `-race`; assert every mutation survives and no `.tmp` remains.
5. **Action pinning (GS-06) / scanner pinning (GS-10):** CI lint (e.g. `zizmor`/`pinact`) rejecting non-SHA action/scanner refs.
6. **Decimals range (GS-07):** stub lookup returning `200`/negative; assert rejection or clamp.
7. **Stale-store clear (GS-08):** populate balances, call `_applyLocked()`/unlock, assert account-scoped `items` empty before any RPC resolves.
8. **WC expiry topic-awareness (GS-09):** two topics sharing an id; expire one; assert the other's hold survives.
9. **Release signature (GS-02):** assert a detached signature/attestation exists and verifies before publishing assets.

---

## 16. Final residual-risk statement

Within the reviewed scope and the evidence gathered, **no path was found that signs or
publishes an incorrect or unauthorized mainnet transaction, discloses key material to the
WebView, or produces a duplicate financial effect** through the dapp/relay/crash threat
model. The funds-critical controls (confirm-what-you-sign on the built block,
node-independent mainnet/chain gating that fails closed on disconnect, serialized
publication, the durable WC journal, TLS verification, path safety, BigInt-only amounts,
and the governance kill-switch) are robustly implemented and well covered by tests, and
the eight prior WalletConnect findings are independently confirmed remediated.

**This is not a claim that the application is "secure."** The evidence is bounded by:
(a) the pinned `znn-sdk-go` and `go-zenon` crypto/ABI/PoW/keystore internals were trusted,
not source-audited — a latent bug there (especially ABI encoding for the GS-04 summary-only
methods) would not be caught by the wallet's current confirmations or tests; (b) `gosec`/
`govulncheck` and a production `wails build` could not be run locally (CI provides this
coverage); (c) no live-node/integration testing was performed; and (d) the WebView has no
effective CSP (GS-01), so any *future* XSS sink would have broad IPC reach. The dominant
residual risks are the WebView CSP gap (GS-01), the unsigned release pipeline (GS-02), and
CI/action supply-chain pinning (GS-06/GS-10) — all addressable without architectural change.
A fully compromised WebView process remains, by design, outside what these controls can
contain for that session; the wallet's mitigations are preventing injection (no sinks +
CSP) and keeping keys in Go so the seed cannot be exfiltrated.
