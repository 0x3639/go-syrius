# nom-ui design system + home dashboard — design

**Date:** 2026-06-24
**Branch:** `ui-home-redesign`
**Scope:** First UI sub-project of the design pass — port the [nom-ui](https://github.com/digitalSloth/nom-ui) (MIT) look-and-feel and restructure the unlocked app into a single, easy-to-use **home dashboard**. A follow-up sub-project restyles the entry/secondary screens (Unlock/Create/Import, Settings) in the same language.

## Context

The wallet is feature-complete (Phases 1–5) but its UI is utilitarian: the unlocked home (`Dashboard.svelte`) is a status bar plus a row of text-link nav buttons that each switch to a separate full-screen route. The user wants the wallet to be **very easy to use and understand**, with the look-and-feel of nom-ui and a **simple home page that shows token balances and lets users send/receive** — confirmed via four reference screenshots (4-card top row, status strip, tabbed panel).

nom-ui is a Vue 3 + Tailwind 4 + shadcn-vue component library, so it cannot be imported into our Svelte + Tailwind 3 stack. This is a **faithful re-implementation of its design tokens and aesthetic in Svelte**, not a dependency. nom-ui is MIT-licensed; we credit it.

**This is presentation-only.** All Go bindings, Svelte stores (`balances`, `plasma`, `stake`, `pillar`, `sentinel`, `token`, `accelerator`, `tx`, …), and the confirm-what-you-sign `TxModal`/`ConfirmPublish` path are unchanged. We re-wrap existing logic; the funds path is untouched.

### Decomposition: Plan A then Plan B (one branch, merged together)

This sub-project is split into two implementation plans executed back-to-back on the `ui-home-redesign` branch (the branch is not merged to `main` until both are done, so users never see an intermediate state):

- **Plan A — design system + home shell.** Theme tokens/fonts/radius; base `ui/` components (`Card`, `Button`, `Input`, `Field`, `Tabs`); `Home.svelte` (top bar, 4-card row, status strip, 7-tab bar + tab-switching state); `BalanceCard`/`ActionCard`/`StatusStrip`; `SendModal`/`ReceiveModal` (fully working); `App.svelte` routing → `Home`; the **Tokens** tab as a native restyled panel. The other six tabs render a lightweight `PanelPlaceholder` stub. The old `routes/*.svelte` + `StatusBar` are left untouched for Plan B to harvest. **Deliverable:** the new-look home builds, renders, and works for balances + Tokens + Send + Receive; tests green.
- **Plan B — tab panels.** Implement the six remaining panels (`RewardsPanel` is new; `Plasma`/`Pillar`/`Staking`/`Sentinels`/`Accelerator` adapted from the current route bodies), wire them into `Home` replacing the placeholders, delete the now-dead `routes/{Plasma,Stake,Pillars,Sentinels,Tokens,Accelerator,Send,Dashboard}.svelte` + `lib/components/StatusBar.svelte`, and migrate their tests into the panel tests. **Deliverable:** all 7 tabs are native restyled panels; no dead routes; tests green.

## Design system (the nom-ui port)

We are **dark-only** (no light theme). Translate nom-ui's `.dark` tokens into our Tailwind 3 setup (CSS variables in `app.css` + a `tailwind.config` color map). Exact values from nom-ui `src/style.css`:

- **Surfaces:** bg `hsl(0 0% 8%)`, card `hsl(0 0% 10%)`, popover/elevated `hsl(0 0% 14%)`, muted surface `hsl(0 0% 15%)`.
- **Text:** foreground `hsl(0 0% 98%)`, muted `hsl(0 0% 65%)`.
- **Primary (green):** `hsl(145 100% 42%)` (= `--zenon-green #00D557`); primary-foreground `hsl(0 0% 8%)` (dark text on green). Focus ring uses the same green. The active tab, primary buttons, and positive numerals are green.
- **QSR accent (blue):** `--zenon-blue #0061EB` (info `hsl(214 100% 62%)`). ZNN balance card = green-tinted; QSR card = blue-tinted.
- **Semantics:** success `hsl(145 63% 45%)`, warning `hsl(38 95% 55%)`, error/destructive `hsl(352 86% 58%)`, border/input `hsl(0 0% 20%)`.
- **Plasma gradient** (primary/active surfaces): `linear-gradient(180deg, hsl(120 86% 63%) 0%, hsl(145 100% 38%) 100%)`.
- **Radius:** `0.375rem` (6px — tighter corners).
- **Type:** self-host **Space Grotesk** (UI) and **JetBrains Mono** (numbers/amounts/addresses) via `@fontsource-variable/space-grotesk` + `@fontsource-variable/jetbrains-mono` (bundled by Vite — no CDN, it is an offline desktop app). Mono is used for every balance, amount, and address.

**Base UI components** (`frontend/src/lib/components/ui/`), small and reused everywhere:
- `Card.svelte` — elevated surface (card bg, border, radius, shadow).
- `Button.svelte` — variants: `primary` (green-filled, dark text), `outline`, `ghost`, `danger`.
- `Input.svelte` — themed text input (input bg/border, green focus ring).
- `Field.svelte` — label + slot (input) + optional hint/error line.
- `Tabs.svelte` — the tab row with green active state + underline; emits the selected tab.

## Home layout & IA (`Home.svelte` replaces the Dashboard view)

`App.svelte` collapses from a 9-route switch to: locked → Unlock/Create/Import; unlocked → `Home.svelte`. The per-feature route bodies (`Plasma`, `Stake`, `Pillars`, `Sentinels`, `Tokens`, `Accelerator`) are refactored into **tab panels** under `lib/components/panels/`, keeping their existing store/binding logic and only changing presentation.

Layout, top to bottom (mirrors the screenshots):
1. **Top bar** (compact): `AccountSwitcher` + auto-receive toggle + `Lock`. Kept minimal above the cards.
2. **Card row (4 cards):**
   - `BalanceCard` ZNN — green-tinted, mono balance.
   - `BalanceCard` QSR — blue-tinted, mono balance.
   - `ActionCard` Send (up-arrow icon) — opens `SendModal`.
   - `ActionCard` Receive (down-arrow icon) — opens `ReceiveModal`.
3. **Status strip** (`StatusStrip.svelte`, restyle of `StatusBar`): Account Height · Tokens (count) · Plasma (⚡ level) · Pillar (delegated name or None).
4. **Tab panel — 7 tabs:** Tokens · Rewards · Plasma · Pillar · Staking · Sentinels · Accelerator. The tab row may wrap/scroll on narrow widths. Each panel:
   - **Tokens** — search + token list (symbol, name, balance), from the balances/token stores.
   - **Rewards** — aggregated uncollected rewards (delegation, staking, sentinel) with Collect buttons; data from the existing reward reads (`GetPillarReward`, stake/sentinel reward + collect Prepare methods). This is the one new aggregation view.
   - **Plasma** — fuse/cancel, from `PlasmaPanel` (current `Plasma.svelte` logic).
   - **Pillar** — list + delegate/undelegate, from `Pillars.svelte` logic.
   - **Staking** — stake/cancel/collect, from `Stake.svelte` logic.
   - **Sentinels** — register/revoke/collect, from `Sentinels.svelte` logic.
   - **Accelerator** — browse/donate/vote/manage, from `Accelerator.svelte` logic.

**Modals over the home:**
- `SendModal.svelte` — recipient / token / amount → existing `TxModal` confirm-what-you-sign → `TxResult`. Reuses the current Send form logic.
- `ReceiveModal.svelte` — active address + QR + copy.

All writes continue through the existing `awaitConfirm` → `TxModal` → `ConfirmPublish` path. No new signing UI; the funds path is unchanged.

## Component structure

- `frontend/src/app.css` + `tailwind.config.*` — the theme tokens (CSS vars + Tailwind color map), fonts, radius. Attribution comment crediting nom-ui (MIT).
- `frontend/src/lib/components/ui/{Card,Button,Input,Field,Tabs}.svelte` — base primitives.
- `frontend/src/routes/Home.svelte` — the home page (top bar, card row, status strip, tab panel host).
- `frontend/src/lib/components/{BalanceCard,ActionCard,StatusStrip}.svelte` — home pieces (StatusStrip supersedes `StatusBar`).
- `frontend/src/lib/components/panels/{TokensPanel,RewardsPanel,PlasmaPanel,PillarPanel,StakingPanel,SentinelsPanel,AcceleratorPanel}.svelte` — tab bodies (adapted from the current route files).
- `frontend/src/lib/components/{SendModal,ReceiveModal}.svelte` — the action modals.
- `frontend/src/App.svelte` — simplified routing (locked screens + `Home`).
- The old `routes/{Plasma,Stake,Pillars,Sentinels,Tokens,Accelerator,Send,Dashboard}.svelte` and `lib/components/StatusBar.svelte` are removed once their logic moves into panels/Home (their `*.test.ts` migrate alongside).

## Testing

- Vitest component tests for the `ui/` primitives (Button variants, Tabs selection, Field) and `Home` (renders cards/status/tabs; switching tabs swaps panels; Send/Receive cards open their modals; balances render in mono). Migrate the existing route tests into their panel equivalents.
- `svelte-check` clean; full vitest suite green.
- Visual verification via `GOWORK=off wails dev` against the testnet node — confirm the home matches the reference screenshots and the send/receive/feature flows still work.

## Out of scope (follow-up sub-project)

- Restyle Unlock / Create / Import and Settings into the same language.
- Polish: animations/transitions, refined empty states, responsive niceties, light theme (explicitly not wanted now).

## Attribution

Credit nom-ui (https://github.com/digitalSloth/nom-ui, MIT) in the theme file and a top-level `NOTICE` for the ported design tokens/aesthetic.
