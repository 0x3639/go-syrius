# Additive User Entropy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional interaction-entropy step to wallet creation that mixes backend `crypto/rand`, renderer `crypto.getRandomValues`, and a SHA-256 digest of a bounded interaction transcript via HKDF-SHA-256 into standard 24-word BIP-39 entropy.

**Architecture:** The Go backend gains a `GenerateMnemonicWithEntropy(MnemonicEntropyRequest)` Wails binding that treats both frontend contributions as untrusted fixed-length inputs and combines them with a fresh backend `crypto/rand` value using the locked HKDF construction. The frontend gains a `wallet-entropy.ts` utility (canonical 13-byte transcript records, digesting, Base64, cleanup) and a reworked `Create.vue` wizard that never generates a phrase at mount — the user explicitly chooses interaction, skip, or (only on Web Crypto failure) backend-only generation.

**Tech Stack:** Go 1.25.12 stdlib (`crypto/hkdf`, `crypto/rand`, `crypto/sha256`, `encoding/base64`), tyler-smith/go-bip39, Wails v2.12.0 bindings, Vue 3 + Pinia + Vitest.

**Spec:** `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md` — read it before starting any task. Section references below (§N) point into that spec.

## Global Constraints

- All Go commands need `GOWORK=off GOTOOLCHAIN=auto` prefixes (parent go.work references a missing sibling module).
- Frontend commands run in `frontend/` with pnpm 10.17.1.
- HKDF construction is locked (§6.2): `IKM = B || R || U` (exactly that order), salt `"go-syrius/bip39-entropy/v1/extract"`, info `"go-syrius/bip39-entropy/v1/output"`, output exactly 32 bytes. Never XOR, never concatenate into BIP-39 directly, never use `math/rand`.
- JSON field names are locked: `version`, `rendererRandomBase64`, `interactionDigestBase64`. `Version` must equal `1`. Base64 is padded RFC 4648 standard (`base64.StdEncoding` / `btoa`).
- Transcript record format is locked (§8.1): 13-byte little-endian records — kind u8 (1=pointer, 2=touch, 3=key), delta-µs u32, A i32, B i32. Pointer A/B = `round(client{X,Y} * 1024)` clamped to i32; key A=B=0. Key identity (`event.key`/`event.code`/modifiers) must NEVER be read.
- Collection constants are locked (§8.2): `POINTER_SAMPLE_INTERVAL_MS = 16`, `COLLECTION_MIN_DURATION_MS = 5_000`, `COLLECTION_TARGET_SAMPLES = 32`, `COLLECTION_MAX_SAMPLES = 2_048`.
- UI copy is locked (§9.2/§9.3) — use the exact button labels and body text quoted in the tasks below. Never show entropy-bit estimates or security-quality claims.
- No silent downgrade: enhanced-generation failure must never automatically invoke `GenerateMnemonic`; only the explicit `Generate using backend randomness` action may (§8.4, invariant 10).
- Never log, persist, or echo entropy inputs, digests, final entropy, or mnemonics (invariant 12). Errors must not contain submitted Base64 values.
- Clear mutable secret byte slices with `clear()` + `defer` in Go and `.fill(0)` in TS, best-effort (§6.4).
- Only files listed in spec §13 change. No SDK, go-zenon, signer, tx, or keystore-format files.
- Commit after every task. Note: gpg signing may need a re-warm on the first commit (pinentry prompt).

## Frozen test vector (pre-computed, independent implementation)

Generated once for this plan with a standalone Python HKDF-SHA-256 (RFC 5869 via `hmac`/`hashlib`) and a BIP-39 encoder using the wordlist from `go-bip39@v1.1.0` — NOT with the Go helper under test. Hard-code these in tests; never recompute them with `deriveMnemonicEntropy` inside an assertion.

```text
B    = 000102...1f            (bytes 0x00..0x1f)
R    = 202122...3f            (bytes 0x20..0x3f)
U    = SHA-256("")            = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
salt = "go-syrius/bip39-entropy/v1/extract"
info = "go-syrius/bip39-entropy/v1/output"
E    = 1f6f9ac930b0ca9a51fffd97c9d87709ffbd4957a7acc79fb343426e04fac74f
mnemonic(E) = buyer lamp rather gesture arrow essay elevator zero oak excite build become wink pigeon future void shy work speak luggage theory latin brush venue
```

Also used in frontend tests: padded standard Base64 of SHA-256("") is `47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=`.

---

### Task 1: plan.md Phase 7f entry + Phase 3 spec addendum

**Files:**
- Modify: `plan.md` (Phase 7 section, ~line 213)
- Modify: `docs/superpowers/specs/2026-06-21-phase-3-wallet-lifecycle-design.md` (addendum after the H1 title)

**Interfaces:**
- Consumes: nothing.
- Produces: docs only; no code contracts.

- [ ] **Step 1: Add the Phase 7f bullet to plan.md**

In `plan.md`, inside the "### Phase 7 — Hardening, packaging, release" checklist (after the "Security pass (§7)…" bullet), add:

```markdown
- [ ] Additive user-entropy wallet creation (7f hardening): optional interaction transcript + renderer CSPRNG mixed with backend `crypto/rand` via versioned HKDF-SHA-256; standard 24-word BIP-39 output, keystore/derivation unchanged. Spec: `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md`.
```

- [ ] **Step 2: Add the addendum to the Phase 3 spec**

In `docs/superpowers/specs/2026-06-21-phase-3-wallet-lifecycle-design.md`, insert immediately after the H1 title line (do not rewrite any other content — §13 forbids rewriting Phase 3 history):

```markdown
> **Addendum (2026-07-31):** The new-wallet entropy-generation step described in
> this spec is superseded by the Phase 7f additive user-entropy design
> (`2026-07-31-additive-user-entropy-design.md`): generation is no longer
> `bip39.NewEntropy` at wizard mount, but an explicit user action combining
> backend `crypto/rand`, renderer CSPRNG, and an optional interaction digest via
> HKDF-SHA-256. Everything else here (keystore, import, reveal) is unchanged.
```

- [ ] **Step 3: Commit**

```bash
git add plan.md docs/superpowers/specs/2026-06-21-phase-3-wallet-lifecycle-design.md docs/superpowers/plans/2026-07-31-additive-user-entropy.md
git commit -m "docs: record Phase 7f additive user-entropy design in plan.md + Phase 3 addendum"
```

---

### Task 2: Go deterministic entropy combiner (`deriveMnemonicEntropy` + `mnemonicFromEntropy`)

**Files:**
- Modify: `app/wallet_service.go` (helpers + constants; imports)
- Test: `app/wallet_service_test.go`

**Interfaces:**
- Consumes: Go stdlib `crypto/hkdf`, `crypto/sha256`; `bip39.NewMnemonic`.
- Produces (used by Task 3):
  - `deriveMnemonicEntropy(backend, renderer, interaction []byte) ([]byte, error)` — unexported, deterministic, rejects any input ≠ 32 bytes.
  - `mnemonicFromEntropy(entropy []byte) (string, error)` — unexported, thin `bip39.NewMnemonic` wrapper.
  - Constants: `entropyRequestVersion = 1`, `entropyByteLen = 32`, `entropySalt`, `entropyInfo`.

- [ ] **Step 1: Write the failing tests**

Append to `app/wallet_service_test.go` (add `"bytes"`, `"crypto/sha256"`, `"encoding/hex"` to its imports):

