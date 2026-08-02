# Additive User Entropy for Wallet Creation

**Date:** 2026-07-31  
**Status:** Proposed  
**Scope:** Phase 7f security hardening of the existing Phase 3 wallet-creation flow  
**Audience:** Implementer and security reviewer  
**Related:** `docs/superpowers/specs/2026-06-21-phase-3-wallet-lifecycle-design.md`, `plan.md` Phase 7

> **Addendum (2026-08-02):** `2026-08-02-entropy-target-indicator-design.md`
> narrowly revises invariant §5.11 and the §9.3 no-bit-estimate rule of this
> spec: the collection UI may show a fixed, conservatively credited bit figure
> (always labeled "estimated") and hard-gates interactive generation on a
> user-selected 128/256/512-bit target. All other invariants here stand
> unchanged.

## 1. Summary

Add an optional interaction-entropy step before a new wallet's BIP-39 recovery
phrase is created. A user may move a pointer, swipe, or press arbitrary keys to
contribute unpredictable timing and position data. This contribution is mixed
with two automatic sources:

1. 32 bytes from Go's `crypto/rand` in the trusted backend.
2. 32 bytes from the WebView's `crypto.getRandomValues` implementation.
3. A 32-byte SHA-256 digest of a bounded, canonical interaction transcript.

The Go backend combines the three fixed-length inputs with the Go standard
library's RFC 5869 HKDF-SHA-256 implementation and uses the resulting 32 bytes
as ordinary BIP-39 entropy. The output remains a normal 24-word BIP-39 mnemonic
and the existing Syrius-compatible keystore and address-derivation paths remain
unchanged.

Manual interaction is additive only. It must never replace backend operating-
system randomness. Weak, predictable, missing, or maliciously chosen frontend
contributions must not make the result weaker than the current backend-only
generation path.

## 2. Motivation

The 2026 COLDCARD entropy incident showed that an intended cryptographic RNG can
be present in a binary while the wallet-generation call path reaches a different,
weak fallback implementation. go-syrius currently has a short, auditable path:

```text
WalletService.GenerateMnemonic
    -> bip39.NewEntropy(256)
    -> crypto/rand.Read(32 bytes)
    -> bip39.NewMnemonic
```

That current path is secure under the supported operating systems and is not
affected by the COLDCARD bug. This feature is defense in depth: it introduces a
second automatic runtime path plus optional human interaction, then combines
the sources using a standard extractor.

The feature is not a claim that mouse movement or key timing provides a known
number of entropy bits. It is an additional noise source whose value is not
credited or quantified.

## 3. Goals

- Always retain 256 bits from backend `crypto/rand` as the primary source.
- Add an automatic 256-bit Web Crypto contribution where available.
- Let users optionally add pointer, touch, or keyboard-timing input before seed
  generation.
- Ensure a bad or attacker-controlled frontend contribution cannot cancel or
  reduce a good backend contribution.
- Keep all source combination and final BIP-39 generation in Go.
- Preserve standard 24-word BIP-39 output, keystore compatibility, and existing
  Zenon address derivation exactly.
- Avoid collecting key identities, typed characters, passwords, or phrases.
- Never persist or log entropy inputs, digests, final entropy, or mnemonics.
- Provide keyboard-only, pointer, and touch completion paths.
- Make the complete algorithm deterministic and test-vectorable below the
  production RNG boundary.

## 4. Non-goals

- Replacing Go's `crypto/rand` with user input.
- Claiming that interaction collection adds 128 or 256 bits.
- Implementing a custom PRNG, randomness estimator, or statistical RNG test.
- Protecting seed generation on a fully compromised host, WebView, application
  binary, or operating system.
- Hiding the generated mnemonic from the WebView; the existing creation flow
  must display it for backup.
- Changing BIP-39, the BIP-39 passphrase policy, derivation paths, Ed25519 keys,
  addresses, keystore encryption, or import behavior.
- Applying new entropy to an imported mnemonic. Imported mnemonics must remain
  byte-for-byte equivalent to what the user supplies.
- Adding dice-only seed generation in this iteration. A rigorously specified
  dice mode may be added later as a separate advanced feature.

## 5. Security invariants

These requirements are non-negotiable.

1. The backend obtains a fresh 32-byte `crypto/rand` value for every generation
   attempt.
2. Backend randomness is generated after the frontend request arrives and is
   never returned to the frontend.
