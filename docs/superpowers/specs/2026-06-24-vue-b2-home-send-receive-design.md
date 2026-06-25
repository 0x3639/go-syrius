# Vue migration — Sub-project B, Phase B2: Home + Send/Receive — design

**Date:** 2026-06-24
**Branch:** `frontend-vue-migration` (continues after B1, `bc734db`)
**Parent:** `docs/superpowers/specs/2026-06-24-frontend-vue-migration-design.md` (overall Svelte→Vue migration). Sub-project B is decomposed B1–B4; **this spec covers B2**. B3 (NoM panels) and B4 (Settings + parity/merge) get their own specs.

## Context

B1 delivered the vue-router foundation + wallet lifecycle. **B2 is the funds-critical phase:** the real Home (replacing the placeholder `home` route) and the **Send** and **Receive** flows. This is where the confirm-what-you-sign invariant, the auto-receive behavior, and balance/transaction display all live. The merged Svelte app on `main` (`routes/Home.svelte`, `lib/components/{SendForm,SendModal,TxModal,TxResult,ReceiveModal,UnreceivedPanel,StatusStrip,TxHistory,BalanceCard,ActionCard,AccountSwitcher}.svelte`, `lib/stores/{balances,tx,txs,unreceived,token,plasma,node}.ts`) is the **UX + behavior reference**.

The Go backend + Wails bindings are unchanged. B2 ports the funds **logic verbatim** but adopts **nom-ui's presentational components** (the purpose of the Vue migration).

## Design stance (approved)