```go
// --- Phase 7f: additive user entropy (spec 2026-07-31) ---

// Frozen vector computed once with an independent RFC 5869 implementation.
// If this test fails, the locked HKDF construction changed — that is a
// security-review event, not a test to update casually.
func TestDeriveMnemonicEntropyFrozenVector(t *testing.T) {
	backend := make([]byte, 32)
	renderer := make([]byte, 32)
	for i := range backend {
		backend[i] = byte(i)
		renderer[i] = byte(0x20 + i)
	}
	interaction, _ := hex.DecodeString("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") // SHA-256("")

	got, err := deriveMnemonicEntropy(backend, renderer, interaction)
	if err != nil {
		t.Fatalf("deriveMnemonicEntropy: %v", err)
	}
	if len(got) != 32 {
		t.Fatalf("entropy length = %d, want 32", len(got))
	}
	const wantHex = "1f6f9ac930b0ca9a51fffd97c9d87709ffbd4957a7acc79fb343426e04fac74f"
	if hex.EncodeToString(got) != wantHex {
		t.Fatalf("entropy = %x, want %s", got, wantHex)
	}

	m, err := mnemonicFromEntropy(got)
	if err != nil {
		t.Fatalf("mnemonicFromEntropy: %v", err)
	}
	const wantMnemonic = "buyer lamp rather gesture arrow essay elevator zero oak excite build become wink pigeon future void shy work speak luggage theory latin brush venue"
	if m != wantMnemonic {
		t.Fatalf("mnemonic = %q, want %q", m, wantMnemonic)
	}
}

func TestDeriveMnemonicEntropyEachInputMatters(t *testing.T) {
	base := func() (b, r, u []byte) {
		b, r, u = make([]byte, 32), make([]byte, 32), make([]byte, 32)
		for i := 0; i < 32; i++ {
			b[i], r[i], u[i] = byte(i), byte(0x20+i), byte(0x40+i)
		}
		return
	}
	b, r, u := base()
	ref, err := deriveMnemonicEntropy(b, r, u)
	if err != nil {
		t.Fatal(err)
	}
	flip := []struct {
		name string
		mut  func(b, r, u []byte)
	}{
		{"backend", func(b, _, _ []byte) { b[7] ^= 0x01 }},
		{"renderer", func(_, r, _ []byte) { r[7] ^= 0x01 }},
		{"interaction", func(_, _, u []byte) { u[7] ^= 0x01 }},
	}
	for _, tc := range flip {
		b, r, u := base()
		tc.mut(b, r, u)
		got, err := deriveMnemonicEntropy(b, r, u)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if bytes.Equal(got, ref) {
			t.Fatalf("flipping one %s byte did not change the output", tc.name)
		}
	}
}

func TestDeriveMnemonicEntropyRejectsBadLengths(t *testing.T) {
	good := make([]byte, 32)
	for _, n := range []int{0, 31, 33} {
		bad := make([]byte, n)
		if _, err := deriveMnemonicEntropy(bad, good, good); err == nil {
			t.Fatalf("backend len %d accepted", n)
		}
		if _, err := deriveMnemonicEntropy(good, bad, good); err == nil {
			t.Fatalf("renderer len %d accepted", n)
		}
		if _, err := deriveMnemonicEntropy(good, good, bad); err == nil {
			t.Fatalf("interaction len %d accepted", n)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'TestDeriveMnemonicEntropy' -v
```

Expected: compile error — `undefined: deriveMnemonicEntropy` / `mnemonicFromEntropy`.

- [ ] **Step 3: Implement the helpers**

In `app/wallet_service.go`, add `"crypto/hkdf"` and `"crypto/sha256"` to the imports, then add (near `GenerateMnemonic`, ~line 250):

```go
// Locked domain-separation values for the Phase 7f additive-entropy
// construction (spec 2026-07-31 §6.2). Changing any of these changes every
// generated wallet — they are version-pinned on purpose.
const (
	entropyRequestVersion = 1
	entropyByteLen        = 32
	entropySalt           = "go-syrius/bip39-entropy/v1/extract"
	entropyInfo           = "go-syrius/bip39-entropy/v1/output"
)

// deriveMnemonicEntropy combines the backend crypto/rand value with the two
// untrusted renderer contributions using HKDF-SHA-256 (RFC 5869). All three
// inputs must be exactly 32 bytes; fixed lengths remove concatenation
// ambiguity. A weak or attacker-chosen renderer input cannot cancel the
// backend input under this extractor.
func deriveMnemonicEntropy(backend, renderer, interaction []byte) ([]byte, error) {
	if len(backend) != entropyByteLen || len(renderer) != entropyByteLen || len(interaction) != entropyByteLen {
		return nil, errors.New("entropy inputs must be exactly 32 bytes")
	}
	ikm := make([]byte, 0, 3*entropyByteLen)
	ikm = append(ikm, backend...)
	ikm = append(ikm, renderer...)
	ikm = append(ikm, interaction...)
	defer clear(ikm)
	return hkdf.Key(sha256.New, ikm, []byte(entropySalt), entropyInfo, entropyByteLen)
}

// mnemonicFromEntropy is the single BIP-39 conversion seam shared by both
// generation paths.
func mnemonicFromEntropy(entropy []byte) (string, error) {
	return bip39.NewMnemonic(entropy)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'TestDeriveMnemonicEntropy' -v
```

Expected: all three tests PASS. If the frozen vector fails, the implementation deviates from §6.2 — fix the implementation, never the vector.

- [ ] **Step 5: Commit**

```bash
git add app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(wallet): locked HKDF-SHA-256 entropy combiner with frozen vector tests"
```

---

### Task 3: Backend DTO, `GenerateMnemonicWithEntropy`, direct `GenerateMnemonic`

**Files:**
- Modify: `app/dto.go` (new DTO at end of file)
- Modify: `app/wallet_service.go` (new method; rework `GenerateMnemonic`)
- Test: `app/wallet_service_test.go`

**Interfaces:**
- Consumes: `deriveMnemonicEntropy`, `mnemonicFromEntropy`, constants from Task 2.
- Produces (used by Tasks 5–6 via Wails bindings):
  - `type MnemonicEntropyRequest struct { Version int; RendererRandomBase64 string; InteractionDigestBase64 string }` with JSON names `version` / `rendererRandomBase64` / `interactionDigestBase64`.
  - `func (w *WalletService) GenerateMnemonicWithEntropy(req MnemonicEntropyRequest) (string, error)`
  - `func (w *WalletService) GenerateMnemonic() (string, error)` — unchanged signature, now reads `crypto/rand` directly.

- [ ] **Step 1: Write the failing service tests**

Append to `app/wallet_service_test.go` (add `"encoding/base64"` to imports):

```go
func validEntropyRequest() MnemonicEntropyRequest {
	renderer := make([]byte, 32)
	interaction := make([]byte, 32)
	for i := 0; i < 32; i++ {
		renderer[i], interaction[i] = byte(i), byte(0x80+i)
	}
	return MnemonicEntropyRequest{
		Version:                 1,
		RendererRandomBase64:    base64.StdEncoding.EncodeToString(renderer),
		InteractionDigestBase64: base64.StdEncoding.EncodeToString(interaction),
	}
}

func TestGenerateMnemonicWithEntropyValid(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.GenerateMnemonicWithEntropy(validEntropyRequest())
	if err != nil {
		t.Fatalf("GenerateMnemonicWithEntropy: %v", err)
	}
	if n := len(strings.Fields(m)); n != 24 {
		t.Fatalf("expected 24 words, got %d", n)
	}
	if !bip39.IsMnemonicValid(m) {
		t.Fatalf("invalid BIP-39 mnemonic: %q", m)
	}
}

// Identical frontend fields must still yield distinct phrases: the backend
// draws fresh crypto/rand per call (spec invariant 1).
func TestGenerateMnemonicWithEntropyFreshBackendRandomness(t *testing.T) {
	w := newTestWalletService(t)
	req := validEntropyRequest()
	m1, err1 := w.GenerateMnemonicWithEntropy(req)
	m2, err2 := w.GenerateMnemonicWithEntropy(req)
	if err1 != nil || err2 != nil {
		t.Fatalf("errs: %v, %v", err1, err2)
	}
	if m1 == m2 {
		t.Fatal("identical mnemonics for identical frontend input — backend randomness not fresh")
	}
}

func TestGenerateMnemonicWithEntropyRejectsBadVersion(t *testing.T) {
	w := newTestWalletService(t)
	for _, v := range []int{0, 2, -1} {
		req := validEntropyRequest()
		req.Version = v
		if _, err := w.GenerateMnemonicWithEntropy(req); err == nil {
			t.Fatalf("version %d accepted", v)
		}
	}
}

func TestGenerateMnemonicWithEntropyRejectsBadEncoding(t *testing.T) {
	w := newTestWalletService(t)
	req := validEntropyRequest()
	req.RendererRandomBase64 = "%%%not-base64%%%"
	if _, err := w.GenerateMnemonicWithEntropy(req); err == nil {
		t.Fatal("malformed renderer base64 accepted")
	}
	req = validEntropyRequest()
	req.InteractionDigestBase64 = "%%%not-base64%%%"
	if _, err := w.GenerateMnemonicWithEntropy(req); err == nil {
		t.Fatal("malformed interaction base64 accepted")
	}
}

func TestGenerateMnemonicWithEntropyRejectsBadLengths(t *testing.T) {
	w := newTestWalletService(t)
	for _, n := range []int{0, 31, 33} {
		enc := base64.StdEncoding.EncodeToString(make([]byte, n))
		req := validEntropyRequest()
		req.RendererRandomBase64 = enc
		if _, err := w.GenerateMnemonicWithEntropy(req); err == nil {
			t.Fatalf("renderer length %d accepted", n)
		}
		req = validEntropyRequest()
		req.InteractionDigestBase64 = enc
		if _, err := w.GenerateMnemonicWithEntropy(req); err == nil {
			t.Fatalf("interaction length %d accepted", n)
		}
	}
}

// Errors must never echo submitted contribution material (spec §7.3).
func TestGenerateMnemonicWithEntropyErrorsOmitInputs(t *testing.T) {
	w := newTestWalletService(t)
	payload := base64.StdEncoding.EncodeToString(make([]byte, 31))
	req := validEntropyRequest()
	req.RendererRandomBase64 = payload
	_, err := w.GenerateMnemonicWithEntropy(req)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), payload) || strings.Contains(err.Error(), req.InteractionDigestBase64) {
		t.Fatalf("error echoes submitted input: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'TestGenerateMnemonicWithEntropy' -v
```