3. Frontend values are treated as untrusted, attacker-controlled input.
4. Both frontend fields must be decoded and length-validated before use.
5. Source combination uses only `crypto/hkdf` with SHA-256 and the exact
   versioned construction in this document.
6. Do not XOR sources, concatenate them directly into a BIP-39 mnemonic, use
   `math/rand`, or invent an entropy-scoring algorithm.
7. The final BIP-39 entropy is exactly 32 bytes.
8. The backend exposes no binding that accepts caller-provided replacement
   backend randomness.
9. The mnemonic is returned only through the existing deliberate creation
   boundary and password-gated reveal boundary.
10. No generation error may silently downgrade from enhanced generation to
    backend-only generation.
11. Interaction progress is a UX completion measure, not a security estimate.
12. Generation inputs and their encoded forms must never enter logs, telemetry,
    analytics, error strings, crash breadcrumbs, or persisted settings.

## 6. Locked cryptographic construction

### 6.1 Inputs

Define these byte strings:

```text
B = 32 bytes generated by Go crypto/rand
R = 32 bytes generated by window.crypto.getRandomValues
U = 32-byte SHA-256 digest of the canonical interaction transcript
```

If the user skips interaction, `U` is SHA-256 of the empty transcript. `R` is
still generated and included. The skip path therefore skips only manual input,
not the second automatic source.

### 6.2 HKDF

The backend constructs the 96-byte input key material in this exact order:

```text
IKM = B || R || U
```

Use these exact ASCII domain-separation values:

```text
salt = "go-syrius/bip39-entropy/v1/extract"
info = "go-syrius/bip39-entropy/v1/output"
```

Derive exactly 32 bytes:

```text
E = HKDF-SHA-256(IKM, salt, info, 32)
```

In Go 1.25.12, use the standard library:

```go
entropy, err := hkdf.Key(
    sha256.New,
    ikm,
    []byte("go-syrius/bip39-entropy/v1/extract"),
    "go-syrius/bip39-entropy/v1/output",
    32,
)
```

Pass `E` to `bip39.NewMnemonic`. Do not pass any individual input directly to
BIP-39.

### 6.3 Why this construction

RFC 5869's extract stage is intended for input keying material that may be
non-uniform, partially known, or partially controlled, including entropy-
gathering applications. Fixed input lengths remove concatenation ambiguity.
The domain strings prevent accidental cross-protocol reuse.

The primary security claim remains the backend's 32-byte OS CSPRNG value. The
other inputs are defense in depth. Hashing and HKDF can consolidate entropy but
cannot manufacture entropy from wholly predictable inputs.

### 6.4 Memory handling

The backend must `clear` the following mutable byte slices/arrays with `defer`
as soon as they are allocated or decoded:

- backend random bytes;
- decoded renderer bytes;
- decoded interaction digest;
- assembled IKM;
- derived final entropy.

The request's Base64 strings and returned mnemonic are immutable Go/JavaScript
strings and cannot be reliably zeroed. They contain contributions rather than
the backend source or final raw entropy, but they must still be short-lived and
must never be retained or logged.

## 7. Backend contract

### 7.1 DTO

Add to `app/dto.go`:

```go
type MnemonicEntropyRequest struct {
    Version                   int    `json:"version"`
    RendererRandomBase64      string `json:"rendererRandomBase64"`
    InteractionDigestBase64   string `json:"interactionDigestBase64"`
}
```

Field names may be aligned by `gofmt`; JSON names are locked as written.

`Version` must equal `1`. Both strings use padded RFC 4648 standard Base64 and
must decode to exactly 32 bytes.

### 7.2 WalletService methods

Add:

```go
func (w *WalletService) GenerateMnemonicWithEntropy(
    req MnemonicEntropyRequest,
) (string, error)
```

Keep:

```go
func (w *WalletService) GenerateMnemonic() (string, error)
```

`GenerateMnemonic` remains the supported backend-only fallback and continues to
produce a standard 24-word phrase. For auditability, change its implementation
to read a local 32-byte buffer directly with `crypto/rand.Read` and then call
`bip39.NewMnemonic`; do not route production generation through a replaceable
or caller-provided `io.Reader`.

Both public methods must share a small private `mnemonicFromEntropy` helper for
BIP-39 conversion only. The enhanced method additionally uses a private,
deterministic `deriveMnemonicEntropy(backend, renderer, interaction []byte)`
helper implementing the locked HKDF construction. That helper is testable but
must not be Wails-bound or exported.

