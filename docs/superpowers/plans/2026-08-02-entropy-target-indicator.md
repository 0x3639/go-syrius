# Entropy Target + Progress Indicator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user pick an entropy target (128/256/512 estimated bits, default 256) for wallet-creation interaction collection, hard-gate `Generate recovery phrase` on reaching it, and show progress as a 32-block segmented bar with an `N / target estimated bits` label.

**Architecture:** Frontend-only. The `InteractionCollector` in `frontend/src/lib/wallet-entropy.ts` gains a deterministic `estimatedBits` getter (1 bit per accepted pointer/touch sample, 2 per accepted key sample); `Create.vue` gains a preset radio group, replaces its sample-count gate with a bits gate, and replaces the percent bar with the segmented indicator. No Go, DTO, binding, transcript-format, or HKDF change of any kind.

**Tech Stack:** Vue 3 + TypeScript, Vitest + @vue/test-utils, Tailwind CSS 4. Spec: `docs/superpowers/specs/2026-08-02-entropy-target-indicator-design.md`.

## Global Constraints

- Branch: all work happens on the existing `entropy-target-indicator` branch (already contains the spec commit).
- Frontend commands run from `frontend/` with pnpm: `pnpm test`, `pnpm run typecheck`, `pnpm run build`.
- Backend commands (verification only — nothing backend may change): `GOWORK=off GOTOOLCHAIN=auto go test ./...` from the repo root.
- **Forbidden diffs:** if any change touches `app/`, `frontend/wailsjs/`, or `frontend/src/stores/wallet.ts`, the implementation has drifted from the spec. (`frontend/wailsjs` has pre-existing *uncommitted* local modifications from before this branch — leave them uncommitted and untouched.)
- Locked values (spec §5–§6): `BITS_PER_POINTER_SAMPLE = 1`, `BITS_PER_KEY_SAMPLE = 2`, presets `[128, 256, 512]`, default `256`, 32 indicator blocks, 5,000 ms duration floor retained.
- Copy rules (spec §10): every displayed bit figure carries the word "estimated"; no quality scores; required copy line: "Bits shown are a conservative estimate of your added interaction. Your wallet always uses operating-system cryptographic randomness as its primary source."
- The estimate is UI-only: it must never be sent to the backend, logged, or added to `EntropyRequest`.
- The user signs commits with GPG; if a commit fails on gpg, stop and report rather than retrying with `--no-gpg-sign`.

---

### Task 1: Credit rule in `InteractionCollector`

**Files:**
- Modify: `frontend/src/lib/wallet-entropy.ts`
- Test: `frontend/src/lib/wallet-entropy.test.ts`

**Interfaces:**
- Consumes: existing `InteractionCollector` (`addPointerSample(x, y, nowMs, pointerType?)`, `addKeySample(nowMs, repeat?)`, `freeze()`, `reset()`, `sampleCount`, `isFull`).
- Produces (Task 2 relies on these exact names):
  - `export const BITS_PER_POINTER_SAMPLE = 1`
  - `export const BITS_PER_KEY_SAMPLE = 2`
  - `export const ENTROPY_TARGET_PRESETS = [128, 256, 512]`
  - `export const ENTROPY_TARGET_DEFAULT = 256`
  - `get estimatedBits(): number` on `InteractionCollector`