Expected: compile error — `undefined: MnemonicEntropyRequest` / `GenerateMnemonicWithEntropy`.

- [ ] **Step 3: Add the DTO**

Append to `app/dto.go`:

```go
// MnemonicEntropyRequest carries the renderer's two additive entropy
// contributions for enhanced wallet creation (Phase 7f, spec 2026-07-31 §7.1).
// Both fields are UNTRUSTED input: padded standard Base64 that must decode to
// exactly 32 bytes. JSON names are part of the locked frontend contract.
type MnemonicEntropyRequest struct {
	Version                 int    `json:"version"`
	RendererRandomBase64    string `json:"rendererRandomBase64"`
	InteractionDigestBase64 string `json:"interactionDigestBase64"`
}
```

- [ ] **Step 4: Implement the enhanced method and rework `GenerateMnemonic`**

In `app/wallet_service.go`, add `"crypto/rand"` and `"encoding/base64"` to the imports. Replace the existing `GenerateMnemonic` body (currently `bip39.NewEntropy(256)`, ~line 253) and add the new method:

```go
// GenerateMnemonic returns a fresh 24-word (256-bit) BIP-39 mnemonic from OS
// randomness alone. It persists nothing — the create wizard shows it for
// backup before calling ImportMnemonic. Kept as the deliberate, user-chosen
// fallback when Web Crypto is unavailable in the renderer (spec §8.4); reads
// crypto/rand directly so the audited call path has no replaceable io.Reader
// seam (spec §7.2).
func (w *WalletService) GenerateMnemonic() (string, error) {
	var entropy [entropyByteLen]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	defer clear(entropy[:])
	return mnemonicFromEntropy(entropy[:])
}

// GenerateMnemonicWithEntropy combines fresh backend crypto/rand with two
// untrusted renderer contributions (Web Crypto random + interaction-transcript
// digest) via the locked HKDF-SHA-256 construction and returns a standard
// 24-word BIP-39 mnemonic (spec 2026-07-31 §6–7). Backend randomness is drawn
// after the request arrives and never returned to the renderer. Stateless;
// persists nothing; never logs or echoes inputs.
func (w *WalletService) GenerateMnemonicWithEntropy(req MnemonicEntropyRequest) (string, error) {
	if req.Version != entropyRequestVersion {
		return "", errors.New("unsupported entropy request version")
	}
	renderer, err := base64.StdEncoding.DecodeString(req.RendererRandomBase64)
	if err != nil {
		return "", errors.New("invalid renderer entropy encoding")
	}
	defer clear(renderer)
	if len(renderer) != entropyByteLen {
		return "", errors.New("renderer entropy must be 32 bytes")
	}
	interaction, err := base64.StdEncoding.DecodeString(req.InteractionDigestBase64)
	if err != nil {
		return "", errors.New("invalid interaction entropy encoding")
	}
	defer clear(interaction)
	if len(interaction) != entropyByteLen {
		return "", errors.New("interaction entropy must be 32 bytes")
	}
	var backend [entropyByteLen]byte
	if _, err := rand.Read(backend[:]); err != nil {
		return "", fmt.Errorf("backend randomness: %w", err)
	}
	defer clear(backend[:])
	entropy, err := deriveMnemonicEntropy(backend[:], renderer, interaction)
	if err != nil {
		return "", fmt.Errorf("derive mnemonic entropy: %w", err)
	}
	defer clear(entropy)
	m, err := mnemonicFromEntropy(entropy)
	if err != nil {
		return "", fmt.Errorf("create mnemonic: %w", err)
	}
	return m, nil
}
```

- [ ] **Step 5: Run the focused tests, then the full backend gates**

```bash
GOWORK=off GOTOOLCHAIN=auto gofmt -w app/dto.go app/wallet_service.go app/wallet_service_test.go
GOWORK=off GOTOOLCHAIN=auto go test ./app -run 'Test.*Mnemonic' -v
GOWORK=off GOTOOLCHAIN=auto go test ./...
GOWORK=off GOTOOLCHAIN=auto go vet ./...
GOWORK=off GOTOOLCHAIN=auto go build ./...
```

Expected: all PASS (including pre-existing `TestGenerateMnemonic24Words` and `TestImportMnemonicRoundTrip` — the import/keystore path is untouched). Manually inspect the diff: the production path must touch only `crypto/rand`, `crypto/hkdf`, `crypto/sha256`, `encoding/base64`, `bip39.NewMnemonic` — no `math/rand`, no injectable reader (spec §12).

- [ ] **Step 6: Commit**

```bash
git add app/dto.go app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(wallet): GenerateMnemonicWithEntropy with validated untrusted renderer inputs"
```

---

### Task 4: Frontend entropy utility (`wallet-entropy.ts`)

**Files:**
- Create: `frontend/src/lib/wallet-entropy.ts`
- Test: `frontend/src/lib/wallet-entropy.test.ts`

**Interfaces:**
- Consumes: `globalThis.crypto` (getRandomValues, subtle.digest), `btoa`, `performance` timestamps supplied by callers.
- Produces (used by Tasks 5–6):
  - Constants `POINTER_SAMPLE_INTERVAL_MS`, `COLLECTION_MIN_DURATION_MS`, `COLLECTION_TARGET_SAMPLES`, `COLLECTION_MAX_SAMPLES`, `KIND_POINTER=1`, `KIND_TOUCH=2`, `KIND_KEY=3`.
  - `type EntropyRequest = { version: number; rendererRandomBase64: string; interactionDigestBase64: string }`
  - `class InteractionCollector` — `addPointerSample(x, y, nowMs, pointerType?): boolean`, `addKeySample(nowMs, repeat?): boolean`, `get sampleCount(): number`, `get isFull(): boolean`, `freeze(): Uint8Array`, `reset(): void`.
  - `webCryptoAvailable(): boolean`
  - `createEntropyRequest(transcript: Uint8Array): Promise<EntropyRequest>` — digests, draws renderer random, Base64s, wipes buffers; throws on Web Crypto failure.
  - `toBase64(bytes: Uint8Array): string`, `wipe(...buffers: Uint8Array[]): void`
  - `pickDistinctIndexes(n: number, max: number): number[]`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/lib/wallet-entropy.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createHash } from 'node:crypto'
import {
  InteractionCollector,
  createEntropyRequest,
  pickDistinctIndexes,
  toBase64,
  wipe,
  webCryptoAvailable,
  POINTER_SAMPLE_INTERVAL_MS,
  COLLECTION_MAX_SAMPLES,
  KIND_POINTER,
  KIND_TOUCH,
  KIND_KEY,
} from './wallet-entropy'

const EMPTY_SHA256_B64 = '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='

// Deterministic Web Crypto stub (spec §11.2: no ambient jsdom randomness).
// getRandomValues MUST vary across calls — a constant stub would spin
// pickDistinctIndexes' rejection-sampling loop forever.
let randCtr = 0
function stubWebCrypto() {
  randCtr = 0
  vi.stubGlobal('crypto', {
    getRandomValues: (arr: ArrayBufferView) => {
      const bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength)
      for (let i = 0; i < bytes.length; i++) bytes[i] = randCtr++ & 0xff
      return arr
    },
    subtle: {
      digest: async (_alg: string, data: Uint8Array) => {
        const h = createHash('sha256')
          .update(Buffer.from(data.buffer, data.byteOffset, data.byteLength))
          .digest()
        return h.buffer.slice(h.byteOffset, h.byteOffset + h.byteLength)
      },
    },
  })
}

