# Entropy Target and Progress Indicator for Interaction Collection

**Date:** 2026-08-02
**Status:** Proposed
**Scope:** Frontend-only UX revision of the Phase 7f additive-entropy collection screen
**Audience:** Implementer and security reviewer
**Related:** `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md` (Phase 7f), `docs/phase7f-acceptance.md`

## 1. Summary

Let the user choose an entropy target (128, 256, or 512 estimated bits; default
256) for the optional interaction-collection step of wallet creation, and show
progress toward that target as a 32-block segmented bar with a live
`N / target estimated bits` label. `Generate recovery phrase` stays disabled
until the target is reached (plus the existing 5-second participation floor).
`Skip interaction` and `Generate without interaction` remain available exactly
as shipped in Phase 7f.

Bits are credited by a fixed, deterministic, deliberately conservative rule —
1 bit per accepted pointer/touch sample, 2 bits per accepted key sample — and
are always labeled *estimated*. This feature changes only frontend gating and
display. The transcript format, SHA-256 digest, renderer contribution, DTO,
Wails contract, and the backend HKDF construction are untouched.

## 2. Relationship to the Phase 7f spec

The Phase 7f spec locked two rules this feature revises:

- Invariant §5.11: "Interaction progress is a UX completion measure, not a
  security estimate."
- §9.3: "Never show an entropy-bit estimate, randomness quality score,
  'military grade,' or similar security theater."

Those rules were written to prevent *pseudo-scientific* claims — adaptive
meters, statistical estimators, and quality scores. This design revises them
narrowly rather than deleting them:

> The UI may display an entropy-bit figure only if it is (a) derived from the
> fixed per-sample credit rule in this document, (b) always labeled
> "estimated," and (c) accompanied by copy stating that operating-system
> cryptographic randomness remains the primary security source. Statistical
> or adaptive randomness estimators, quality scores, and unlabeled bit claims
> remain forbidden.

All other Phase 7f invariants — additive-only mixing, no silent downgrade,
no key-identity capture, scoped listeners, buffer wiping, no logging of
contributions — remain in force unchanged.

## 3. Goals

- Let the user dictate a target amount of estimated interaction entropy before
  generation is allowed on the interactive path.
- Show clearly how much more interaction is needed to reach the target.
- Keep the credit rule fixed, deterministic, and testable — a defensible lower
  bound, not a measurement.
- Change nothing in Go, the DTO, the bindings, the transcript encoding, or the
  HKDF construction.
- Preserve the always-available skip path and keyboard-only completion.

## 4. Non-goals

- Measuring actual min-entropy of user input (no statistical estimators).
- Backend validation of the target or the achieved bit count. The backend
  already treats every frontend field as untrusted; a backend check of a
  frontend-computed number adds no security.
- Changing the skip / backend-only paths, the mnemonic display, verification,
  or any later wizard step.
- Persisting the chosen target across sessions.

## 5. Credit rule (locked)

Credits accrue only for samples the collector *accepts* (post-throttle,
post-dedup, within the transcript cap):

```text
BITS_PER_POINTER_SAMPLE = 1   // kinds 1 (pointer) and 2 (touch)
BITS_PER_KEY_SAMPLE     = 2   // kind 3
```

Rationale: pointer samples are throttled to one per 16 ms and carry position
plus timing; 1 bit is a deliberate floor. Inter-keystroke timing is commonly
estimated well above 2 bits of min-entropy per keystroke, and the higher key
credit keeps the keyboard-only path humane (a 256-bit target needs 128
keypresses rather than 256).

Consequences at the transcript cap (2,048 samples): the maximum creditable
total is 2,048–4,096 bits depending on mix — every preset is reachable well
inside existing limits. No Phase 7f constant changes.

The estimate is a participation gate. It is never sent to the backend, never
logged, and never used in the cryptographic construction.

## 6. Targets and presets

```text
ENTROPY_TARGET_PRESETS = [128, 256, 512]
ENTROPY_TARGET_DEFAULT = 256
```

- Rendered as a radio group on the collection screen, visible before and
  during collection.
- 256 is the default because it matches the 24-word BIP-39 entropy width.
- Changing the target mid-collection re-evaluates the gate against already
  collected samples; it never resets or discards them.
- The choice is component state only; it is not persisted.

## 7. Gating

`Generate recovery phrase` enables when **both** hold:

1. `estimatedBits >= selected target`
2. `elapsed >= COLLECTION_MIN_DURATION_MS` (existing 5,000 ms floor, kept as a
   cheap guard against degenerate event bursts)

This replaces the Phase 7f `COLLECTION_TARGET_SAMPLES` (32-sample) gate on the
interactive path. `COLLECTION_TARGET_SAMPLES` is removed along with its uses;
sample counts may still be displayed but no longer gate anything.

Unchanged:

- `Skip interaction` stays visible and enabled at all times during collection.
- The entropy-choice screen's `Generate without interaction` path is untouched.
- Web Crypto failure handling and the explicit backend-only fallback are
  untouched.