### 7.3 Validation and errors

Validate in this order:

1. `Version == 1`.
2. Renderer Base64 decodes successfully.
3. Renderer decoded length is exactly 32.
4. Interaction Base64 decodes successfully.
5. Interaction decoded length is exactly 32.
6. Generate fresh backend randomness.
7. Derive and convert the entropy.

Return concise, non-sensitive errors such as:

- `unsupported entropy request version`
- `invalid renderer entropy encoding`
- `renderer entropy must be 32 bytes`
- `invalid interaction entropy encoding`
- `interaction entropy must be 32 bytes`
- `derive mnemonic entropy: ...`
- `create mnemonic: ...`

Never echo an input string in an error.

### 7.4 Concurrency and persistence

Generation remains stateless and persists nothing. Concurrent generation calls
are allowed; every call obtains independent backend randomness. No WalletService
mutex is required unless implementation details introduce shared mutable state,
which this design does not require.

No new Wails events are needed.

## 8. Frontend entropy utility

Create a focused utility, preferably:

```text
frontend/src/lib/wallet-entropy.ts
frontend/src/lib/wallet-entropy.test.ts
```

It owns interaction serialization, transcript limits, digesting, Base64
encoding, renderer random generation, and cleanup. `Create.vue` should not
implement byte-level encoding inline.

### 8.1 Canonical transcript records

Each accepted interaction is encoded as one fixed 13-byte little-endian record:

```text
offset  size  field
0       1     event kind
1       4     microseconds since the previous accepted sample, uint32
5       4     value A, int32
9       4     value B, int32
```

Event kinds:

```text
1 = pointer movement
2 = touch/pointer movement with pointerType == "touch"
3 = key press timing
```

For pointer/touch events:

```text
A = round(clientX * 1024)
B = round(clientY * 1024)
```

Clamp values to the signed 32-bit range before encoding.

For key events:

```text
A = 0
B = 0
```

Only the timing delta contributes. Do not read or retain `event.key`,
`event.code`, modifiers, locale, scan code, or character data. Ignore repeated
keydown events where `event.repeat` is true.

Derive timing from `performance.now()`. Convert the non-negative delta to
microseconds, round it, and clamp it to `uint32`. A non-finite or negative delta
must be encoded as zero rather than throwing.

### 8.2 Bounds and throttling

Use locked constants:

```text
POINTER_SAMPLE_INTERVAL_MS = 16
COLLECTION_MIN_DURATION_MS  = 5_000
COLLECTION_TARGET_SAMPLES   = 32
COLLECTION_MAX_SAMPLES      = 2_048
```

- Accept no more than one pointer/touch sample per 16 milliseconds.
- Key events are not time-throttled, but repeated events are ignored.
- Stop appending after 2,048 records.
- The maximum transcript is therefore 26,624 bytes.
- Do not continue collecting invisibly once the maximum is reached.

The UI may enable `Generate recovery phrase` after both the minimum duration and
target sample count are met. These thresholds are a participation gate only.
The user can always choose the explicit skip-interaction path without waiting.

### 8.3 Digest

At finalization:

1. Freeze the transcript; stop accepting events.
2. Compute SHA-256 of the exact transcript bytes with
   `window.crypto.subtle.digest("SHA-256", transcript)`.
3. Generate a fresh 32-byte renderer value with
   `window.crypto.getRandomValues(new Uint8Array(32))`.
4. Base64-encode both 32-byte values using padded standard Base64.
5. Call `GenerateMnemonicWithEntropy` with `version: 1`.
6. In `finally`, fill all mutable byte arrays with zero and discard the sample
   buffer, regardless of success or failure.

Generate renderer randomness at finalization, not at component mount. Do not
expose it in Vue state, the DOM, developer messages, or errors beyond the brief
request object required for the binding call.

### 8.4 Availability failure

If `crypto.getRandomValues` or `crypto.subtle.digest` is unavailable or fails:

- Do not silently call `GenerateMnemonic`.
- Stop collection and show a factual error explaining that enhanced generation
  is unavailable.
- Offer an explicit secondary action labeled `Generate using backend randomness`.
- Only that deliberate action may invoke the existing `GenerateMnemonic` path.

Likewise, if `GenerateMnemonicWithEntropy` returns an error, show it and let the
user retry. Do not silently downgrade.

## 9. Create-wallet UX

