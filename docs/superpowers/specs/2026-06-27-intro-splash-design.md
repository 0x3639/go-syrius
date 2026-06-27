# Intro Splash — first-launch Lottie animation

**Date:** 2026-06-27
**Branch:** `ui-ux-fixes`
**Status:** Approved (brainstorming) — ready for implementation plan

## Goal

Play the `zn-logo.json` Zenon logo animation as a full-screen splash the **first
time the app is ever opened** on a machine. Subsequent launches show no splash.
Move the animation asset out of the repository root in the process.

## Decisions (from brainstorming)

| Question | Decision |
|----------|----------|
| Render method | **Lottie via `lottie-web`** (vector, scriptable, clean completion event) |
| Playback | **Play once** on first launch; full ~6s, dismiss on completion |
| Frequency | **First ever launch only** — persisted "seen" flag |
| Flag storage | **`localStorage`** key `zn:introSeen` (frontend-only, no Go changes) |
| Skippable | **Yes** — click anywhere or press Esc skips early |
| Asset location | **`frontend/src/assets/zn-logo.json`**, self-contained; removed from repo root |

## Asset facts

- `zn-logo.json` is a Lottie animation: format `v5.1.13`, `fr:30`, `op:180`
  (~6 seconds), `1920×1080`, name `"FINAL ANIMATION"`.
- It is **not self-contained**: `assets[0]` references an external image
  `images/img_0.png` (`"u":"images/","p":"img_0.png"`).
- `zn-logo.png` at the repo root is that image (501×800 PNG, ~2KB).

### Asset transform

Before moving the file, inline the PNG so the Lottie carries its own image and
needs no external path resolution under Vite/Wails:

- Read `zn-logo.png`, base64-encode it.
- On `assets[0]`: set `p` to `data:image/png;base64,<...>` and `u` to `""`.
- Write the result to `frontend/src/assets/zn-logo.json`.
- Delete `zn-logo.json` and `zn-logo.png` from the repo root.
- Leave the `animation/` working folder untouched.

## Components

### `frontend/src/components/IntroSplash.vue` (new)

Single purpose: play the Lottie once, then tell its parent it's done.

- **Props:** none. **Emits:** `done` (fired exactly once).
- **Template:** a `position: fixed` full-screen overlay (z-index above the app),
  dark background consistent with the app's dark theme, containing a single
  Lottie render container that fills/centres the frame.
- **On mount:**
  - Dynamically `import('lottie-web')` and `import('../assets/zn-logo.json')` so
    the library + 130KB asset are code-split and never loaded on later launches.
  - `lottie.loadAnimation({ container, renderer: 'svg', loop: false,
    autoplay: true, animationData })`.
  - Register the lottie `complete` event → `finish()`.
  - Add a `keydown` (Esc) listener and a container click handler → `finish()`.
- **`finish()`** (idempotent via a guard flag): trigger a short CSS fade-out
  (~250ms), then `emit('done')`.
- **On unmount:** `anim.destroy()`, remove the keydown listener. (Esc anywhere,
  click anywhere on the overlay.)

### `frontend/src/App.vue` (modified)

- Add `import IntroSplash from './components/IntroSplash.vue'`.
- Add `const showIntro = ref(localStorage.getItem('zn:introSeen') !== '1')`.
- Add `function dismissIntro() { localStorage.setItem('zn:introSeen', '1'); showIntro.value = false }`.
- Template: render `<IntroSplash v-if="showIntro" @done="dismissIntro" />`
  alongside `<RouterView />` and `<Toaster />`. The splash overlays whatever the
  router renders underneath; existing `onMounted` node-connect / theme logic is
  unchanged.

## Data flow

```
app start
  └─ App.vue: showIntro = (localStorage['zn:introSeen'] !== '1')
       ├─ true  → render IntroSplash
       │            ├─ lottie plays once (or user clicks / Esc)
       │            └─ 'complete' | skip → fade → emit 'done'
       │                 └─ App.dismissIntro(): set localStorage flag, hide splash
       └─ false → no splash; app as today
```

## Dependency

Add `lottie-web` to `frontend/package.json` (`pnpm add lottie-web`). It is
loaded only via dynamic import, so non-first launches never fetch it.

## Testing

- **`frontend/src/components/IntroSplash.test.ts` (new):** mock `lottie-web`
  (capture the registered `complete` handler; stub `loadAnimation`/`destroy`).
  - Emits `done` when the captured `complete` handler runs.
  - Emits `done` on skip (click on overlay, and Esc keydown).
  - Emits `done` only once even if completion + skip both fire.
- **`frontend/src/App.test.ts` (extend):** mock `IntroSplash` as a stub.
  - Splash rendered when `zn:introSeen` is absent.
  - Splash not rendered when `zn:introSeen === '1'`.
  - `@done` handler sets `localStorage['zn:introSeen'] = '1'` and removes the
    splash.

## Out of scope

- No Go/Wails backend changes (no `ConfigService` flag).
- No replay-from-settings control, no per-wallet reset.
- `animation/` working folder (conversion scripts, webm/webp/gif) is untouched.
- Pre-rendered video fallback is not used.

## Acceptance

1. Fresh app data (no `zn:introSeen`): launching shows the full-screen logo
   animation once; on completion (or click/Esc) it fades into the wallet UI and
   `localStorage['zn:introSeen']` becomes `'1'`.
2. Relaunch: no splash; app opens straight to its route.
3. Repo root no longer contains `zn-logo.json` / `zn-logo.png`; the asset lives
   at `frontend/src/assets/zn-logo.json` and renders without an external image.
4. `pnpm run typecheck`, `pnpm test`, and `pnpm run build` pass.