beforeEach(stubWebCrypto)
afterEach(() => vi.unstubAllGlobals())

function view(transcript: Uint8Array) {
  return new DataView(transcript.buffer, transcript.byteOffset, transcript.byteLength)
}

describe('InteractionCollector record format', () => {
  it('encodes a pointer record as 13 little-endian bytes', () => {
    const c = new InteractionCollector()
    expect(c.addPointerSample(10.4, 20.6, 100)).toBe(true)
    expect(c.addPointerSample(30, 40, 120)).toBe(true)
    const t = c.freeze()
    expect(t.length).toBe(26)
    const v = view(t)
    expect(v.getUint8(0)).toBe(KIND_POINTER)
    expect(v.getUint32(1, true)).toBe(0) // first record has no previous sample
    expect(v.getInt32(5, true)).toBe(Math.round(10.4 * 1024)) // 10650
    expect(v.getInt32(9, true)).toBe(Math.round(20.6 * 1024)) // 21094
    // second record: 20 ms after the previous accepted sample = 20000 µs
    expect(v.getUint8(13)).toBe(KIND_POINTER)
    expect(v.getUint32(14, true)).toBe(20_000)
  })

  it('uses kind 2 for touch and kind 3 for keys with zeroed positions', () => {
    const c = new InteractionCollector()
    c.addPointerSample(1, 2, 0, 'touch')
    c.addKeySample(20)
    const t = c.freeze()
    const v = view(t)
    expect(v.getUint8(0)).toBe(KIND_TOUCH)
    expect(v.getUint8(13)).toBe(KIND_KEY)
    expect(v.getInt32(18, true)).toBe(0)
    expect(v.getInt32(22, true)).toBe(0)
  })

  it('serializes identically for different keys with the same timing (no key identity)', () => {
    // The API cannot even receive key identity; equal timings ⇒ equal bytes.
    const a = new InteractionCollector()
    a.addKeySample(5)
    a.addKeySample(30)
    const b = new InteractionCollector()
    b.addKeySample(5)
    b.addKeySample(30)
    expect(a.freeze()).toEqual(b.freeze())
  })

  it('ignores repeated keydown', () => {
    const c = new InteractionCollector()
    expect(c.addKeySample(5, true)).toBe(false)
    expect(c.sampleCount).toBe(0)
  })

  it('throttles pointer samples to one per 16 ms; keys are not throttled', () => {
    const c = new InteractionCollector()
    expect(c.addPointerSample(1, 1, 0)).toBe(true)
    expect(c.addPointerSample(2, 2, 10)).toBe(false)
    expect(c.addPointerSample(3, 3, POINTER_SAMPLE_INTERVAL_MS)).toBe(true)
    expect(c.addKeySample(POINTER_SAMPLE_INTERVAL_MS + 1)).toBe(true)
    expect(c.addKeySample(POINTER_SAMPLE_INTERVAL_MS + 2)).toBe(true)
    expect(c.sampleCount).toBe(4)
  })

  it('encodes non-finite/negative timing as zero and clamps large deltas to uint32', () => {
    // Order matters: the NaN sample must come last — it poisons lastSampleMs,
    // so any record after it would encode delta 0 regardless of clamping.
    const c = new InteractionCollector()
    c.addKeySample(1000)
    c.addKeySample(500) // negative delta -> 0
    c.addKeySample(500 + 5_000_000) // 5e9 µs > uint32 max -> clamp
    c.addKeySample(Number.NaN) // non-finite -> 0
    const v = view(c.freeze())
    expect(v.getUint32(14, true)).toBe(0)
    expect(v.getUint32(27, true)).toBe(4_294_967_295)
    expect(v.getUint32(40, true)).toBe(0)
  })

  it('clamps coordinates to the signed 32-bit range', () => {
    const c = new InteractionCollector()
    c.addPointerSample(3e9, -3e9, 0)
    const v = view(c.freeze())
    expect(v.getInt32(5, true)).toBe(2_147_483_647)
    expect(v.getInt32(9, true)).toBe(-2_147_483_648)
  })

  it('stops appending at 2,048 records', () => {
    const c = new InteractionCollector()
    for (let i = 0; i < COLLECTION_MAX_SAMPLES + 10; i++) c.addKeySample(i * 2)
    expect(c.sampleCount).toBe(COLLECTION_MAX_SAMPLES)
    expect(c.isFull).toBe(true)
    expect(c.addKeySample(99_999)).toBe(false)
    expect(c.freeze().length).toBe(COLLECTION_MAX_SAMPLES * 13)
  })

  it('reset wipes the buffer and restarts state', () => {
    const c = new InteractionCollector()
    c.addPointerSample(9, 9, 0)
    const before = c.freeze()
    expect(before.some((b) => b !== 0)).toBe(true)
    c.reset()
    expect(c.sampleCount).toBe(0)
    expect(c.isFull).toBe(false)
    expect(c.freeze().length).toBe(0)
    // reset also restarts throttle/delta state: a new first sample is accepted
    c.reset()
    expect(c.addPointerSample(1, 1, 5)).toBe(true)
  })
})

describe('createEntropyRequest', () => {
  it('digests the empty transcript to the standard SHA-256 value', async () => {
    const req = await createEntropyRequest(new Uint8Array(0))
    expect(req.version).toBe(1)
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
  })

  it('produces a 32-byte renderer value in padded base64', async () => {
    const req = await createEntropyRequest(new Uint8Array(0))
    const decoded = atob(req.rendererRandomBase64)
    expect(decoded.length).toBe(32)
    expect(req.rendererRandomBase64.endsWith('=')).toBe(true)
  })

  it('wipes the caller transcript buffer even on success', async () => {
    const transcript = new Uint8Array([1, 2, 3, 4])
    await createEntropyRequest(transcript)
    expect([...transcript]).toEqual([0, 0, 0, 0])
  })

  it('throws when Web Crypto is unavailable', async () => {
    vi.stubGlobal('crypto', undefined)
    expect(webCryptoAvailable()).toBe(false)
    await expect(createEntropyRequest(new Uint8Array(0))).rejects.toThrow()
  })
})

describe('encoding helpers', () => {
  it('round-trips all byte values through base64', () => {
    const all = new Uint8Array(256)
    for (let i = 0; i < 256; i++) all[i] = i
    const decoded = atob(toBase64(all))
    expect(decoded.length).toBe(256)
    for (let i = 0; i < 256; i++) expect(decoded.charCodeAt(i)).toBe(i)
  })

  it('wipe zeroes every buffer passed', () => {
    const a = new Uint8Array([1, 2])
    const b = new Uint8Array([3])
    wipe(a, b)
    expect([...a]).toEqual([0, 0])
    expect([...b]).toEqual([0])
  })
})