The current route generates a mnemonic in `onMounted`. Remove that behavior.
No mnemonic may exist before an explicit generation action.

### 9.1 Wizard states

The create wizard becomes:

```text
entropy choice
    -> collecting interaction (optional sub-state)
    -> display recovery phrase
    -> verify recovery phrase
    -> wallet name and password
    -> ImportMnemonic / unlock / dashboard
```

This is four user-visible steps; the collection screen may have internal idle,
collecting, ready, and generating states.

### 9.2 Entropy-choice screen

Required content:

**Heading:** `Create wallet`

**Title:** `Generate your recovery phrase`

**Body:**

> Syrius always uses operating-system cryptographic randomness. You can also
> contribute unpredictable pointer movement or key timing before the recovery
> phrase is created.

Primary action:

```text
Add interaction randomness
```

Secondary action:

```text
Generate without interaction
```

The secondary action still uses both backend `crypto/rand` and renderer
`crypto.getRandomValues`; it skips only the transcript contribution.

Do not describe the secondary path as insecure or unsafe. Backend randomness
remains the primary security source.

### 9.3 Collection screen

Required content:

**Title:** `Add interaction randomness`

**Instructions:**

> Move your pointer, swipe, or focus this area and press arbitrary keys. Syrius
> records only movement, timing, and position—not the keys you press.

Also show:

> Do not type a password or recovery phrase. Your final recovery phrase is the
> only backup you need; these interactions are not saved.

The collection target must be a clearly visible, focusable region with an
accessible name. Collection listeners attach only to this target, not to
`window` or `document`.

Show:

- elapsed collection participation;
- a progress indicator labeled `Interaction collected`;
- sample count only if useful, labeled as samples rather than entropy bits;
- a ready state reading `Enough interaction collected`;
- `Generate recovery phrase` when ready;
- `Skip interaction` at all times.

Never show an entropy-bit estimate, randomness quality score, “military grade,”
or similar security theater.

### 9.4 Recovery phrase and later steps

The existing display, clipboard timeout, three-word backup verification,
password form, `ImportMnemonic`, unlock, and dashboard redirect behavior remain.

When selecting the three backup-check positions, replace the current
`Math.random` usage with a small `crypto.getRandomValues`-based distinct-index
helper. This does not affect seed security but removes an unrelated non-
cryptographic random call from the wallet-creation audit surface.

If the user leaves the route before persistence:

- remove all listeners;
- clear interaction buffers;
- clear renderer contribution buffers;
- clear the mnemonic, word array, and answers from component state as a best-
  effort cleanup.

### 9.5 Accessibility

- Pointer use must never be mandatory.
- The collection target is reachable and operable by keyboard.
- Do not intercept Tab, Escape, or assistive-technology navigation globally.
- Key collection occurs only while the dedicated target has focus.
- Progress uses `aria-valuemin`, `aria-valuemax`, and `aria-valuenow`, with
  equivalent text.
- Ready/error states are announced through appropriate live regions.
- Status never relies on color alone.
- Respect `prefers-reduced-motion`; no decorative motion is required.

## 10. Pinia and Wails bindings

Regenerate bindings after the Go DTO and method exist:

```bash
GOWORK=off GOTOOLCHAIN=auto wails generate module
```

If local PATH requires it, use the repository-standard full Wails binary path.
Keep only relevant changes under `frontend/wailsjs/go/app/` and
`frontend/wailsjs/go/models.ts`; revert unrelated `frontend/wailsjs/runtime/`
churn caused by CLI-version skew.

Extend the wallet store with an action corresponding to:

```ts
generateMnemonicWithEntropy(request: MnemonicEntropyRequest): Promise<string>
```

Keep the existing `generateMnemonic()` action for the explicit backend-only
fallback.

The store must not retain the request, contributions, or mnemonic. It returns
the phrase to the component exactly as the existing action does.

## 11. Testing requirements

### 11.1 Go unit tests

Add focused tests in `app/wallet_service_test.go` or a dedicated nearby test
file.

Required deterministic helper tests:

- A frozen vector with fixed `B`, `R`, and `U` verifies the exact 32-byte HKDF
  output and resulting 24-word BIP-39 phrase.
- The frozen expected output must be hard-coded, not computed with the same
  helper during the assertion.
- Changing one byte of `B` changes the result.
- Changing one byte of `R` changes the result.
- Changing one byte of `U` changes the result.
- Output length is exactly 32 bytes.
- The helper rejects any input not exactly 32 bytes.