- Do **not** remove `COLLECTION_TARGET_SAMPLES` in this task — `Create.vue` still imports it until Task 2.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/lib/wallet-entropy.test.ts` (and add `BITS_PER_POINTER_SAMPLE`, `BITS_PER_KEY_SAMPLE`, `ENTROPY_TARGET_PRESETS`, `ENTROPY_TARGET_DEFAULT` to the existing import from `./wallet-entropy`):

```ts
describe('estimatedBits credit rule', () => {
  it('locks the credit constants and presets', () => {
    expect(BITS_PER_POINTER_SAMPLE).toBe(1)
    expect(BITS_PER_KEY_SAMPLE).toBe(2)
    expect(ENTROPY_TARGET_PRESETS).toEqual([128, 256, 512])
    expect(ENTROPY_TARGET_DEFAULT).toBe(256)
  })

  it('credits 1 bit per accepted pointer/touch sample and 2 per accepted key sample', () => {
    const c = new InteractionCollector()
    c.addPointerSample(1, 2, 0) // +1 (pointer)
    c.addPointerSample(3, 4, 20, 'touch') // +1 (touch)
    c.addKeySample(40) // +2 (key)
    expect(c.estimatedBits).toBe(4)
  })

  it('credits nothing for throttled, repeated, or post-cap samples', () => {
    const c = new InteractionCollector()
    c.addPointerSample(1, 1, 0) // accepted, +1
    c.addPointerSample(2, 2, 5) // throttled (<16 ms)
    c.addKeySample(6, true) // repeat ignored
    expect(c.estimatedBits).toBe(1)

    const full = new InteractionCollector()
    for (let i = 0; i < COLLECTION_MAX_SAMPLES; i++) full.addKeySample(i * 2)
    expect(full.estimatedBits).toBe(COLLECTION_MAX_SAMPLES * BITS_PER_KEY_SAMPLE)
    full.addKeySample(99_999) // over capacity, rejected
    expect(full.estimatedBits).toBe(COLLECTION_MAX_SAMPLES * BITS_PER_KEY_SAMPLE)
  })

  it('freeze leaves the estimate readable and stops crediting; reset zeroes it', () => {
    const c = new InteractionCollector()
    c.addKeySample(0)
    c.freeze()
    expect(c.estimatedBits).toBe(2)
    c.addKeySample(30) // frozen, rejected
    expect(c.estimatedBits).toBe(2)
    c.reset()
    expect(c.estimatedBits).toBe(0)
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run (in `frontend/`): `pnpm test -- run src/lib/wallet-entropy.test.ts`
Expected: FAIL — the new describe block errors on missing exports / missing `estimatedBits`.

- [ ] **Step 3: Implement the credit rule**

In `frontend/src/lib/wallet-entropy.ts`:

Add after the existing `COLLECTION_MAX_SAMPLES` constant:

```ts
// Entropy-target UI (spec 2026-08-02): fixed, deliberately conservative
// per-accepted-sample credit. A participation estimate for gating/display
// only — never a cryptographic input, never sent to the backend.
export const BITS_PER_POINTER_SAMPLE = 1
export const BITS_PER_KEY_SAMPLE = 2
export const ENTROPY_TARGET_PRESETS = [128, 256, 512]
export const ENTROPY_TARGET_DEFAULT = 256
```

In `InteractionCollector`, add two private fields next to `count`:

```ts
  private pointerLikeCount = 0
  private keyCount = 0
```

Add a getter next to `sampleCount`:

```ts
  get estimatedBits(): number {
    return this.pointerLikeCount * BITS_PER_POINTER_SAMPLE + this.keyCount * BITS_PER_KEY_SAMPLE
  }
```

In `addPointerSample`, after the `this.lastSampleMs = nowMs` line (i.e., only on the accepted path), add:

```ts
    this.pointerLikeCount++
```

In `addKeySample`, after its `this.lastSampleMs = nowMs` line, add:

```ts
    this.keyCount++
```

In `reset()`, add:

```ts
    this.pointerLikeCount = 0
    this.keyCount = 0
```

Do not touch `freeze()`, `append()`, throttling, capacity, or any encoding.

- [ ] **Step 4: Run tests to verify they pass**

Run (in `frontend/`): `pnpm test -- run src/lib/wallet-entropy.test.ts`
Expected: PASS (all existing + 4 new tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/wallet-entropy.ts frontend/src/lib/wallet-entropy.test.ts
git commit -m "feat(entropy): fixed conservative estimatedBits credit on InteractionCollector

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: Presets, bits gate, and segmented indicator in `Create.vue`

**Files:**
- Modify: `frontend/src/views/Create.vue`
- Modify: `frontend/src/lib/wallet-entropy.ts` (remove `COLLECTION_TARGET_SAMPLES` only)
- Test: `frontend/src/views/Create.test.ts`

**Interfaces:**
- Consumes (from Task 1): `estimatedBits` getter, `ENTROPY_TARGET_PRESETS`, `ENTROPY_TARGET_DEFAULT`, plus existing `COLLECTION_MIN_DURATION_MS`.
- Produces: final UI. Gate = `estimatedBits >= entropyTarget && elapsedMs >= COLLECTION_MIN_DURATION_MS`. `COLLECTION_TARGET_SAMPLES` ceases to exist anywhere.

- [ ] **Step 1: Update existing tests that assumed the 32-sample gate, and add the new tests**

In `frontend/src/views/Create.test.ts`, inside `describe('Create.vue — interaction collection', ...)`:

**(a)** Replace the test `'pointer collection reaches ready only after samples AND duration; then generates'` entirely with:

```ts
  it('pointer collection reaches ready only after bits AND duration; then generates', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    // 260 pointer moves at exactly the 16 ms throttle boundary -> 260 accepted
    // samples = 260 estimated bits >= 256 by t=4160ms (duration floor not met)
    for (let i = 1; i <= 260; i++) {
      setNow(i * 16)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    // bits met, duration not met -> still disabled
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
```

**(b)** Replace the body of `'keyboard-only collection reaches ready state'` with (128 presses × 2 bits = 256):

```ts
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 128; i++) {
      setNow(i * 30)
      await target.trigger('keydown', { repeat: false })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
```

**(c)** In `'a failed generation from the collect stage clears the stale ready state'`, keep the structure but make it run against the 128-bit preset so the loops stay short. Replace the whole test with:

```ts
  it('a failed generation from the collect stage clears the stale ready state', async () => {
    GenerateMnemonicWithEntropy.mockRejectedValueOnce(new Error('backend exploded'))
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    await w.findAll('input[type="radio"]')[0].setValue() // 128-bit target
    for (let i = 1; i <= 130; i++) {
      setNow(i * 20)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')

    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)

    expect(w.text()).toContain('backend exploded')
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    // The collector was frozen+wiped by the failed attempt: the UI must not
    // still claim readiness, or a second click would submit an empty transcript.
    expect(w.text()).not.toContain('Enough interaction collected')
    expect(w.text()).toContain('0 samples')
    expect(w.text()).toContain('0 / 128 estimated bits')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()

    // Re-collecting works, and the retry carries a real transcript digest.
    for (let i = 1; i <= 130; i++) {
      setNow(6000 + i * 20)
      await target.trigger('pointermove', { clientX: i * 7, clientY: i * 11, pointerType: 'mouse' })
    }
    setNow(13000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')
    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    expect(GenerateMnemonicWithEntropy).toHaveBeenCalledTimes(2)
    expect(GenerateMnemonicWithEntropy.mock.calls[1][0].interactionDigestBase64).not.toBe(EMPTY_SHA256_B64)
    expect(w.text()).toContain('alpha')
  })
```

**(d)** Leave `'duration alone is not enough — sample target also gates readiness'`, `'skip is available before readiness...'`, `'events outside the collection target are not collected'` (asserts `'0 samples'`, which the new status line retains), `'starting collection shows...'`, and both teardown tests unchanged — they must still pass.

**(e)** Add these new tests to the same describe block:

```ts
  it('defaults to a 256-bit target with 128/256/512 presets', async () => {
    const { w } = mountCollecting()
    await startCollection(w)
    const radios = w.findAll('input[type="radio"]')
    expect(radios).toHaveLength(3)
    expect(radios.map((r) => (r.element as HTMLInputElement).value)).toEqual(['128', '256', '512'])
    expect((radios[1].element as HTMLInputElement).checked).toBe(true)
    expect(w.text()).toContain('conservative estimate')
    expect(w.text()).toContain('operating-system cryptographic randomness')
  })

  it('the old 32-sample participation level no longer satisfies the gate', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 40; i++) {
      setNow(i * 20)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('40 / 256 estimated bits')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
  })

  it('changing the target mid-collection re-gates without resetting samples', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 140; i++) {
      setNow(i * 16)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    const radios = w.findAll('input[type="radio"]')
    // 140 bits < 256 -> not ready on the default target
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    await radios[0].setValue() // 128
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
    await radios[2].setValue() // 512 — re-gates, samples preserved
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('140 / 512 estimated bits')
    expect(w.text()).toContain('140 samples')
    await radios[0].setValue() // back to 128 — ready again without new samples
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
  })

  it('segmented bar fills blocks and exposes aria values against the selected target', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 136; i++) {
      setNow(i * 16)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    const bar = w.find('[role="progressbar"]')
    expect(bar.attributes('aria-valuemin')).toBe('0')
    expect(bar.attributes('aria-valuemax')).toBe('256')
    expect(bar.attributes('aria-valuenow')).toBe('136')
    // floor(136 * 32 / 256) = 17 filled blocks
    expect(bar.attributes('aria-valuetext')).toBe('17 of 32 blocks · 136 / 256 estimated bits')
    expect(w.findAll('[data-filled="true"]')).toHaveLength(17)
    expect(w.findAll('[data-filled="false"]')).toHaveLength(15)
    expect(w.text()).toContain('17 of 32 blocks · 136 / 256 estimated bits')
    // Over-target display clamps to the target
    await w.findAll('input[type="radio"]')[0].setValue() // 128-bit target, 136 bits collected
    expect(bar.attributes('aria-valuemax')).toBe('128')
    expect(bar.attributes('aria-valuenow')).toBe('128')
    expect(w.findAll('[data-filled="true"]')).toHaveLength(32)
  })
```

- [ ] **Step 2: Run the Create tests to verify the new/updated ones fail**

Run (in `frontend/`): `pnpm test -- run src/views/Create.test.ts`
Expected: FAIL — no radio inputs, no `estimated bits` text, ready state reached at 32 samples in the replaced tests.

- [ ] **Step 3: Implement in `Create.vue`**

**Imports** — replace the `COLLECTION_TARGET_SAMPLES` import with the new constants:

```ts
import {
  InteractionCollector,
  createEntropyRequest,
  pickDistinctIndexes,
  webCryptoAvailable,
  COLLECTION_MIN_DURATION_MS,
  ENTROPY_TARGET_PRESETS,
  ENTROPY_TARGET_DEFAULT,
  type EntropyRequest,
} from '../lib/wallet-entropy'
```

**State** — next to the existing `sampleCount`/`elapsedMs` refs add:

```ts
const INDICATOR_BLOCKS = 32
const estimatedBits = ref(0)
const entropyTarget = ref<number>(ENTROPY_TARGET_DEFAULT)
```

**Resets** — in `startCollection()` (after `sampleCount.value = 0`) and in `restartCollectionAfterFailure()` (after `sampleCount.value = 0`) add:

```ts
  estimatedBits.value = 0
```

(Do not reset `entropyTarget` — the user's choice survives a failed attempt.)

**Event handlers** — extend both accepted-sample branches:

```ts
function onPointer(e: PointerEvent) {
  if (collector.addPointerSample(e.clientX, e.clientY, performance.now(), e.pointerType)) {
    sampleCount.value = collector.sampleCount
    estimatedBits.value = collector.estimatedBits
  }
}

// Only the timing delta contributes; e.key/e.code/modifiers are never read
// (spec §8.1).
function onKey(e: KeyboardEvent) {
  if (collector.addKeySample(performance.now(), e.repeat)) {
    sampleCount.value = collector.sampleCount
    estimatedBits.value = collector.estimatedBits
  }
}
```

**Computed** — replace the `collectionReady`/`collectionPercent` block (including its "Participation gate" comment) with:

```ts
// Hard participation gate (spec 2026-08-02 §7): the conservatively credited
// bit estimate must reach the user-selected target, and the minimum-duration
// floor still applies. Still not a security estimate of the final seed.
const collectionReady = computed(
  () => elapsedMs.value >= COLLECTION_MIN_DURATION_MS && estimatedBits.value >= entropyTarget.value,
)
const shownBits = computed(() => Math.min(estimatedBits.value, entropyTarget.value))
const filledBlocks = computed(() =>
  Math.min(INDICATOR_BLOCKS, Math.floor((estimatedBits.value * INDICATOR_BLOCKS) / entropyTarget.value)),
)
const blocksLabel = computed(
  () => `${filledBlocks.value} of ${INDICATOR_BLOCKS} blocks · ${shownBits.value} / ${entropyTarget.value} estimated bits`,
)
```

**Template (collect stage)** — insert the preset selector and honesty copy between the second `<p class="text-sm text-muted-foreground">` (the "Do not type a password…" paragraph) and the collection-area `<div tabindex="0" …>`:

```html
          <fieldset :disabled="generating">
            <legend class="text-sm text-muted-foreground">Entropy target</legend>
            <div class="mt-1 flex gap-4">
              <label
                v-for="p in ENTROPY_TARGET_PRESETS"
                :key="p"
                class="flex items-center gap-1.5 text-sm text-foreground">
                <input v-model="entropyTarget" type="radio" name="entropy-target" :value="p" />
                {{ p }} bits
              </label>
            </div>
          </fieldset>
          <p class="text-sm text-muted-foreground">
            Bits shown are a conservative estimate of your added interaction. Your wallet
            always uses operating-system cryptographic randomness as its primary source.
          </p>
```

Replace the existing `role="progressbar"` div (the percent bar and its inner width div) with the segmented bar:

```html
          <div
            role="progressbar"
            aria-label="Estimated entropy collected"
            :aria-valuemin="0"
            :aria-valuemax="entropyTarget"
            :aria-valuenow="shownBits"
            :aria-valuetext="blocksLabel"
            class="grid grid-cols-[repeat(32,minmax(0,1fr))] gap-0.5">
            <div
              v-for="i in INDICATOR_BLOCKS"
              :key="i"
              :data-filled="i <= filledBlocks"
              class="h-2 rounded-sm"
              :class="i <= filledBlocks ? 'bg-primary' : 'border border-border'"></div>
          </div>
```

Replace the status `<p ... aria-live="polite">` content with:

```html
          <p class="text-sm text-muted-foreground" aria-live="polite">
            <template v-if="collectionReady">Enough interaction collected</template>
            <template v-else>
              {{ blocksLabel }} — {{ sampleCount }} samples, {{ Math.floor(elapsedMs / 1000) }}s
            </template>
          </p>
```

Everything else in the template (buttons, error display, other stages) is unchanged.

**Remove the dead constant** — in `frontend/src/lib/wallet-entropy.ts`, delete the line `export const COLLECTION_TARGET_SAMPLES = 32` (spec §7: the sample-count gate ceases to exist). Then confirm nothing references it:

Run: `rg -n "COLLECTION_TARGET_SAMPLES" frontend/`
Expected: no matches.

- [ ] **Step 4: Run the full frontend suite and typecheck**

Run (in `frontend/`): `pnpm test` then `pnpm run typecheck`
Expected: all test files PASS (including untouched teardown/choice/flow tests); vue-tsc clean. Note filled/unfilled blocks differ by shape (solid fill vs. outline), satisfying the not-color-alone rule — verify no test regressed.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/Create.vue frontend/src/views/Create.test.ts frontend/src/lib/wallet-entropy.ts
git commit -m "feat(create): user-selectable entropy target with segmented progress indicator

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Docs, spec addendum, and full verification gates

**Files:**
- Modify: `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md` (addendum link only)
- Modify: `plan.md` (one bullet)

**Interfaces:**
- Consumes: completed Tasks 1–2 on the branch.
- Produces: docs consistent with shipped behavior; branch green on every gate.

- [ ] **Step 1: Add the Phase 7f spec addendum**

In `docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md`, directly after the `**Related:** ...` header line, insert:

```markdown

> **Addendum (2026-08-02):** `2026-08-02-entropy-target-indicator-design.md`
> narrowly revises invariant §5.11 and the §9.3 no-bit-estimate rule of this
> spec: the collection UI may show a fixed, conservatively credited bit figure
> (always labeled "estimated") and hard-gates interactive generation on a
> user-selected 128/256/512-bit target. All other invariants here stand
> unchanged.
```

- [ ] **Step 2: Update plan.md**

In `plan.md`, after the line 220 bullet (`- [ ] Additive user-entropy wallet creation (7f hardening): ...`), insert this bullet:

```markdown
- [ ] Entropy-target indicator (7f follow-up, frontend-only): user-selectable 128/256/512-bit estimated-entropy target with fixed conservative per-sample credit (1 bit pointer/touch, 2 bits key), 32-block segmented progress bar, hard gate + 5 s floor; skip paths unchanged, no backend/HKDF change. Spec: `docs/superpowers/specs/2026-08-02-entropy-target-indicator-design.md`.
```

- [ ] **Step 3: Run every verification gate**

From `frontend/`:

```bash
pnpm test
pnpm run typecheck
pnpm run build
```

From the repo root:

```bash
GOWORK=off GOTOOLCHAIN=auto go test ./...
git diff --stat main -- app/ frontend/src/stores/wallet.ts
```

Expected: all pass; the `git diff --stat` prints nothing (no backend/store drift). (`frontend/wailsjs` is excluded from the check because of the pre-existing uncommitted local modifications — confirm instead that no *commit* on this branch touches it: `git log --stat main..HEAD -- frontend/wailsjs` prints nothing.)

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-07-31-additive-user-entropy-design.md plan.md
git commit -m "docs: link entropy-target addendum from Phase 7f spec and plan

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

## Self-review notes

- Spec coverage: §5 credit rule → Task 1; §6 presets, §7 gate, §8 collector API, §9 indicator/aria/copy → Task 2; §2 addendum + plan.md → Task 3; §13 verification commands → Task 3 Step 3. §11.1/§11.2 test lists are all present verbatim in Tasks 1–2.
- The aria-valuenow clamp (`shownBits`) keeps `valuenow ≤ valuemax` when collected bits exceed the selected target; the spec's "aria-valuenow = current estimated bits" is satisfied up to the target, and the over-target case is asserted in the segmented-bar test.
- `COLLECTION_TARGET_SAMPLES` removal is deliberately deferred to Task 2 so Task 1 leaves the tree green.
