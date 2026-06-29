# Wallet Reskin — Sidebar + Dashboard, `/zenon-design` Compliance

**Date:** 2026-06-29
**Branch:** `wallet-reskin`
**Status:** Design approved — ready for implementation plan

## Goal

Reskin the syrius wallet to the **sidebar + dashboard** layout shown in the
`zenon-design-system` wallet UI kit (`ui_kits/wallet/`), and bring the frontend
into compliance with the `/zenon-design` standards (now installed at
`.claude/skills/zenon-design/`, distilled from `nom-ui` — the library this app
already uses, so the standard *is* our source of truth).

This is an **information-architecture change**, not a pure restyle: today's
single `Home.vue` screen hosts 7–8 NoM features as a `Tabs` strip; the target
is a persistent left sidebar + a plasma-hero Dashboard, with each feature on its
own routed page.

Scope decision (approved): **everything in one branch** — chrome, Dashboard, all
NoM pages, Transfer/Receive pages, lock screen, picker-component fixes, price
feed, and the Lucide icon migration.

## Non-goals

- **No Bridge feature** — the mockup's "Bridge" nav item is omitted entirely
  until a real bridge exists (no dead-end nav).
- No new NoM functionality — existing panel components are reused unchanged;
  only their wrapper/route changes.
- No backend/SDK changes beyond the read-only price fetch (§7).
- The binding boundary and all security invariants (CLAUDE.md) are untouched.

## Current state (what we're changing)

- **Chrome:** `TopBar.vue` only — an `AccountSlotPicker` + a row of hand-rolled
  inline-SVG icon buttons (plasma, auto-receive, votes badge, lock, address-book,
  settings). No sidebar.
- **`Home.vue`:** a centered `w-[56rem]` column → balance/action card grid →
  `StatusStrip` → a `Tabs` block with `Tokens / Rewards / Plasma / Pillar /
  Staking / Sentinels / Accelerator / Governance` (Governance testnet-gated).
- **Send/Receive:** modal components (`SendModal`, `ReceiveModal`).
- **Routes:** flat — `unlock / create / import / home / settings / tokens /
  address-book`. `Home` holds all NoM features via tabs.
- **Icons:** ~30 hand-rolled inline `<svg>` across TopBar + panels; `nom-ui`
  itself already depends on `@lucide/vue@^1.20.0`.
- **Known violations** (from the design-standard review): `from-primary to-info`
  avatar gradients, `rounded-[7px]/[10px]` arbitrary radii, off-scale
  `text-[13/15px]`, `✓`/`✕` unicode glyphs as buttons, QR hex `#00d659`
  (off-brand; brand green is `#00d557`), and an unused Nunito font asset.

## Target architecture

### 1. App shell

New `components/AppShell.vue` = persistent layout for all authenticated routes:

```
┌────────────┬──────────────────────────────┐
│  Sidebar   │  TopBar (title · pill · icons)│
│  (232px)   ├──────────────────────────────┤
│            │  <main> scrollable            │
│            │    <router-view/>             │
└────────────┴──────────────────────────────┘
```

- **`components/Sidebar.vue`** (new): ZNN logo + `syrius` wordmark with a mono
  uppercase tracked `NETWORK OF MOMENTUM` ledger-label eyebrow. Grouped nav (§2)
  with active state = `--sidebar-accent` background, semibold label, green
  (`--primary`) Lucide icon. Bottom block: Settings, Address book, and a
  **node-sync status pill** (`shield` icon + "Node synced" + mono `#height`,
  green when synced, muted/`warning` while syncing). Pill reads from
  `useNodeStore`.
- **`components/TopBar.vue`** (rewritten): left = current page title (`<h1>`,
  19px semibold). Right = **account/address pill** (mono truncated `z1…`,
  opens the account picker) + Lucide icon buttons that absorb today's TopBar
  controls: **theme toggle**, **plasma**, **auto-receive** (pressed state),
  **accelerator-votes** badge (when `pillar.ownsPillar` + `needsVoteCount`),
  **lock**. All wired to the same stores/handlers as today.

Public routes (`/unlock`, `/create`, `/import`) render **without** the shell.

### 2. Navigation & routing