Required service tests:

- `GenerateMnemonicWithEntropy` returns a valid 24-word BIP-39 phrase.
- Two calls with identical frontend fields return different phrases because
  backend randomness is fresh.
- Unsupported request versions are rejected.
- Malformed Base64 in each field is rejected.
- Decoded lengths of 0, 31, and 33 bytes are rejected for each field.
- Errors do not include submitted Base64 values.
- Existing `GenerateMnemonic` remains valid and distinct across calls.
- Existing import/keystore round-trip tests continue to pass.

Do not add weak statistical randomness tests. Small-sample frequency tests do
not validate a cryptographic entropy source and create false confidence.

### 11.2 Frontend entropy-utility tests

Required tests:

- Pointer records use the locked 13-byte format and little-endian fields.
- Touch and keyboard event-kind values are correct.
- Two key events with different actual keys but the same timing serialize
  identically, proving key identities are not retained.
- Repeated keydown is ignored.
- Pointer throttling enforces the 16 ms interval.
- Non-finite/negative timing becomes zero; large timing clamps to `uint32`.
- Coordinates clamp to signed 32-bit values.
- The transcript stops at 2,048 records.
- Empty transcript SHA-256 matches the standard known digest.
- Renderer output is exactly 32 bytes.
- Base64 round-trips all byte values correctly.
- Cleanup overwrites mutable buffers and resets internal state.
- The distinct backup-position helper returns three unique in-range indexes.

Mock `crypto.getRandomValues` and `crypto.subtle.digest` deterministically in
Vitest. Tests must not depend on ambient jsdom randomness.

### 11.3 Create view tests

Extend `frontend/src/views/Create.test.ts` to verify:

- Mounting does not call either mnemonic-generation binding.
- `Generate without interaction` calls enhanced generation with a digest of the
  empty transcript, not the backend-only fallback.
- Starting collection attaches scoped listeners.
- Pointer and keyboard-only collection both reach ready state.
- Generate remains disabled until both participation thresholds are met.
- Skip remains available before readiness.
- Enhanced generation failure does not invoke `GenerateMnemonic` silently.
- Web Crypto failure exposes the explicit backend-only action.
- Successful generation advances to the existing phrase display.
- Leaving the route removes listeners and clears state.
- Existing backup verification, password, import, unlock, clipboard, and
  redirect tests remain green.

### 11.4 Manual acceptance

On macOS, Windows, and Linux release-target WebViews:

1. Open Create Wallet and verify that no phrase is shown or generated yet.
2. Complete pointer collection and create a wallet.
3. Complete keyboard-only collection and create a wallet.
4. Use the skip-interaction path and create a wallet.
5. Confirm all three produce 24 valid BIP-39 words.
6. Confirm the created keystore unlocks and derives the displayed base address.
7. Confirm at least one enhanced wallet opens in original Syrius.
8. Navigate away mid-collection and verify no listeners or stale progress remain
   on returning.
9. Exercise the explicit backend-only fallback by simulating Web Crypto
   unavailability in a development build.
10. Inspect application logs and network activity; no contribution, digest,
    mnemonic, or final entropy may appear.

## 12. Security review checklist

- [ ] Final code imports Go `crypto/rand`, `crypto/hkdf`, and `crypto/sha256`
      directly in the production generation path.
- [ ] No `math/rand` remains in wallet creation.
- [ ] No production RNG can be injected through a Wails method or mutable
      package-level variable.
- [ ] Enhanced generation cannot silently downgrade.
- [ ] Every frontend contribution is treated as untrusted and fixed-length
      validated in Go.
- [ ] HKDF source order, salt, info, and output length match this spec exactly.
- [ ] Frozen test vectors make any construction change visible in review.
- [ ] No event records actual key identity or character data.
- [ ] Collection listeners are component-scoped and removed on every exit path.
- [ ] Mutable buffers are cleared best-effort on both sides.
- [ ] No entropy material or mnemonic is logged or persisted outside the
      encrypted keystore.
- [ ] UI makes no quantified claim about interaction entropy.
- [ ] ImportMnemonic remains unchanged and does not mix imported phrases.
- [ ] Keystore and address compatibility gates pass.
- [ ] The final built-artifact call path is reviewed, not only the presence of
      the intended RNG implementation in source.

## 13. Expected files

Expected modifications:

