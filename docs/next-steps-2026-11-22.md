# Next steps after the two-mode work (written 2026-11-21)

What is left after PRs #26–#30 and release v0.3.19. Ordered by what to do
first. Each item says what is known, what is not, and where to start.

## 1. Look at the new states in the running app

Everything below is covered by tests but has never been seen running. Do this
before telling anyone the release is solid.

| State | How to see it | What to check |
|---|---|---|
| Incoming lookup failed (#27) | Connect to a remote node, then make `GetUnreceived` fail (block the node mid-session, or point Remote at a node that drops the call) | Receive shows "Couldn't check for incoming payments" with a last-checked time; sidebar and top bar show "?"; "Nothing is waiting" never appears |
| Collateral rows (#28, #30) | Open the Advanced overview on an account that owns a pillar or sentinel | ZNN shows 15,000 pillar collateral (5,000 for a sentinel), QSR shows 50,000 sentinel collateral; amounts have a space before their label; the estimate includes them |
| Registration race (#30) | Register a sentinel or deposit QSR on testnet and return to the dashboard straight away | The locked amount appears within a momentum or two and stays. This race was reasoned from protocol timing, not reproduced |
| Failed collateral read (#30) | Same as the first row, but with the dashboard open | Committed column shows "? couldn't check"; scope line says the estimate may be low |
| Mainnet opt-in banner (#29) | Connect to mainnet, turn "Enable mainnet transactions" off in Settings → Advanced network settings | Banner on every screen except Settings; "Review setting" opens that section; after confirming the warning, "Back to where you were" returns |
| Network row (#29) | Start any send or contract call | Review shows "Mainnet · chain 1 · Real funds" or "Testnet · chain 73404" |
| Narrow window | Drag the window small in both modes | Top bar wraps acceptably with the mode switch and network chip |

To build the app without running Node on the host:

```bash
cd frontend && nodebox npx pnpm@10.17.1 run build
```

```bash
GOWORK=off GOTOOLCHAIN=go1.26.11 ~/go/bin/wails build -s
```

`GOTOOLCHAIN=go1.26.11` matters: the Wails CLI was built with 1.26.11, and with
the host's Go 1.27 it fails with "package syscall without types".

## 2. Confirm PR #29 had a human review

#29 touched `TxModal.vue` and the preview types, which fall under
confirm-what-you-sign. It merged with green CI and a clean CodeRabbit pass. If
nobody read the Go diff (two `ChainID` fields, three assignments) and the
Network row by eye, do that now. It is about forty lines.

## 3. Known gaps left on purpose

Small, each its own PR.

- **Send form loses its values on the consent detour.** Going from the banner
  to Settings and back returns to the Send screen with an empty form. Fixing it
  means keeping the draft in a store instead of component state. Start in
  `frontend/src/components/SendForm.vue`.
- **`GetCollateral` is all-or-nothing.** One failed read of its four hides the
  other three, and the four are not read at one height. The per-momentum reload
  hides both in practice. A cleaner version returns per-field results. Start in
  `app/nom_service.go` (`GetCollateral`, `collateralFrom`).
- **Pillar ownership is read twice per momentum.** `GetCollateral` and the
  shell's `pillar.refreshMyPillar` both call the node's pillar-by-owner read,
  which lists every pillar. The collateral store could reuse the pillar and
  sentinel stores for ownership and only fetch the two deposits.
- **Fused QSR has no unknown state.** If the plasma read fails, fused QSR is
  simply absent from the committed column, the same mistake #27 and #30 fixed
  for incoming payments, staking and collateral. Start in
  `frontend/src/lib/useHoldings.ts` (`committedStatus`).
- **Upgraded installs are not told Everyday mode exists.** They stay in
  Advanced and only find it through the top-bar switch. Decide whether a
  one-time note is wanted.
- **"Bridge sessions" is still labelled WalletConnect**, and node management
  is still a section of Settings rather than its own page. Both differ from the
  prototype; neither is a defect.

## 4. Deferred: embedded-node sync diagnostics

The last item from the prototype. A Node page showing where sync stopped, peer
history, recent node events and what to try. It needs new output from
`NodeService` (peer counts over time, an event log). Recommendation unchanged:
wait for a real stall report so the page shows what people actually need. The
prototype's version is in `docs/prototypes/two-mode-wallet.html` (tick
"Embedded sync stalled").

## 5. Project docs that no longer match the repo

Found while working; not fixed, because they are judgement calls about status.

- **`CLAUDE.md` and `AGENTS.md` say Phase 7b–7f remain**, including release
  builds. `.github/workflows/release.yml` exists, there are releases up to
  v0.3.19, and `docs/phase7f-acceptance.md` exists. The status lines need
  reconciling with what has shipped. What is clearly still open: signing and
  notarization (the release workflow says "Phase 7c, layered on later").
- **`AGENTS.md` lists old versions** (Go 1.25.13, `znn-sdk-go` v0.3.1) where
  `CLAUDE.md` has Go 1.26 and v0.5.0.
- **The store list in `CLAUDE.md`** does not include the two new stores,
  `collateral` and `consent`, or the new views (`HomeEveryday`,
  `OverviewAdvanced`, `Earn`, `MoreTools`, `Activity`).

## 6. Release work still open

- **Signing and notarization (Phase 7c).** Releases are unsigned; macOS and
  Windows users get a warning and manual steps on every install.
- **Auto-update.** Not present in the release workflow.
- **Manual GUI write-flow acceptance on testnet** for the NoM features is
  still listed as user-run in `CLAUDE.md`.

## Local tooling notes

- `govulncheck` and `gosec` are not installed on this machine. CI runs both.
- The Bend law gate (`test/bend/run.sh`) was not run locally for any of these
  PRs. CI ran it and it passed each time.
- `wails dev` runs `pnpm` on the host. Use the two build commands above
  instead, or run Vite inside `nodebox`.