describe('pickDistinctIndexes', () => {
  it('returns three unique in-range sorted indexes', () => {
    const idx = pickDistinctIndexes(3, 24)
    expect(idx).toHaveLength(3)
    expect(new Set(idx).size).toBe(3)
    for (const i of idx) expect(i >= 0 && i < 24).toBe(true)
    expect([...idx].sort((x, y) => x - y)).toEqual(idx)
  })

  it('works without Web Crypto via the deterministic spread fallback', () => {
    vi.stubGlobal('crypto', undefined)
    expect(pickDistinctIndexes(3, 24)).toEqual([0, 11, 23])
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd frontend && pnpm test -- src/lib/wallet-entropy.test.ts
```

Expected: FAIL — cannot resolve `./wallet-entropy`.

- [ ] **Step 3: Implement the utility**

Create `frontend/src/lib/wallet-entropy.ts`:

```ts
// Phase 7f additive user entropy — canonical interaction transcript, digesting,
// and request assembly (spec docs/superpowers/specs/2026-07-31-additive-user-
// entropy-design.md §8). All values here are contributions mixed by the Go
// backend via HKDF; nothing in this file is a security estimate.

export const POINTER_SAMPLE_INTERVAL_MS = 16
export const COLLECTION_MIN_DURATION_MS = 5_000
export const COLLECTION_TARGET_SAMPLES = 32
export const COLLECTION_MAX_SAMPLES = 2_048

export const KIND_POINTER = 1
export const KIND_TOUCH = 2
export const KIND_KEY = 3

const RECORD_SIZE = 13
const INT32_MIN = -2_147_483_648
const INT32_MAX = 2_147_483_647
const UINT32_MAX = 4_294_967_295

// Matches the Go MnemonicEntropyRequest JSON contract (app/dto.go).
export type EntropyRequest = {
  version: number
  rendererRandomBase64: string
  interactionDigestBase64: string
}

function clampInt32(v: number): number {
  if (!Number.isFinite(v)) return 0
  return Math.min(INT32_MAX, Math.max(INT32_MIN, Math.round(v)))
}

// Non-finite or negative deltas encode as zero rather than throwing (spec §8.1).
function deltaMicros(nowMs: number, prevMs: number | null): number {
  if (prevMs === null) return 0
  const d = (nowMs - prevMs) * 1000
  if (!Number.isFinite(d) || d < 0) return 0
  return Math.min(UINT32_MAX, Math.round(d))
}

// Bounded transcript of fixed 13-byte little-endian records:
//   [0]=kind u8, [1..4]=µs since previous accepted sample u32,
//   [5..8]=A i32, [9..12]=B i32. Key records never carry key identity.
export class InteractionCollector {
  private buf = new Uint8Array(COLLECTION_MAX_SAMPLES * RECORD_SIZE)
  private dv = new DataView(this.buf.buffer)
  private count = 0
  private lastSampleMs: number | null = null
  private lastPointerMs: number | null = null
  private frozen = false

  get sampleCount(): number {
    return this.count
  }

  get isFull(): boolean {
    return this.count >= COLLECTION_MAX_SAMPLES
  }

  addPointerSample(x: number, y: number, nowMs: number, pointerType = 'mouse'): boolean {
    if (this.frozen || this.isFull) return false
    if (this.lastPointerMs !== null && nowMs - this.lastPointerMs < POINTER_SAMPLE_INTERVAL_MS) return false
    const kind = pointerType === 'touch' ? KIND_TOUCH : KIND_POINTER
    this.append(kind, deltaMicros(nowMs, this.lastSampleMs), clampInt32(x * 1024), clampInt32(y * 1024))
    this.lastPointerMs = nowMs
    this.lastSampleMs = nowMs
    return true
  }

  addKeySample(nowMs: number, repeat = false): boolean {
    if (this.frozen || this.isFull || repeat) return false
    this.append(KIND_KEY, deltaMicros(nowMs, this.lastSampleMs), 0, 0)
    this.lastSampleMs = nowMs
    return true
  }

  private append(kind: number, delta: number, a: number, b: number): void {
    const off = this.count * RECORD_SIZE
    this.dv.setUint8(off, kind)
    this.dv.setUint32(off + 1, delta, true)
    this.dv.setInt32(off + 5, a, true)
    this.dv.setInt32(off + 9, b, true)
    this.count++
  }

  /** Stop accepting events and return a copy of the exact transcript bytes. */
  freeze(): Uint8Array {
    this.frozen = true
    return this.buf.slice(0, this.count * RECORD_SIZE)
  }

  /** Best-effort wipe of the sample buffer; restarts all collection state. */
  reset(): void {
    this.buf.fill(0)
    this.count = 0
    this.lastSampleMs = null
    this.lastPointerMs = null
    this.frozen = false
  }
}

export function webCryptoAvailable(): boolean {
  const c = globalThis.crypto as Crypto | undefined
  return (
    !!c &&
    typeof c.getRandomValues === 'function' &&
    !!c.subtle &&
    typeof c.subtle.digest === 'function'
  )
}

export function toBase64(bytes: Uint8Array): string {
  let s = ''
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i])
  return btoa(s)
}

export function wipe(...buffers: Uint8Array[]): void {
  for (const b of buffers) b.fill(0)
}

/**
 * Digest the frozen transcript, draw the fresh 32-byte renderer contribution
 * (at finalization, never at mount — spec §8.3), and assemble the version-1
 * request. Mutable buffers, including the caller's transcript, are wiped in
 * `finally`. Throws when Web Crypto is unavailable or fails — the caller must
 * surface that and offer the explicit backend-only action, never silently
 * fall back (spec §8.4).
 */
export async function createEntropyRequest(transcript: Uint8Array): Promise<EntropyRequest> {
  let digest: Uint8Array | null = null
  let renderer: Uint8Array | null = null
  try {
    if (!webCryptoAvailable()) throw new Error('Enhanced generation is unavailable: this environment has no Web Crypto support')
    digest = new Uint8Array(await crypto.subtle.digest('SHA-256', transcript))
    renderer = crypto.getRandomValues(new Uint8Array(32))
    return {
      version: 1,
      rendererRandomBase64: toBase64(renderer),
      interactionDigestBase64: toBase64(digest),
    }
  } finally {
    if (digest) wipe(digest)
    if (renderer) wipe(renderer)
    wipe(transcript)
  }
}

/**
 * n distinct uniform indexes in [0, max), sorted ascending, via rejection
 * sampling over crypto.getRandomValues (replaces Math.random in the wallet-
 * creation audit surface — spec §9.4). Backup positions are a UX check, not
 * entropy: in the degenerate no-Web-Crypto environment (only reachable on the
 * explicit backend-only path) fixed spread positions are used instead.
 */
export function pickDistinctIndexes(n: number, max: number): number[] {
  if (n > max) throw new Error('not enough positions to pick from')
  const c = globalThis.crypto as Crypto | undefined
  if (!c || typeof c.getRandomValues !== 'function') {
    return Array.from({ length: n }, (_, i) => Math.floor((i * (max - 1)) / Math.max(1, n - 1)))
  }
  const picked = new Set<number>()
  const buf = new Uint32Array(1)
  const limit = Math.floor(0x1_0000_0000 / max) * max
  while (picked.size < n) {
    c.getRandomValues(buf)
    if (buf[0] >= limit) continue
    picked.add(buf[0] % max)
  }
  return [...picked].sort((a, b) => a - b)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd frontend && pnpm test -- src/lib/wallet-entropy.test.ts && pnpm run typecheck
```

Expected: all PASS; typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/wallet-entropy.ts frontend/src/lib/wallet-entropy.test.ts
git commit -m "feat(frontend): canonical interaction-entropy collector and request utility"
```

---

### Task 5: Wails bindings regeneration + wallet store action

**Files:**
- Modify (generated): `frontend/wailsjs/go/app/WalletService.d.ts`, `frontend/wailsjs/go/app/WalletService.js`, `frontend/wailsjs/go/models.ts`
- Modify: `frontend/src/stores/wallet.ts`
- Test: `frontend/src/stores/wallet.test.ts`

**Interfaces:**
- Consumes: Go `GenerateMnemonicWithEntropy` (Task 3); `EntropyRequest` type (Task 4).
- Produces (used by Task 6): wallet store action `generateMnemonicWithEntropy(request: EntropyRequest): Promise<string>`; existing `generateMnemonic()` kept.

- [ ] **Step 1: Regenerate bindings**

```bash
GOWORK=off GOTOOLCHAIN=auto wails generate module
```

(If `wails` is not on PATH, use `$(go env GOPATH)/bin/wails`.) Then inspect `git status` / `git diff frontend/wailsjs`:
- Keep: `GenerateMnemonicWithEntropy` additions in `WalletService.d.ts` (`export function GenerateMnemonicWithEntropy(arg1:app.MnemonicEntropyRequest):Promise<string>;`) and `WalletService.js`, plus the `app.MnemonicEntropyRequest` class in `models.ts` with fields `version`, `rendererRandomBase64`, `interactionDigestBase64`.
- Revert any unrelated churn, especially under `frontend/wailsjs/runtime/`: `git checkout -- frontend/wailsjs/runtime/` (spec §10).

- [ ] **Step 2: Write the failing store test**

In `frontend/src/stores/wallet.test.ts`, add to the hoisted mocks (next to the existing `GenerateMnemonic` mock at the top) and register it in the `vi.mock('../../wailsjs/go/app/WalletService', ...)` factory object:

```ts
const GenerateMnemonicWithEntropy = vi.hoisted(() => vi.fn().mockResolvedValue('e1 e2 e3'))
```

```ts
  GenerateMnemonicWithEntropy,
```

Then add the test case alongside the existing `generateMnemonic` test:

```ts
  it('passes the entropy request through unchanged and returns the phrase', async () => {
    const s = useWalletStore()
    const req = {
      version: 1,
      rendererRandomBase64: 'AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=',
      interactionDigestBase64: '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=',
    }
    expect(await s.generateMnemonicWithEntropy(req)).toBe('e1 e2 e3')
    expect(GenerateMnemonicWithEntropy).toHaveBeenCalledWith(req)
  })
```

(Match the surrounding file's way of obtaining the store instance — if existing tests use a shared `s`, reuse that pattern.)

- [ ] **Step 3: Run test to verify it fails**

```bash
cd frontend && pnpm test -- src/stores/wallet.test.ts
```

Expected: FAIL — `s.generateMnemonicWithEntropy is not a function`.

- [ ] **Step 4: Add the store action**

In `frontend/src/stores/wallet.ts`, add the import at the top:

```ts
import type { EntropyRequest } from '../lib/wallet-entropy'
```

and add directly below the existing `generateMnemonic()` action:

```ts
    // Enhanced Phase 7f generation: pass the renderer contributions through to
    // Go and return the phrase. Never retain the request, contributions, or
    // mnemonic in store state (spec §10).
    async generateMnemonicWithEntropy(request: EntropyRequest): Promise<string> {
      return await W.GenerateMnemonicWithEntropy(request)
    },
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd frontend && pnpm test -- src/stores/wallet.test.ts && pnpm run typecheck
```

Expected: PASS, typecheck clean. (If typecheck rejects passing the plain `EntropyRequest` where the generated `app.MnemonicEntropyRequest` class is expected, wrap the call as `W.GenerateMnemonicWithEntropy(app.MnemonicEntropyRequest.createFrom(request))` with `import { app } from '../../wailsjs/go/models'` — structural compatibility normally makes this unnecessary.)

- [ ] **Step 6: Commit**

```bash
git add frontend/wailsjs frontend/src/stores/wallet.ts frontend/src/stores/wallet.test.ts
git commit -m "feat(frontend): wallet-store entropy action + regenerated Wails bindings"
```

---

### Task 6: Create.vue explicit entropy flow

**Files:**
- Modify: `frontend/src/views/Create.vue` (full rework of script + new wizard stages)
- Test: `frontend/src/views/Create.test.ts` (full rewrite)

**Interfaces:**
- Consumes: wallet store `generateMnemonicWithEntropy` / `generateMnemonic` (Task 5); `InteractionCollector`, `createEntropyRequest`, `pickDistinctIndexes`, `webCryptoAvailable`, `COLLECTION_MIN_DURATION_MS`, `COLLECTION_TARGET_SAMPLES` (Task 4).
- Produces: user-facing wizard only; no downstream code consumers.

- [ ] **Step 1: Write the failing view tests**

Replace `frontend/src/views/Create.test.ts` with:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createHash } from 'node:crypto'

const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const GenerateMnemonicWithEntropy = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ id: 'abc.dat', name: 'New', baseAddress: 'z1' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  GenerateMnemonic,
  GenerateMnemonicWithEntropy,
  ImportMnemonic,
  Unlock,
  Lock: vi.fn(),
}))
const ClipboardSetText = vi.hoisted(() => vi.fn().mockResolvedValue(true))
const ClipboardGetText = vi.hoisted(() => vi.fn().mockResolvedValue(''))
vi.mock('../../wailsjs/runtime/runtime', () => ({ ClipboardSetText, ClipboardGetText }))
const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ autoReceive: true }))
const SetAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetAutoReceive }))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import Create from './Create.vue'