```text
app/dto.go
app/wallet_service.go
app/wallet_service_test.go
frontend/src/lib/wallet-entropy.ts
frontend/src/lib/wallet-entropy.test.ts
frontend/src/stores/wallet.ts
frontend/src/stores/wallet.test.ts
frontend/src/views/Create.vue
frontend/src/views/Create.test.ts
frontend/wailsjs/go/app/WalletService.d.ts
frontend/wailsjs/go/app/WalletService.js
frontend/wailsjs/go/models.ts
plan.md
docs/superpowers/specs/2026-06-21-phase-3-wallet-lifecycle-design.md
```

The historical Phase 3 spec should receive only a short addendum link stating
that this Phase 7f spec supersedes its new-wallet entropy-generation step. Do
not rewrite Phase 3 history as though enhanced entropy shipped originally.

No SDK, go-zenon, transaction, signer, address-derivation, or keystore-format
files should change.

## 14. Implementation order

1. Add this feature to `plan.md` Phase 7f and link this addendum from the Phase 3
   spec.
2. Write failing deterministic Go combiner and validation tests.
3. Implement the backend DTO, HKDF helper, enhanced method, and direct
   backend-only generation path.
4. Run focused Go tests and inspect the exact crypto call path.
5. Regenerate Wails bindings; discard unrelated generated runtime churn.
6. Write failing frontend entropy-utility tests.
7. Implement the canonical collector/digest utility.
8. Extend wallet-store bindings and tests.
9. Write failing Create-view state/cleanup/accessibility tests.
10. Replace mount-time generation with the new explicit entropy flow.
11. Run backend and frontend full gates.
12. Perform the cross-platform manual acceptance and original-Syrius
    compatibility check.
13. Record acceptance in the Phase 7f acceptance document.

## 15. Verification commands

From the repository root:

```bash
GOWORK=off GOTOOLCHAIN=auto gofmt -w app/dto.go app/wallet_service.go app/wallet_service_test.go
GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'Test.*Mnemonic.*Entropy|TestGenerateMnemonic' -v
GOWORK=off GOTOOLCHAIN=auto go test ./...
GOWORK=off GOTOOLCHAIN=auto go vet ./...
GOWORK=off GOTOOLCHAIN=auto go build ./...
GOWORK=off GOTOOLCHAIN=auto wails generate module
```

From `frontend/`:

```bash
pnpm test
pnpm run typecheck
pnpm run build
```

Also run the repository's Phase 7 security gates if the implementation is being
accepted as part of the release security pass:

```bash
bash scripts/govulncheck-gate.sh
gosec -conf .gosec.json ./...
```

## 16. Acceptance criteria

The feature is complete only when all of the following are true:

- Wallet creation no longer generates a phrase at component mount.
- A user can add pointer, touch, or key-timing interaction before generation.
- A user can skip interaction without losing backend or renderer CSPRNG input.
- The backend combines three fixed 32-byte values with the exact versioned
  HKDF-SHA-256 construction.
- Weak or all-zero frontend contributions still leave backend CSPRNG security
  intact.
- Enhanced-generation errors never silently fall back.
- The resulting phrase is a standard 24-word BIP-39 mnemonic.
- Existing `ImportMnemonic`, keystore format, derivation, and address behavior
  are unchanged.
- A generated wallet opens in original Syrius and derives the same address.
- Automated backend/frontend tests and all build/security gates pass.
- Manual acceptance passes on macOS, Windows, and Linux target WebViews.
- Security review confirms no key identities, entropy inputs, or mnemonics leak
  through logs, persistence, telemetry, or network traffic.

## 17. References

- Coinkite, “Technical Deep Dive into the Entropy Issue”:
  https://blog.coinkite.com/entropy-technical-backgrounder/
- RFC 5869, HMAC-based Extract-and-Expand Key Derivation Function:
  https://www.rfc-editor.org/rfc/rfc5869.html
- NIST SP 800-90B, Recommendation for Entropy Sources Used for Random Bit
  Generation: https://csrc.nist.gov/pubs/sp/800/90/b/final
- W3C Web Cryptography API, `getRandomValues`:
  https://www.w3.org/TR/WebCryptoAPI/
- BIP-39 mnemonic specification:
  https://github.com/bitcoin/bips/blob/master/bip-0039.mediawiki
- Go `crypto/rand`: https://pkg.go.dev/crypto/rand
- Go `crypto/hkdf`: https://pkg.go.dev/crypto/hkdf

