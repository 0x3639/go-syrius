# Security Audit Hardening — Handoff Notes (2026-07-19)

> **Audience:** an AI agent (Claude) continuing work on this repo. Read this whole
> document before touching anything below. The "DO NOT" section is load-bearing —
> violating it will silently break a shipped mainnet feature.

## TL;DR

A full security audit of go-syrius found **no exploitable vulnerability in the
shipped signing/key-handling path** — the codebase is unusually well-hardened.
This branch (`security/audit-hardening`, created from `main`, **changes
uncommitted**) implements the *code-level* recommendations from that audit:

1. Zero the transient SDK keystore's derived seed/entropy after each signing op.
2. Add a baseline (safe-subset) Content-Security-Policy to the WebView.
3. `chmod 600` the local-only `secrets/` test material (not in git).

It also independently verified the audit's other claims (clipboard auto-clear was
**already implemented**; `gosec`/`govulncheck` gates are **green**; the local
`secrets/` material has **never been committed and cannot be pushed**).

---

## Context

go-syrius is a Go + Wails v2 desktop wallet for the Zenon Network of Momentum
(reimplementation of the Flutter `syrius`). Core invariant from `AGENTS.md`:
**the WebView is untrusted for key material** — the frontend sends *intent*, Go
builds → PoWs → signs → publishes; no private key / mnemonic / decrypted keystore
ever crosses the binding except the explicit, password-gated `RevealMnemonic` and
the create-time `GenerateMnemonic`. The audit confirmed this invariant holds.

Crypto primitives (Argon2id/AES/BIP39/Ed25519) live in the imported
`znn-sdk-go` / `go-zenon`; the audit reviewed their **usage** and the
compatibility proofs, not the upstream implementations.

---

## Changes made

### 1. `app/wallet_service.go` — zero the transient SDK keystore

Each signing operation builds a *transient* `sdkwallet.KeyStore` from the resident
mnemonic inside `signingKeyPair()`. Its derived BIP39 `Seed` (64 bytes) and
`Entropy` previously lingered on the heap until GC. They are now zeroed as soon as
the keypair is derived.

```diff
@@ import block @@
 	"os"
 	"path/filepath"
+	stdruntime "runtime"
 	"strings"
```

```diff
@@ before signingKeyPair @@
+// zeroBytes overwrites b with zeros. stdruntime.KeepAlive keeps the slice live
+// until the writes complete so the compiler cannot elide them as dead stores
+// (mirrors the SDK's own secure-zero in znn-sdk-go/wallet).
+func zeroBytes(b []byte) {
+	for i := range b {
+		b[i] = 0
+	}
+	stdruntime.KeepAlive(b)
+}
```

```diff
@@ inside signingKeyPair, after NewKeyStoreFromMnemonic, before GetKeyPair @@
+	defer func() {
+		zeroBytes(sdkKs.Seed)
+		zeroBytes(sdkKs.Entropy)
+	}()
 	kp, err := sdkKs.GetKeyPair(w.active)
```

**Why it is safe:** `GetKeyPair` derives a BIP32 child key and calls
`ed25519.NewKeyFromSeed`, which allocates a **fresh** 64-byte private key
(`znn-sdk-go/wallet/keypair.go:44-51`). The returned `KeyPair` is therefore
**independent** of the keystore — zeroing the keystore's `Seed`/`Entropy` does not
affect signing. The `Mnemonic` field is an immutable Go string aliasing the
resident mnemonic and cannot/need not be zeroed.

**Why `stdruntime`:** the plain stdlib package name `runtime` is already taken in
this file by the Wails import `github.com/wailsapp/wails/v2/pkg/runtime`, so the
stdlib is aliased. `KeepAlive` prevents the compiler eliding the zeroing loop as a
dead store (the SDK does the same in its own `zeroBytes`).

**Honest value assessment — this is marginal defense-in-depth.** The unlocked
go-zenon keystore *intentionally* holds the mnemonic resident for the whole
session, so a memory-read attacker gets the mnemonic regardless. The SDK's own
`znn-sdk-go/wallet/MEMORY_SECURITY.md` concludes "no changes needed." The change
was kept because it is cheap, correct, and removes one extra derived-seed copy. Do
not overstate its security impact, and do not extend it into risky territory
(e.g. trying to mlock memory — the SDK explicitly recommends against this for a
cross-platform app).

### 2. `frontend/index.html` — baseline Content-Security-Policy