**Faithful behavior, nom-ui presentation:**
- **Adopt nom-ui components** where they map: **Dialog** (Send/Receive/confirm modals), **Tabs** (the tab container), **Address** (account bar + tx history, with truncation/copy), **TxStatus**/**TxDirection** (tx-history rows), **TokenIcon** (token rows), **CopyButton** (receive address), **Table** (tx history), and **sonner `useToast`** (transaction published/received feedback).
- **Keep `src/lib/format.ts`** (`formatAmount`/`formatAmountExact`) for ALL amounts — nom-ui's `Amount` rounds through `Number()` and loses precision on large balances. The confirm modal uses `formatAmountExact`.
- **Port funds logic verbatim:** the `tx` store contract, the confirm-what-you-sign rendering (TxModal renders the *built block*, never raw form inputs), and the auto-receive sweep + account-switch + "Receiving…" UX all carry over unchanged from the merged Svelte app.

Consequence (accepted): the Home will not be a pixel-clone of the Svelte version — same layout + behavior, nom-ui's styling for dialogs/tabs/tx-rows.

## Scope

**In:** the real `Home.vue` (account bar, 4-card row, status strip, nom-ui Tabs container, tx history), the **Tokens** panel, **Send** (SendForm/SendModal/TxModal/TxResult), **Receive** (ReceiveModal/UnreceivedPanel) + auto-receive, and the Pinia stores: `node` (events), `balances`, `tx`, `txs`, `unreceived`, `token`, `plasma` (+ a minimal pillar-delegation read for the status strip).

**Out (→ later phases):** the other 6 tab panels — Rewards, Plasma, Pillar, Staking, Sentinels, Accelerator — are **placeholder panels** in B2's tab container, filled in **B3**. Settings, account rename/reveal-mnemonic UI, Tokens-management route, and the branch merge are **B4**.

## Funds-safety invariants (non-negotiable, carried from the Svelte app)

- The frontend sends **intent** only (recipient/token/amount); Go builds → PoWs → signs → publishes. No key material reaches the frontend.
- **Confirm-what-you-sign:** `TxModal` renders the effect from the **built block** returned by `PrepareSend` (the `SendPreview`: address/amount/zts/fee/hash), not from the form inputs. The user confirms that preview; `confirm()` calls `ConfirmPublish()` on the Go-held pending block. `cancel()` calls `CancelPending()`.
- Amounts in the confirm modal use **`formatAmountExact`** (exact value being signed).
- Every state-changing call re-validates server-side (Go bindings).

## Architecture

### Stores (`src/stores/`, Pinia)

- **`node`** (extend B1/A's): an `initEvents()` action registers Wails `runtime.EventsOn` listeners for `node:status`, `momentum-tick`, `node:sync`, `tx:published`, `tx:received` (constants from the Go `Event*` names). Status/height/syncing become reactive state; `tx:published`/`tx:received` fire a toast and trigger a refresh of balances/txs/unreceived. Mirrors the Svelte `initNodeEvents(refresh)`.
- **`balances`** — `load()` → `NodeService.GetBalances()` → `TokenBalance[]`.
- **`tx`** — `prepare(toAddress, zts, amount)` → `TxService.PrepareSend({toAddress,zts,amount})` → `SendPreview` (status `awaiting`); `awaitConfirm(preview)`; `confirm()` → `TxService.ConfirmPublish()` (status `done`, hash); `cancel()` → `TxService.CancelPending()`; `reset()`. State machine `idle|preparing|awaiting|publishing|done|error`. Reset on route change (the Svelte store reset on nav). **Identical contract to the Svelte `tx.ts`.**
- **`txs`** — `load(page,count)` → `NodeService.GetTransactions()` → `TxRecord[]`.
- **`unreceived`** — `load()` → `NodeService.GetUnreceived()`; `receive(hash)`/`receiveAll()` → `TxService.Receive(hash)` with the per-row "Receiving…" busy state + error surfacing (the UX we built in the Svelte app).
- **`token`** — token metadata/list for the Tokens panel (the binding the Svelte `token.ts` used).
- **`plasma`** — `plasmaInfo` for the status strip; plus a minimal pillar-`delegation` read (status strip shows "Pillar: <name>/None"). The full plasma/pillar panels + actions are B3.

### Components (`src/components/` + `src/views/Home.vue`)

- **`Home.vue`** — the `home` route. Composition (faithful to the Svelte Home):
  - **Account bar:** `AccountSwitcher` (account select + the rename affordance) · **Auto-receive** toggle · **Settings** link (`router.push('/settings')`) · **Lock** (`wallet.lock()` → guard redirects to `/unlock`).
  - **4-card row:** `BalanceCard` ZNN, `BalanceCard` QSR (our `formatAmount`), `ActionCard` Send (opens SendModal), `ActionCard` Receive (badge = `unreceived.length`, opens ReceiveModal).
  - **`StatusStrip`** — account height, tokens count, plasma level, pillar name. (B2 also registers a **placeholder `settings` route** + trivial view — same pattern B1 used for `home` — so the Settings link is not dead navigation; B4 replaces it with the real Settings.)
  - **nom-ui `Tabs`** — Tokens (filled) + Rewards/Plasma/Pillar/Staking/Sentinels/Accelerator (placeholder components in B2, real in B3).
  - **`TxHistory`** — nom-ui `Table` rows with `TxDirection` + `TxStatus` + `Address` + our `formatAmount`.
  - Auto-receive logic: `startAR`/`stopAR`/`onActiveChange` (stop-then-start, re-sweep on account switch, resume on load) — ported verbatim from the Svelte Home.
- **`SendModal.vue`** (nom-ui **Dialog**) → **`SendForm.vue`** (recipient `Input` + token `Select`/native over `balances` + `AmountInput`) → on submit `tx.prepare(...)`. When `tx.status==='awaiting'`, the Dialog shows **`TxModal.vue`** (the preview); `confirm()` publishes → **`TxResult.vue`** + a success toast. Errors surface in the modal.
- **`ReceiveModal.vue`** (nom-ui **Dialog**) — QR (the `qrcode` dep) + nom-ui `Address` + `CopyButton` for the active address + **`UnreceivedPanel.vue`** (the Receive/Receive-all list with "Receiving…" + error surfacing).
- **`TxModal.vue`** — confirm-what-you-sign: renders `preview` (to/amount via `formatAmountExact`/zts/fee/hash) with Confirm/Cancel.
- Small ported pieces: `BalanceCard.vue`, `ActionCard.vue` (badge), `AccountSwitcher.vue`, `AmountInput.vue`, `TokensPanel.vue`.

### Data flow

1. On Home mount: `node.initEvents()`, refresh balances/txs/unreceived/plasma, resume auto-receive if enabled.
2. Momentum tick / `tx:received` / `tx:published` events → refresh the affected stores + toast.
3. **Send:** form intent → `tx.prepare` → preview → user confirms the *built block* → `tx.confirm` → publish → toast + history refresh.
4. **Receive:** badge reflects `unreceived.length`; manual receive or the auto-receive sweep calls `TxService.Receive`; "Receiving…" while PoW runs.

## Error handling

- Send/receive errors surface in their modal (`role="alert"`); never swallowed.
- The "Receiving…" busy state + the PoW hint (receive needs PoW when plasma is low) carry over.
- `node` event listeners are registered once and are best-effort (a failed connect leaves the UI usable/disconnected).
- The `tx` store resets on route change so a stale result never appears on another screen.

## Testing

Vitest + @vue/test-utils, mocking `wailsjs/go/app/*` bindings + stubbing nom-ui components; a test router/pinia where needed:
- **`tx` store contract:** `prepare` sets `awaiting` with the preview; `confirm` calls `ConfirmPublish` and sets `done`+hash; `cancel` calls `CancelPending`; error path sets `error`.
- **Send flow:** SendForm submit → `tx.prepare` called with the entered intent; once `awaiting`, `TxModal` renders the **preview** amount (via `formatAmountExact`); confirm → `ConfirmPublish`.
- **Receive:** `UnreceivedPanel` receive calls `TxService.Receive(hash)` and shows "Receiving…"; the Receive `ActionCard` badge reflects `unreceived.length`.
- **Home:** renders ZNN/QSR via `formatAmount`; the Tabs container shows the Tokens panel; Lock calls `wallet.lock()`.
- **node events:** a mocked `tx:received` event triggers a balances/unreceived refresh.
- Gates: `pnpm test`, `pnpm run typecheck` (vue-tsc), `pnpm run build`; controller live `wails dev` gate — view balances, send a small testnet tx end-to-end (prepare→confirm→published), receive a pending block.

## Risks

- **nom-ui Dialog/Tabs/Address/TxStatus/TxDirection/TokenIcon/Table/sonner API** — verify each export + props against the installed `node_modules/nom-ui/src` (the A/B1 discipline); adapt or fall back and note it.
- **Wails events in Vue** — `runtime.EventsOn` is available via `wailsjs/runtime`; register in a store action after app mount; clean up is not critical (single long-lived app) but listeners should not double-register (guard with an `initialized` flag, as B1's router guard does for Pinia init).
- **confirm-what-you-sign fidelity** — the single most important correctness point; TxModal must read the preview from the built block, never the form. Reviewed explicitly in the final B2 review.
- **Toast/sonner setup** — sonner needs a `<Toaster/>` mounted once (in `App.vue`); verify the nom-ui export.
- **Scope creep** — the 6 NoM panels are placeholders only; do not implement panel logic in B2.