`router/index.ts` is restructured so authenticated screens are children under
the shell. The lock guard (`wallet.locked` → redirect `/unlock`;
`!locked && isPublic` → redirect to dashboard) is preserved, just re-pointed at
the new default route `/dashboard`.

| Nav label | Route | Body |
|---|---|---|
| Dashboard | `/dashboard` | §3 — plasma hero + balances + recent activity |
| Transfer | `/transfer` | `SendForm` internals as a routed page |
| Receive | `/receive` | `ReceiveModal` body as a routed page |
| Tokens | `/tokens` | existing `Tokens.vue` / `TokensPanel` |
| — Network of Momentum — | (section label) | |
| Plasma | `/network/plasma` | `PlasmaPanel` |
| Staking | `/network/staking` | `StakingPanel` |
| Pillars | `/network/pillars` | `PillarPanel` |
| Sentinels | `/network/sentinels` | `SentinelsPanel` |
| Accelerator | `/network/accelerator` | `AcceleratorPanel` (keeps `?sub=` deep-link) |
| Rewards | `/network/rewards` | `RewardsPanel` |
| Governance | `/network/governance` | `GovernancePanel` — **conditional**: shown only when `ui.showGovernance && node.chainId !== 1` |
| — bottom — | | |
| Settings | `/settings` | `Settings.vue` |
| Address book | `/address-book` | `AddressBook.vue` |

- The 7–8 `Tabs` in `Home.vue` are **decomposed into routed pages reusing the
  existing panel components unchanged**. `Home.vue` is removed (or becomes a thin
  redirect to `/dashboard`).
- `tx.reset()` (today fired on tab change) moves to a route-leave/enter guard on
  the Network pages so a half-built block can't leak across features.
- Accelerator deep-link (`?tab=Accelerator&sub=Vote`) becomes
  `/network/accelerator?sub=Vote`; the TopBar votes badge updates accordingly.

### 3. Dashboard (`views/Dashboard.vue`, new)

Top-to-bottom composition, on the standard token surfaces:

1. **Plasma hero card** — `bg-plasma`, `--radius-xl`. Ledger-label eyebrow
   "TOTAL PORTFOLIO VALUE", a large mono `tabular-nums` **USD** total (§7),
   and primary **Send** / **Receive** buttons that route to `/transfer` and
   `/receive`. Plasma is reserved to this card + primary buttons only.
2. **ZNN + QSR balance cards** — `--card` surface, mono `tabular-nums` amounts
   with dimmed insignificant trailing zeros (existing `formatAmount` behavior),
   token icon via `TokenIcon`/`ZnnLogo`/`QsrLogo`, and a muted `≈ $X` fiat line
   (§7). Money is **not** colored.
3. **Recent activity card** — ledger-label table headers
   (`TYPE / HASH / COUNTERPARTY / AMOUNT / STATUS / TIME`), rows via the existing
   `TxHistory` / `TxStatus` / `TxDirection`, truncated `start…end` hashes,
   "View all" → `/tokens` (full history). Reuses `TxHistory.vue`.

### 4. `/zenon-design` standards applied