```diff
     <meta charset="UTF-8" />
     <meta name="viewport" content="width=device-width, initial-scale=1.0" />
+    <!-- Baseline Content-Security-Policy. ... (see file for full rationale) ... -->
+    <meta
+      http-equiv="Content-Security-Policy"
+      content="object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
+    />
     <title>syrius</title>
```

**Why only these three directives:** they are provably non-breaking. The WebView
renders no `<object>`/`<embed>`, uses no `<base>` tag, and is never framed. They
block plugin/embed-based exfiltration and `<base>`-tag hijacking — baseline
hardening with zero functional risk. **Correction (post-review):**
`frame-ancestors` is *ignored* in `<meta>`-delivered policies per the CSP spec,
so that directive is inert as shipped; it is kept only for parity if the policy
ever moves to an HTTP header. The effective anti-framing protection is simply
that a desktop WebView is never framed.

**Why `script-src` / `connect-src` are deliberately LEFT OPEN:** the WebView makes
legitimate direct connections a restrictive CSP would break, and this **cannot be
verified outside a production `wails build`** (it is invisible under `wails dev`
and `vitest`):

- **WalletConnect relay** — `SignClient.init` in
  `frontend/src/stores/walletconnect.ts` uses the SDK **default relay**
  (`wss://relay.walletconnect.com`) plus its Verify/explorer HTTP APIs. No
  `relayUrl` override is set; the exact endpoint set is SDK-managed and
  version-dependent.
- **Price feed** — `frontend/src/stores/price.ts:4,25` does a direct
  `fetch('https://api.zenon.info/price')`.
- **Wails runtime** — a restrictive `script-src` could block the Wails-injected
  runtime and brick the backend binding.

> **⚠️ DO NOT tighten `script-src` or `connect-src` without first running a
> production `wails build` and launch-smoke-testing WalletConnect pairing + a send
> AND the price ticker.** Getting it wrong silently breaks WalletConnect (a shipped
> **mainnet** feature) or the backend binding, and the failure only appears in
> production builds — not in tests. The HTML comment in `index.html` restates this.

The proper long-term fix is to **proxy the price feed and WalletConnect through the
Go backend**, which would also remove direct WebView→internet calls (a security
improvement in its own right under the "frontend is untrusted" invariant). That is
a larger change and out of scope here.

### 3. `secrets/*` — `chmod 600` (local only, NOT in git)

The local testing material (`secrets/pillar-password.txt`,
`secrets/pillar-seed-words.txt`, `secrets/pillar.json`) was group/world-readable
(`0664`). It is now `0600`. **This is not in the git diff** — `secrets/` is
gitignored, so the permission change is a local-only operational fix, not a
committable change.

---

## Already implemented — DO NOT re-do

- **Clipboard auto-clear of the recovery phrase.** The audit initially recommended
  this, but it already exists: `frontend/src/views/Create.vue:13`
  (`SEED_CLIPBOARD_TTL_MS = 45_000`) and `:27-33` clear the clipboard after ~45s,
  guarded to only wipe it if the seed is still the current clipboard content. Do
  not add a second implementation.

---

## DO NOT (guardrails)

1. **Do not add `script-src`/`connect-src` to the CSP** without a production
   `wails build` smoke test (see the ⚠️ box above).
2. **Do not bump `github.com/ethereum/go-ethereum`** to "fix" the 5 govulncheck
   findings. They are intentionally allowlisted in `.github/govulncheck-allow.txt`
   (go-ethereum devp2p/RLPx DoS/handshake vulns, reachable **only** via the opt-in
   embedded node, none touch keys/signing/funds). Bumping breaks the pinned
   `go-zenon` build (verified against v1.16.8/1.16.9/v1.17.0). They clear when
   go-zenon completes its upstream libp2p migration.
3. **Do not commit anything under `secrets/`**, and do not use `git add -f` on it.
4. **Do not add git hooks / change git config** without an explicit user request
   (`AGENTS.md` prohibits it). A pre-commit guard that rejects commits touching
   `secrets/` was *offered* to the user but not implemented for this reason.
5. **Do not gratuitously add comments**, but **keep** the two comments added here
   (the CSP rationale in `index.html` and the `zeroBytes`/`signingKeyPair` notes) —
   they are load-bearing regressions guards, and this codebase's convention is
   rich security-rationale comments.

---

## Verification (all green at time of writing)

Run with the local-dev hazard workaround from `AGENTS.md` (a parent `go.work`
references a missing sibling module, so Go tooling needs `GOWORK=off`;
`go.mod` pins go 1.25.x so use `GOTOOLCHAIN=auto`):