const EMPTY_SHA256_B64 = '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='

let randCtr = 0
function stubWebCrypto() {
  randCtr = 0
  vi.stubGlobal('crypto', {
    getRandomValues: (arr: ArrayBufferView) => {
      const bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength)
      for (let i = 0; i < bytes.length; i++) bytes[i] = randCtr++ & 0xff
      return arr
    },
    subtle: {
      digest: async (_alg: string, data: Uint8Array) => {
        const h = createHash('sha256')
          .update(Buffer.from(data.buffer, data.byteOffset, data.byteLength))
          .digest()
        return h.buffer.slice(h.byteOffset, h.byteOffset + h.byteLength)
      },
    },
  })
}

const flush = () => new Promise((r) => setTimeout(r))
const btn = (w: ReturnType<typeof mount>, label: string) =>
  w.findAll('button').find((b) => b.text() === label)

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  stubWebCrypto()
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  // The collection tests spy on performance.now; clearAllMocks alone would
  // leave a spy with no implementation installed for later tests.
  vi.restoreAllMocks()
})

describe('Create.vue — entropy choice', () => {
  it('does not generate any mnemonic at mount', async () => {
    const w = mount(Create)
    await flush()
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    expect(GenerateMnemonicWithEntropy).not.toHaveBeenCalled()
    expect(w.text()).toContain('Generate your recovery phrase')
    expect(btn(w, 'Add interaction randomness')).toBeTruthy()
    expect(btn(w, 'Generate without interaction')).toBeTruthy()
  })

  it('"Generate without interaction" uses enhanced generation with the empty-transcript digest', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    expect(GenerateMnemonicWithEntropy).toHaveBeenCalledTimes(1)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.version).toBe(1)
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
    expect(atob(req.rendererRandomBase64).length).toBe(32)
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    expect(w.text()).toContain('alpha')
  })

  it('enhanced-generation failure surfaces the error and never falls back silently', async () => {
    GenerateMnemonicWithEntropy.mockRejectedValueOnce(new Error('backend exploded'))
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(w.text()).toContain('backend exploded')
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    // still on the choice screen; user can retry
    expect(btn(w, 'Generate without interaction')).toBeTruthy()
  })

  it('Web Crypto unavailability exposes the explicit backend-only action', async () => {
    vi.stubGlobal('crypto', undefined)
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(GenerateMnemonic).not.toHaveBeenCalled() // no silent fallback
    const fallback = btn(w, 'Generate using backend randomness')
    expect(fallback).toBeTruthy()
    await fallback!.trigger('click')
    await flush()
    expect(GenerateMnemonic).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('alpha')
  })
})