- **Lucide migration:** add `@lucide/vue@^1.20.0` as a direct dependency
  (matches nom-ui's pin). Replace **all** inline `<svg>` across TopBar, Sidebar,
  and panels with Lucide components. The `✓`/`✕` glyphs in `WalletPicker`,
  `ContactPicker`, `PillarLaunch`, `SentinelLaunch` → `Check` / `X` Lucide icons.
- **Review fixes #1–#4** in `WalletPicker.vue` + `AccountSlotPicker.vue`:
  - Remove `bg-gradient-to-br from-primary to-info` avatars → solid
    `--sidebar-accent` / `--muted` surface (plasma stays reserved).
  - `rounded-[7px]/[10px]` → `rounded-lg` (6px) / `rounded-xl` (10px) tokens.
  - `text-[10/13/15px]` → `text-xs` / `text-sm` / `text-base` scale.
- **QR color fix:** `AddressDisplay.vue` `#00d659` → brand `#00d557`
  (read from token where the `<canvas>` API allows; literal otherwise, corrected
  to the brand value).
- **Ledger labels** (`.text-ledger`) on every table header and stat caption.
- **No raw hex** anywhere else; semantic tokens only.
- **Cleanup:** delete the unused `nunito-v16-latin-regular.woff2` + `OFL.txt`.

### 5. Lock / Unlock screen (`views/Unlock.vue`)

Restyle to the reference lock screen: centered ZNN logo, "Welcome back" /
"Unlock your Syrius wallet", single 40px password field, **full-width plasma
Unlock button**, and a soft radial plasma halo background
(`radial-gradient(circle at 50% 30%, rgba(0,213,87,.10), transparent 60%)`).
Create/Import screens inherit the same calm, shell-less framing.

### 6. Theme

Keep light + dark (tokens define both). TopBar theme toggle flips
`document.documentElement.classList.toggle('dark')`, persisted via the existing
`ui` store. **Dark is the default** (the wallet's native habitat). Every new
surface is verified in both themes.

### 7. Price feed (USD portfolio total)

The hero and balance cards show USD, which requires live prices.

- **New `stores/price.ts`** (Pinia): fetches `https://api.zenon.info/price`.
  Response shape (verified):
  ```json
  { "data": { "znn": { "usd": 0.118422, "timestamp": "…" },
              "qsr": { "usd": 0.02343554, "timestamp": "…" },
              "btc": {…}, "eth": {…} } }
  ```
  Store exposes `znnUsd`, `qsrUsd`, `updatedAt`, `available` (bool).
- **Polling & resilience:** the endpoint is **rate-limited (observed HTTP 429)**.
  Poll **once on load and every 60s**, with: a single in-flight guard, backoff
  on 429/error, and **graceful degradation** — if no price is available the
  hero shows the **ZNN balance as the headline amount** (mono) with QSR
  secondary, and the `≈ $` lines are **omitted** rather than showing `$0`.
- **Portfolio total** = `znnAmount * znnUsd + qsrAmount * qsrUsd`, computed in
  the store from BigInt balances at full precision, formatted via
  `lib/format.ts` (never nom-ui `Amount`). Fiat is display-only — never used for
  any signing/amount path.
- The fetch is frontend-side (read-only public endpoint); it crosses no security
  boundary and handles no key material.

### 8. Testing

- **Survives:** most panel component tests (internals reused unchanged).
- **Updated:** `router` tests (new route table + default `/dashboard`); any test
  asserting the old `TopBar`/`Home` `Tabs` structure (`Home.test.ts` →
  `Dashboard.test.ts`).
- **New:** `AppShell` / `Sidebar` (nav renders, active state, node-sync pill),
  `Dashboard` (hero + cards + recent activity compose; lock guard still
  redirects), `stores/price` (parse, total math, 429/empty → `available=false`
  and `≈$` omitted), `Transfer`/`Receive` page mounts.
- Gates: `pnpm run typecheck`, `pnpm test`, `pnpm run build`, plus
  `GOWORK=off GOTOOLCHAIN=auto go build ./...` (Wails bindings untouched, but
  build proves the embed still compiles).

## Risks

- **Routing churn** is the biggest risk — decomposing `Home.vue`'s tabs into
  routes touches the lock guard, deep-links, and `tx.reset()` timing. Mitigate
  by porting panel bodies verbatim and moving `tx.reset()` to a route guard.
- **Price endpoint** rate-limits/down → covered by graceful degradation (§7).
- **Large diff** (Lucide migration + new chrome + routing) → land in reviewable
  commits: (a) shell + routing, (b) Dashboard + price, (c) Lucide migration,
  (d) picker/QR/token fixes + cleanup, (e) lock screen.

## Acceptance

- App opens to a sidebar + plasma-hero Dashboard matching the kit; every NoM
  feature reachable as its own page; Transfer/Receive are pages.
- Portfolio total + `≈$` show real USD when the feed is up; degrade cleanly to
  ZNN-headline + no `≈$` when it's not.
- No `from-primary to-info` gradients, no arbitrary radii, no unicode-as-icon,
  no raw hex; all icons Lucide; both themes verified.
- `pnpm run typecheck` / `pnpm test` / `pnpm run build` / `go build ./...` green.