```bash
# Backend
GOWORK=off GOTOOLCHAIN=auto go build ./...          # ok (only the known gopsutil/IOKit cgo deprecation warning)
GOWORK=off GOTOOLCHAIN=auto go vet ./app/...        # clean
GOWORK=off GOTOOLCHAIN=auto go test ./app/...       # ok

# Frontend (in frontend/, pnpm)
pnpm run typecheck                                  # vue-tsc, clean
pnpm test                                           # vitest, 392/392
pnpm run build                                      # Vite; CSP confirmed present in dist/index.html

# Security gates
GOWORK=off GOTOOLCHAIN=auto gosec -conf .gosec.json ./...   # 0 issues, 9 justified #nosec
bash scripts/govulncheck-gate.sh                            # OK: only allowlisted vulnerabilities present
```

**Gotcha:** `gosec` **must** be run with `GOWORK=off`, otherwise the parent
`go.work` makes it fail to load packages and it reports a misleading
`Files: 0 / Issues: 0` (a load failure, **not** a clean pass). `govulncheck` and
`gosec` may need installing first (`go install golang.org/x/vuln/cmd/govulncheck@latest`,
`go install github.com/securego/gosec/v2/cmd/gosec@latest`); they are present in CI.

---

## "Secrets are not pushed to GitHub" — verification

The user's explicit concern. Confirmed conclusively:

- `.gitignore:5:/secrets/` covers all three files (verified with `git check-ignore -v`).
- Not in the index, not in any stash, no secret-*named* file ever committed to any ref.
- **Content-level pickaxe search across all refs (including remote-tracking
  `origin/*`): 0 commits ever containing each secret's bytes** — i.e. the contents
  were never pasted into any tracked file either.
- `security/audit-hardening` has no upstream (never pushed); local `main` is 0
  commits ahead of `origin/main`.
- `internal/compat/keystore_compat_test.go:18-19,36` references the **path**
  `../../secrets/pillar.json` and reads it at runtime, skipping when absent
  (overridable via `ZNN_COMPAT_KEYSTORE` / `ZNN_COMPAT_PASSWORD`). It embeds only
  the path string, **never the content** — that committed reference is harmless.

Note: `pillar.json` is covered only by the `/secrets/` directory rule (not a
filename pattern like `*.dat`/`*-seed-words.txt`/`*-password.txt`), so it must stay
inside `secrets/` to remain ignored.

---

## Invariants to preserve

- **Binding boundary:** no secret crosses to the WebView except password-gated
  `RevealMnemonic` and create-time `GenerateMnemonic`. `signingKeyPair` and the
  mnemonic stay backend-only.
- **Confirm-what-you-sign:** `assertMatches` (`app/tx_service.go:81`) re-asserts
  sender/recipient/token/amount/data on the *built* block; `decodeContractCall`
  (`app/tx_effect.go`) requires a byte-exact ABI round-trip and fails closed. Do
  not weaken these.
- **Mainnet guard** is re-checked at publish against the built block's chain id;
  `disconnectLocked` resets the cached chain id to 0. Governance is kill-switched
  off (`governanceFeatureEnabled = false`) and testnet-only.

---

## Open follow-ups (not done here)

1. **Full CSP** (`connect-src`/`script-src`) — design an allowlist for the WC relay
   + price API + Wails runtime, then smoke-test in a production `wails build`.
   Better: proxy price + WalletConnect through Go.
2. **`secrets/` relocation/rotation** — operational; move the pillar seed/password
   off the dev machine and rotate if it was ever synced/shared.
3. **Optional pre-commit guard** against committing `secrets/` (offered, needs
   explicit user opt-in because it adds a git hook).
4. **devtools-off** — no code needed; Wails v2 prod builds exclude devtools by
   default. Confirm in the Phase 7b release matrix.
5. **Upstream SDK zeroing (found in review):** `znn-sdk-go` v0.2.1
   `wallet/keystore.go:115-121` — `GetKeyPair` never zeroes the derived BIP32
   child key (`keyData.Key`/`ChainCode`) after `NewKeyPairFromSeed` copies it,
   so one intermediate secret still lingers until GC one layer below the app fix.
   The SDK is author-controlled: add `zeroBytes(keyData.Key)` there (and consider
   exporting `zeroBytes` so the app can drop its private copy). Same
   marginal-defense-in-depth caveat as the app-side change.

## Commit status

Committed to branch `security/audit-hardening` (2026-07-19) after an independent
review verified every claim above (SDK v0.2.1 source, git-history pickaxe,
all gates re-run green). The review's two findings are folded in: the
`frame-ancestors` correction above, and open follow-up #5 (upstream SDK zeroing).