describe('Create.vue — interaction collection', () => {
  function mountCollecting() {
    vi.useFakeTimers()
    let now = 0
    vi.spyOn(performance, 'now').mockImplementation(() => now)
    const w = mount(Create)
    const setNow = (v: number) => { now = v }
    return { w, setNow }
  }

  async function startCollection(w: ReturnType<typeof mount>) {
    await btn(w, 'Add interaction randomness')!.trigger('click')
    return w.find('[aria-label="interaction collection area"]')
  }

  it('starting collection shows the scoped target and a disabled generate button', async () => {
    const { w } = mountCollecting()
    const target = await startCollection(w)
    expect(target.exists()).toBe(true)
    expect(w.text()).toContain('not the keys you press')
    expect(w.text()).toContain('Do not type a password or recovery phrase')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    expect(btn(w, 'Skip interaction')!.attributes('disabled')).toBeUndefined()
  })

  it('pointer collection reaches ready only after samples AND duration; then generates', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    // 40 pointer moves spaced 20ms apart -> >32 accepted samples by t=800ms
    for (let i = 1; i <= 40; i++) {
      setNow(i * 20)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    // samples met, duration not met -> still disabled
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300) // elapsed timer tick
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.interactionDigestBase64).not.toBe(EMPTY_SHA256_B64) // real transcript digested
    expect(w.text()).toContain('alpha')
  })

  it('keyboard-only collection reaches ready state', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 32; i++) {
      setNow(i * 30)
      await target.trigger('keydown', { repeat: false })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
  })

  it('duration alone is not enough — sample target also gates readiness', async () => {
    const { w, setNow } = mountCollecting()
    await startCollection(w)
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
  })

  it('skip is available before readiness and sends the empty-transcript digest', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    setNow(20)
    await target.trigger('pointermove', { clientX: 5, clientY: 6, pointerType: 'mouse' })
    await btn(w, 'Skip interaction')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
    expect(w.text()).toContain('alpha')
  })

  it('events outside the collection target are not collected', async () => {
    const { w, setNow } = mountCollecting()
    await startCollection(w)
    setNow(100)
    // dispatch on document body, not the target — no scoped listener fires
    document.body.dispatchEvent(new Event('pointermove'))
    document.body.dispatchEvent(new Event('keydown'))
    expect(w.text()).toContain('0 samples')
  })

  it('unmounting mid-collection clears the elapsed timer and state', async () => {
    const { w } = mountCollecting()
    await startCollection(w)
    w.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('Create.vue — existing flow after generation', () => {
  it('walks phrase -> verify -> password -> import/unlock/dashboard', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    expect(w.text()).toContain('alpha')

    await btn(w, "I've backed it up")!.trigger('click')
    const words = ['alpha', 'bravo', 'charlie']
    for (const input of w.findAll('input')) {
      const label = input.attributes('aria-label') || ''
      const m = label.match(/^word (\d+)$/)
      if (m) await input.setValue(words[Number(m[1]) - 1])
    }
    await btn(w, 'Continue')!.trigger('click')
    await w.find('input[aria-label="wallet name"]').setValue('New')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await btn(w, 'Create wallet')!.trigger('click')
    await flush()

    expect(ImportMnemonic).toHaveBeenCalledWith('New', 'pw', 'alpha bravo charlie')
    expect(Unlock).toHaveBeenCalledWith('abc.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/dashboard')
    expect(SetAutoReceive).toHaveBeenCalledWith(false)
  })

  it('copies the seed phrase to the clipboard', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await w.findAll('button').find((b) => b.text().includes('Copy recovery phrase'))!.trigger('click')
    await flush()
    expect(ClipboardSetText).toHaveBeenCalledWith('alpha bravo charlie')
    expect(w.text()).toContain('Copied')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd frontend && pnpm test -- src/views/Create.test.ts
```

Expected: FAIL — mount-time generation still fires, no choice screen exists.

- [ ] **Step 3: Rework Create.vue**

Replace the `<script setup>` block and template of `frontend/src/views/Create.vue` with the following (the `copySeed`/`finish` bodies, clipboard TTL logic, and the phrase/verify/password markup are the existing ones, relocated behind stage guards):

```vue
<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import { ClipboardSetText, ClipboardGetText } from '../../wailsjs/runtime/runtime'
import { XIcon, TriangleAlertIcon, CopyIcon, CheckIcon } from '@lucide/vue'
import {
  InteractionCollector,
  createEntropyRequest,
  pickDistinctIndexes,
  webCryptoAvailable,
  COLLECTION_MIN_DURATION_MS,
  COLLECTION_TARGET_SAMPLES,
} from '../lib/wallet-entropy'

const wallet = useWalletStore()
const router = useRouter()

// Phase 7f (spec 2026-07-31 §9): no mnemonic may exist before an explicit
// generation action — there is deliberately no onMounted generation here.
type Stage = 'choice' | 'collect' | 'phrase' | 'verify' | 'password'
const stage = ref<Stage>('choice')
const generating = ref(false)
const cryptoUnavailable = ref(false)
const error = ref('')

const SEED_CLIPBOARD_TTL_MS = 45_000
const copied = ref(false)
const everCopied = ref(false)
const mnemonic = ref('')
const words = ref<string[]>([])
const positions = ref<number[]>([])
const answers = ref<Record<number, string>>({})
const name = ref('')
const password = ref('')
const confirm = ref('')

// --- interaction collection (spec §8, §9.3) ---
const collector = new InteractionCollector()
const sampleCount = ref(0)
const elapsedMs = ref(0)
let collectStart = 0
let elapsedTimer: ReturnType<typeof setInterval> | null = null

function startCollection() {
  error.value = ''
  if (!webCryptoAvailable()) {
    cryptoUnavailable.value = true
    error.value = 'Enhanced generation is unavailable: this environment has no Web Crypto support'
    return
  }
  collector.reset()
  sampleCount.value = 0
  elapsedMs.value = 0
  collectStart = performance.now()
  elapsedTimer = setInterval(() => {
    elapsedMs.value = performance.now() - collectStart
  }, 250)
  stage.value = 'collect'
}

function stopCollectionTimer() {
  if (elapsedTimer !== null) {
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
}

function onPointer(e: PointerEvent) {
  if (collector.addPointerSample(e.clientX, e.clientY, performance.now(), e.pointerType))
    sampleCount.value = collector.sampleCount
}

// Only the timing delta contributes; e.key/e.code/modifiers are never read
// (spec §8.1).
function onKey(e: KeyboardEvent) {
  if (collector.addKeySample(performance.now(), e.repeat)) sampleCount.value = collector.sampleCount
}

// Participation gate only — not a security estimate (spec invariant 11).
const collectionReady = computed(
  () => elapsedMs.value >= COLLECTION_MIN_DURATION_MS && sampleCount.value >= COLLECTION_TARGET_SAMPLES,
)
const collectionPercent = computed(() => {
  const t = Math.min(1, elapsedMs.value / COLLECTION_MIN_DURATION_MS)
  const s = Math.min(1, sampleCount.value / COLLECTION_TARGET_SAMPLES)
  return Math.round(Math.min(t, s) * 100)
})

function showPhrase(phrase: string) {
  mnemonic.value = phrase
  words.value = phrase.split(/\s+/)
  positions.value = pickDistinctIndexes(3, words.value.length)
  answers.value = {}
  stage.value = 'phrase'
}

// Enhanced generation. `useTranscript: false` is the skip path — it still
// includes backend crypto/rand AND renderer getRandomValues; only the
// transcript contribution becomes the empty digest (spec §6.1).
async function generate(useTranscript: boolean) {
  if (generating.value) return
  generating.value = true
  error.value = ''
  stopCollectionTimer()
  const transcript = useTranscript ? collector.freeze() : new Uint8Array(0)
  try {
    const request = await createEntropyRequest(transcript)
    showPhrase(await wallet.generateMnemonicWithEntropy(request))
  } catch (e: any) {
    // Never silently downgrade (spec §8.4): show the failure; when Web Crypto
    // is the cause, additionally expose the explicit backend-only action.
    if (!webCryptoAvailable()) cryptoUnavailable.value = true
    error.value = e?.message ?? String(e)
  } finally {
    collector.reset()
    generating.value = false
  }
}

// Deliberate, user-chosen fallback only — nothing calls this automatically.
async function generateBackendOnly() {
  if (generating.value) return
  generating.value = true
  error.value = ''
  stopCollectionTimer()
  try {
    showPhrase(await wallet.generateMnemonic())
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    collector.reset()
    generating.value = false
  }
}

async function copySeed() {
  try {
    await ClipboardSetText(mnemonic.value)
    copied.value = true
    everCopied.value = true
    setTimeout(() => (copied.value = false), 1500)
    // Don't let the seed linger in the clipboard for other apps to read: clear it
    // after a short window, but only if it's still ours (never wipe something the
    // user has copied since).
    const seed = mnemonic.value
    setTimeout(async () => {
      try {
        if ((await ClipboardGetText()) === seed) await ClipboardSetText('')
      } catch {
        /* ignore */
      }
    }, SEED_CLIPBOARD_TTL_MS)
  } catch {
    /* clipboard unavailable */
  }
}

const verifyOk = computed(
  () => positions.value.length === 3 && positions.value.every((p) => (answers.value[p] ?? '').trim() === words.value[p]),
)
const canCreate = computed(() => name.value.trim() !== '' && password.value.length > 0 && password.value === confirm.value)

async function finish() {
  error.value = ''
  try {
    // `name` is now a display name; the backend assigns a uuid keystore filename.
    // Capture the returned meta and unlock by its real id.
    const meta = await wallet.importMnemonic(name.value.trim(), password.value, mnemonic.value)
    // A freshly created wallet starts with auto-receive OFF, so it never inherits
    // a previously-enabled global toggle and surprise-sweeps the new account.
    try {
      const s = await Cfg.GetSettings()
      if (s.autoReceive) {
        await Cfg.SetAutoReceive(false)
      }
    } catch {
      /* best-effort */
    }
    await wallet.unlock(meta.id, password.value)
    router.push('/dashboard')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    password.value = ''
    confirm.value = ''
  }
}

// Best-effort teardown on any route exit (spec §9.4): stop timers, wipe the
// sample buffer, and drop mnemonic material from component state.
onUnmounted(() => {
  stopCollectionTimer()
  collector.reset()
  mnemonic.value = ''
  words.value = []
  positions.value = []
  answers.value = {}
})
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="relative w-[32rem]">
      <button
        class="absolute right-4 top-4 text-muted-foreground transition-colors hover:text-foreground"
        aria-label="close"
        @click="router.push('/unlock')">
        <XIcon :size="20" />
      </button>
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Create wallet</h1>

        <template v-if="stage === 'choice'">
          <h2 class="text-lg text-foreground">Generate your recovery phrase</h2>
          <p class="text-sm text-muted-foreground">
            Syrius always uses operating-system cryptographic randomness. You can also
            contribute unpredictable pointer movement or key timing before the recovery
            phrase is created.
          </p>
          <Button class="w-full" :disabled="generating" @click="startCollection">Add interaction randomness</Button>
          <Button variant="outline" class="w-full" :disabled="generating" @click="generate(false)">Generate without interaction</Button>
          <Button v-if="cryptoUnavailable" variant="outline" class="w-full" :disabled="generating" @click="generateBackendOnly">
            Generate using backend randomness
          </Button>
        </template>

        <template v-else-if="stage === 'collect'">
          <h2 class="text-lg text-foreground">Add interaction randomness</h2>
          <p class="text-sm text-muted-foreground">
            Move your pointer, swipe, or focus this area and press arbitrary keys. Syrius
            records only movement, timing, and position—not the keys you press.
          </p>
          <p class="text-sm text-muted-foreground">
            Do not type a password or recovery phrase. Your final recovery phrase is the
            only backup you need; these interactions are not saved.
          </p>
          <div
            tabindex="0"
            aria-label="interaction collection area"
            class="grid h-40 place-items-center rounded-lg border border-dashed border-border bg-background text-sm text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            @pointermove="onPointer"
            @keydown="onKey">
            <span aria-hidden="true">Move your pointer here, or focus and press keys</span>
          </div>
          <div
            role="progressbar"
            aria-label="Interaction collected"
            :aria-valuemin="0"
            :aria-valuemax="100"
            :aria-valuenow="collectionPercent"
            class="h-2 overflow-hidden rounded bg-border">
            <div
              class="h-full bg-primary transition-[width] motion-reduce:transition-none"
              :style="{ width: collectionPercent + '%' }"></div>
          </div>
          <p class="text-sm text-muted-foreground" aria-live="polite">
            <template v-if="collectionReady">Enough interaction collected</template>
            <template v-else>
              Interaction collected: {{ collectionPercent }}% — {{ sampleCount }} samples,
              {{ Math.floor(elapsedMs / 1000) }}s
            </template>
          </p>
          <Button class="w-full" :disabled="!collectionReady || generating" @click="generate(true)">Generate recovery phrase</Button>
          <Button variant="outline" class="w-full" :disabled="generating" @click="generate(false)">Skip interaction</Button>
          <Button v-if="cryptoUnavailable" variant="outline" class="w-full" :disabled="generating" @click="generateBackendOnly">
            Generate using backend randomness
          </Button>
        </template>

        <template v-else-if="stage === 'phrase'">
          <div class="flex gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
            <TriangleAlertIcon class="mt-0.5 shrink-0" :size="20" />
            <div>
              <p class="font-semibold">Important: save your recovery phrase</p>
              <p class="mt-0.5">Write these {{ words.length }} words down in order and store them safely. This is the only way to recover your wallet — anyone with them controls your funds. They are shown only once.</p>
            </div>
          </div>
          <div class="grid grid-cols-3 gap-2">
            <div v-for="(wd, i) in words" :key="i" class="rounded border border-border bg-background px-3 py-2 font-mono text-sm text-foreground">
              <span class="text-muted-foreground">{{ i + 1 }}.</span> {{ wd }}
            </div>
          </div>
          <Button variant="outline" class="w-full" aria-label="copy recovery phrase" @click="copySeed">
            <span class="inline-flex items-center justify-center gap-2">
              <CheckIcon v-if="copied" :size="15" />
              <CopyIcon v-else :size="15" />
              {{ copied ? 'Copied' : 'Copy recovery phrase' }}
            </span>
          </Button>
          <p v-if="everCopied" class="text-xs text-muted-foreground">
            In your clipboard — it auto-clears in ~45s. Clear it sooner if you paste it elsewhere.
          </p>
          <Button class="w-full" @click="stage = 'verify'">I've backed it up</Button>
        </template>

        <template v-else-if="stage === 'verify'">
          <p class="text-sm text-muted-foreground">Confirm your backup — enter these words:</p>
          <label v-for="p in positions" :key="p" class="block text-sm text-muted-foreground">
            Word #{{ p + 1 }}
            <Input v-model="answers[p]" :aria-label="`word ${p + 1}`" class="mt-1 font-mono" />
          </label>
          <Button class="w-full" :disabled="!verifyOk" @click="stage = 'password'">Continue</Button>
        </template>

        <template v-else>
          <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
          <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
          <Button class="w-full" :disabled="!canCreate" @click="finish">Create wallet</Button>
        </template>

        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
```

Accessibility notes satisfied by this markup (spec §9.5): the collection target is a focusable (`tabindex="0"`) named region; key listeners live only on it (Vue template listeners are element-scoped and removed on unmount); Tab/Escape are not intercepted; progress exposes `aria-valuemin/max/now` plus equivalent text; ready state is announced via `aria-live="polite"`; errors use `role="alert"`; readiness is conveyed by text, not color; the only motion is the progress-bar width transition, disabled under `motion-reduce`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd frontend && pnpm test -- src/views/Create.test.ts && pnpm run typecheck
```

Expected: all PASS, typecheck clean.

- [ ] **Step 5: Run the whole frontend suite (regression)**

```bash
cd frontend && pnpm test
```

Expected: PASS — no other suite touches mount-time generation, but verify nothing regressed.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/Create.vue frontend/src/views/Create.test.ts
git commit -m "feat(frontend): explicit entropy-choice wallet-creation flow (no mount-time generation)"
```

---

### Task 7: Full gates, security-review checklist, acceptance doc stub

**Files:**
- Create: `docs/phase7f-acceptance.md`
- No code changes expected (fix regressions if gates fail).

**Interfaces:**
- Consumes: everything above.
- Produces: green gates; the manual-acceptance checklist for the user to run.

- [ ] **Step 1: Run all backend and frontend gates**

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./...
GOWORK=off GOTOOLCHAIN=auto go vet ./...
GOWORK=off GOTOOLCHAIN=auto go build ./...
bash scripts/govulncheck-gate.sh
gosec -conf .gosec.json ./...
cd frontend && pnpm test && pnpm run typecheck && pnpm run build
```

Expected: everything passes. If gosec flags the new code, evaluate the finding against the spec's construction before touching `.gosec.json` — the production path must keep `crypto/rand` + `crypto/hkdf`.

- [ ] **Step 2: Walk the spec §12 security-review checklist against the diff**

Run `git diff main...HEAD` and confirm each §12 item, in particular:
- `grep -rn "math/rand" app/ frontend/src/` → no hits in wallet creation.
- No Wails-bound method or package-level var lets a caller replace backend randomness.
- HKDF salt/info/order/output length match §6.2 exactly (the frozen-vector test enforces this).
- No log/`console.*`/`fmt.Print` statements touch entropy material: `grep -n "console\." frontend/src/lib/wallet-entropy.ts frontend/src/views/Create.vue` → no hits.
- `ImportMnemonic` and keystore files are untouched: `git diff main...HEAD --stat` shows only the spec §13 file list (plus this plan and the acceptance doc).

- [ ] **Step 3: Create the acceptance document stub**

Create `docs/phase7f-acceptance.md`:

```markdown
# Phase 7f acceptance — additive user entropy (wallet creation)

Spec: `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md`

## Automated gates (record command output dates/results when run)

- [ ] `GOWORK=off GOTOOLCHAIN=auto go test ./...` / `go vet ./...` / `go build ./...`
- [ ] `bash scripts/govulncheck-gate.sh` and `gosec -conf .gosec.json ./...`
- [ ] `pnpm test`, `pnpm run typecheck`, `pnpm run build` (frontend/)

## Manual acceptance (spec §11.4 — user-run, per release-target WebView: macOS, Windows, Linux)

- [ ] 1. Open Create Wallet: no phrase is shown or generated before an explicit action.
- [ ] 2. Complete pointer collection and create a wallet.
- [ ] 3. Complete keyboard-only collection and create a wallet.
- [ ] 4. Use the skip-interaction path and create a wallet.
- [ ] 5. All three paths produce 24 valid BIP-39 words.
- [ ] 6. Each created keystore unlocks and derives the displayed base address.
- [ ] 7. At least one enhanced wallet opens in original Syrius with the same address.
- [ ] 8. Navigate away mid-collection; on return no listeners or stale progress remain.
- [ ] 9. Simulate Web Crypto unavailability in a dev build; only the explicit
      `Generate using backend randomness` action generates.
- [ ] 10. Inspect logs and network activity: no contribution, digest, mnemonic,
      or final entropy appears anywhere.
```

- [ ] **Step 4: Commit**

```bash
git add docs/phase7f-acceptance.md
git commit -m "chore(7f): acceptance record stub for additive user entropy"
```

- [ ] **Step 5: Report status**

Summarize gate results to the user and hand off the §11.4 manual acceptance (cross-platform GUI runs and the original-Syrius compatibility check are user-run, matching how Phase 5 acceptance was handled). Per the review-before-merge memory: offer the merge decision — do not auto-merge this funds-adjacent branch.
