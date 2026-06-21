# Phase 2 Crypto-Path Review Checklist (Gate 2)

Scope: the send/receive crypto path in `app/` — keystore read, key derivation,
hash construction, signing, Proof-of-Work, confirm-what-you-sign, and the
mainnet gate. This is the Gate-2 review gate that must clear before
`AllowMainnetSend` may be enabled in a release.

Legend:

- **[TEST]** — exercised by an automated test.
- **[REVIEW]** — requires human review/sign-off before enabling `AllowMainnetSend`.

The integration coverage referenced below lives in
`internal/spike/phase2_send_integration_test.go` (build tag `integration`). Its
live run is **PENDING** a testnet node that exposes the `embedded` RPC namespace
(`zenon.PrepareBlock` calls `embedded.plasma.getRequiredPoWForAccountBlock`).
Until that node is available, the items marked **[TEST]** are *written and
compile* but have **not** been observed passing live — treat them as REVIEW-plus
until the live run is recorded.

---

## 1. Keystore read (go-zenon canonical reader)

- [TEST] Keystore is read and decrypted via go-zenon's `wallet.ReadKeyFile` /
  `KeyFile.Decrypt`, not a hand-rolled parser. Covered structurally by
  `WalletService.Unlock` and exercised in the integration test
  (`buildApp` → `Wallet.Unlock`). Offline: `app/wallet_service_test.go`.
- [REVIEW] Wrong-password handling returns a generic "incorrect password" and
  does not leak whether the file or the password was at fault beyond that.
  (`WalletService.Unlock`.)
- [REVIEW] The decrypted keystore / mnemonic never crosses the Wails binding
  boundary: only `WalletMeta`, `AccountInfo`, addresses, and previews are
  returned to the frontend. Grep the DTOs for any field that could carry a
  secret. (`app/dto.go`, `app/wallet_service.go`.)
- [REVIEW] `Lock()` zeroes the in-memory keystore (`KeyStore.Zero`) and drops the
  reference; confirm no other goroutine retains the mnemonic. The integration
  test always `Lock()`s in cleanup.

## 2. Key derivation cross-check (SDK ⇄ go-zenon BIP-44)

- [TEST] `WalletService.signingKeyPair()` derives the SDK keypair from the
  unlocked mnemonic (`sdkwallet.NewKeyStoreFromMnemonic` → `GetKeyPair(index)`)
  and asserts the SDK-derived address equals the go-zenon active address. Any
  derivation divergence aborts the send. Exercised on every send/receive in the
  integration test (PrepareSend, Receive, RequiresPoW all call it).
- [REVIEW] The derivation index used for signing is the *same* index whose
  address is shown to the user and used for balances (`active`). Confirm
  `SelectAccount`, `activeAddress`, and `signingKeyPair` all key off `w.active`.
- [REVIEW] Account-range bound (`accountRange = 10`) and index validation in
  `SelectAccount` prevent out-of-range derivation.

## 3. Hash construction + signing (delegated to `zenon.PrepareBlock`)

- [TEST] The app never hand-builds the block hash or signature. It builds a
  template via `LedgerApi.SendTemplate` / `ReceiveTemplate` and hands it to
  `zenon.PrepareBlock` (autofill → set address/pubkey → required-PoW → nonce →
  hash → sign), mirroring the official Dart/TS SDKs. Covered by the send and
  receive subtests.
- [REVIEW] No code path bypasses `PrepareBlock`/`Send` to publish a block that
  was constructed or mutated elsewhere. `ConfirmPublish` only ever publishes the
  block produced by `PrepareSend`. (`app/tx_service.go`.)
- [REVIEW] Receive uses `zenon.Send` (which includes PoW when the receiving
  account is unfused) — confirm receive on an unfused account is acceptable
  (it will compute PoW). (`TxService.Receive`.)

## 4. Proof-of-Work (canonical, via the SDK facade)

- [TEST] PoW is produced only by the SDK facade (`zenon.PrepareBlock`/`Send`),
  driven by `embedded.plasma.getRequiredPoWForAccountBlock`; the app never sets
  `Difficulty`/`Nonce` itself. The Gate-2 **PoWSend** subtest sends from an
  unfused account, asserts `RequiresPoW == true`, and asserts the prepared block
  has `Difficulty > 0` and confirms on-chain.
- [REVIEW] PoW progress is surfaced to the UI via `PowCallback`
  (`EventTxPowProgress`) without blocking or leaking key material.
- [REVIEW] **Live run PENDING:** the PoW path cannot be validated against a node
  lacking the `embedded` namespace. Record a real testnet PoW send (Difficulty>0,
  confirmed) before sign-off.

## 5. Confirm-what-you-sign

- [TEST] The confirmation preview (`SendPreview`) is rendered from the **built,
  signed block** (`built.ToAddress/TokenStandard/Amount/Difficulty/Hash`), not
  from the raw request — so the user confirms exactly what was signed.
  (`TxService.PrepareSend`.)
- [TEST] Before broadcasting, `ConfirmPublish` **re-asserts** the held block
  still matches the originating request (to/zts/amount) and refuses to publish
  otherwise; the Send subtest also asserts the published hash equals the
  previewed hash. (`TxService.ConfirmPublish`.)
- [REVIEW] The held `pending` block is single-shot: cleared on publish, cancel,
  or mismatch, and guarded by a mutex so a stale or concurrent block can't be
  published. Confirm there is no path that publishes without the match check.

## 6. Mainnet gating

- [TEST] `assertTestnet` (integration) refuses to run unless the node's
  `ChainIdentifier` equals the expected testnet id (`ZNN_EXPECT_CHAINID`,
  default 73404) — the test can never broadcast against mainnet.
- [REVIEW] `TxService.guard()` blocks sends when `currentChainID() ==
  mainnetChainID (1)` unless `Settings.AllowMainnetSend` is true. This is the
  production gate. Confirm:
  - the guard runs at the *start* of `PrepareSend` (it does), and there is no
    alternate send entry point that skips it;
  - `AllowMainnetSend` defaults to `false` (`defaultSettings`) and is only
    toggled through an explicit, reviewed UI action;
  - `mainnetChainID` is correct for the live network.
- [REVIEW] Decide and document whether receive should also be mainnet-gated
  (currently `Receive` is not gated by `guard()`).

---

## Gate-2 sign-off summary

| Area | Status | Blocker to `AllowMainnetSend` |
|------|--------|-------------------------------|
| Keystore read | Test + offline unit tests | Human review of secret-boundary (1) |
| Derivation cross-check | Test (every send) | Human review of index consistency (2) |
| Hash + sign delegation | Test | Human review: no bypass path (3) |
| PoW canonical | Test written; **live PENDING** | Record live PoW send (4) |
| Confirm-what-you-sign | Test (preview + re-assert) | Human review of single-shot pending (5) |
| Mainnet gate | Test guard (testnet only) | Human review of `guard()` coverage + default (6) |

**Do not enable `AllowMainnetSend` until:**

1. The integration test (`TestPhase2SendReceive`) has been run live against a
   testnet node with the `embedded` namespace enabled, and the Send, Receive,
   and PoWSend subtests are observed passing (PoWSend with `Difficulty > 0`).
2. All **[REVIEW]** items above are signed off by a reviewer.