## 8. Collector changes (`frontend/src/lib/wallet-entropy.ts`)

`InteractionCollector` gains per-kind accepted-sample counts and:

```ts
get estimatedBits(): number
```

computed as `pointerAndTouchCount * 1 + keyCount * 2`. `reset()` zeroes the
counts; `freeze()` does not alter them (the UI may keep showing the final
figure while generation runs). No change to record encoding, throttling,
capacity, `freeze()`, digesting, or `createEntropyRequest`.

Export the new constants (`BITS_PER_POINTER_SAMPLE`,
`BITS_PER_KEY_SAMPLE`, `ENTROPY_TARGET_PRESETS`, `ENTROPY_TARGET_DEFAULT`) so
tests and the view share one source of truth.

## 9. Indicator UI (`frontend/src/views/Create.vue`)

A segmented block bar with a fixed **32 blocks** for every preset, so the
visual language is constant and only the per-block denomination changes
(128 → 4 bits/block, 256 → 8, 512 → 16). Block *i* (0-based) renders filled
when `estimatedBits >= (i + 1) * target / 32`.

Below the bar, one text line:

```text
17 of 32 blocks · 136 / 256 estimated bits
```

On reaching the target, the existing ready state text updates to
`Enough interaction collected` (unchanged wording) and the live region
announces it as today.

Copy addition near the target selector:

> Bits shown are a conservative estimate of your added interaction. Your
> wallet always uses operating-system cryptographic randomness as its primary
> source.

### Accessibility

- The bar keeps `role="progressbar"` with `aria-valuemin="0"`,
  `aria-valuemax` = selected target, `aria-valuenow` = current estimated bits,
  and an `aria-valuetext` equivalent of the text line.
- Filled vs. unfilled blocks differ by more than color (e.g., filled vs.
  outlined), consistent with existing status-not-by-color-alone handling.
- The radio group is keyboard-operable and labeled; changing it does not steal
  focus from the collection target.
- No decorative motion is added; block fill is a discrete state change, safe
  under `prefers-reduced-motion`.

## 10. Honesty constraints

- Every displayed bit figure carries the word "estimated."
- No adaptive meters, randomness quality scores, or statistical estimators.
- No copy may present interaction as the primary security source or imply the
  wallet is weak without it.
- The estimate never appears in logs, errors, telemetry, or the entropy
  request.

## 11. Testing requirements

Frontend only; the Go suite is untouched and must remain green.

### 11.1 `wallet-entropy.test.ts`

- Pointer, touch, and key samples credit 1, 1, and 2 bits respectively.
- Throttled/rejected pointer samples, repeated keydowns, and post-cap samples
  credit nothing.
- `reset()` zeroes `estimatedBits`; `freeze()` leaves it readable.
- Credit constants and presets are exported with the locked values.

### 11.2 `Create.test.ts`

- Default target is 256; presets 128/512 selectable.
- Generate stays disabled below the target even when the old 32-sample level
  is passed, and enables exactly at `bits >= target && elapsed >= 5 s`.
- Changing the target mid-collection re-gates without resetting samples
  (raise → disables again if below new target; lower → enables if satisfied).
- Keyboard-only collection reaches a 256-bit target at 128 accepted
  keypresses.
- Segmented bar fills the correct block count for given bits/target, and
  `aria-valuemax`/`aria-valuenow` track target/bits.
- `Skip interaction` remains enabled before readiness.
- Existing Phase 7f tests (no mount-time generation, no silent downgrade,
  cleanup on route leave, backup verification, etc.) remain green, updated
  only where they referenced the removed 32-sample gate.

## 12. Expected files

```text
frontend/src/lib/wallet-entropy.ts
frontend/src/lib/wallet-entropy.test.ts
frontend/src/views/Create.vue
frontend/src/views/Create.test.ts
docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md   (short addendum link only)
plan.md                                                             (Phase 7f note)
```

No Go, DTO, binding, store, or SDK files change. If any diff touches
`app/`, `frontend/wailsjs/`, or `frontend/src/stores/wallet.ts`, the
implementation has drifted from this design.

## 13. Verification commands

From `frontend/`:

```bash
pnpm test
pnpm run typecheck
pnpm run build
```

From the repository root (confirm nothing backend-visible changed):

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./...
git diff --stat main -- app/ frontend/wailsjs/   # must be empty
```

## 14. Acceptance criteria

- User can select 128/256/512 estimated bits; default 256.
- Segmented 32-block bar and `N / target estimated bits` label track accepted
  samples live under the locked credit rule.
- Generate is hard-gated on target + 5 s floor; skip and
  generate-without-interaction remain available and unchanged.
- Keyboard-only, pointer-only, and mixed collection can all reach every
  preset.
- All displayed figures are labeled "estimated"; no estimator/quality-score
  code exists.
- No backend, DTO, binding, transcript, or HKDF change; Go tests and
  compatibility gates pass unmodified.
- Frontend test suite covers §11 and passes; typecheck and build pass.
