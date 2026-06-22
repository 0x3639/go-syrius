# Phase 5e — Tokens / ZTS (Issue / Mint / Burn / Update) Design

**Date:** 2026-06-22
**Status:** Approved
**Scope:** Fifth sub-phase of Phase 5 (NoM features). ZTS token management: view the tokens the active address owns + look up any token by ZTS; issue a new token; mint; burn; and update a token (transfer ownership / permanently disable mint/burn). Reuses the shared embedded-contract-call path from 5a/5b/5c/5d (NomService + `TxService.prepareCall`).

## Goal

Let the user create and manage ZTS tokens from the wallet — issue a token (paying the 1 ZNN fee), mint to a receiver, burn tokens they hold, and transfer ownership / finalize a token — all through the existing confirm-what-you-sign / PoW / chain-guard pipeline, with the on-chain field rules validated in Go before any node use.

## Locked decisions (brainstorming 2026-06-22)

- **Actions:** Issue + Mint + Burn + Update (full set). Update is a single call exposing transfer-owner + one-way disable-mint + one-way disable-burn.
- **Read scope:** `GetByOwner` ("my tokens", drives mint/update) + `GetByZts` (look up any token, drives burn targeting). **No global all-tokens browser** (`GetAll`) — YAGNI for a wallet.
- **Amount convention:** supplies and mint/burn amounts are **base-unit decimal strings** (explicit, no hidden whole↔base conversion — consistent with Stake/Deposit), displayed via `formatAmount(_, token.decimals)`.
- **Issue fee:** the 1 ZNN fee is read from the SDK template (`TokenIssueAmount`), never hardcoded.
- **No SDK / no go-zenon forks:** templates via `client.TokenApi.*`; publish via the shared `prepareCall`.

## Context (verified against go-zenon @ v0.0.8-alphanet / SDK @ v0.1.17)

- `client.TokenApi.GetByOwner(address, pageIndex, pageSize uint32) (*TokenList{Count int; List []*Token}, error)` — tokens owned by the address.
- `client.TokenApi.GetByZts(zts types.ZenonTokenStandard) (*Token, error)` — info for one token.
- `Token{Name, Symbol, Domain string; TotalSupply *big.Int; Decimals uint8; Owner types.Address; TokenStandard types.ZenonTokenStandard; MaxSupply *big.Int; IsBurnable, IsMintable, IsUtility bool}`.
- `client.TokenApi.IssueToken(name, symbol, domain string, totalSupply, maxSupply *big.Int, decimals uint8, isMintable, isBurnable, isUtility bool) *nom.AccountBlock` — `{ToAddress: TokenContract, TokenStandard: ZnnTokenStandard, Amount: constants.TokenIssueAmount (1 ZNN = 100000000), Data: ABIToken.Pack(Issue, …)}`.
- `client.TokenApi.Mint(zts, amount *big.Int, receiver types.Address) *nom.AccountBlock` — `{TokenContract, ZnnTokenStandard, Amount: Big0, Data: Pack(Mint, zts, amount, receiver)}`.
- `client.TokenApi.Burn(zts, amount *big.Int) *nom.AccountBlock` — `{TokenContract, **TokenStandard: zts** (the token being burned), **Amount: amount** (the tokens are sent to the contract), Data: Pack(Burn)}`.
- `client.TokenApi.UpdateToken(zts, owner types.Address, isMintable, isBurnable bool) *nom.AccountBlock` — `{TokenContract, ZnnTokenStandard, Amount: Big0, Data: Pack(UpdateToken, zts, owner, isMintable, isBurnable)}`.
- Constants: `types.TokenContract`, `types.ZnnTokenStandard`; `TokenIssueAmount = 1 ZNN`; `TokenNameLengthMax = 40`; `TokenSymbolLengthMax = 10`; `TokenDomainLengthMax = 128`; `TokenMaxDecimals = 18`; `TokenMaxSupplyBig = 2^255 − 1`.
- On-chain Issue validation (mirror in Go): name length 1–40 and regex `^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`; symbol length 1–10 and regex `^[A-Z0-9]+$`; domain empty OR regex `^([A-Za-z0-9][A-Za-z0-9-]{0,61}[A-Za-z0-9]\.)+[A-Za-z]{2,}$` and length ≤ 128; decimals in [0, 18]; maxSupply ≤ 2^255−1; maxSupply > 0; maxSupply ≥ totalSupply; if **not** mintable, maxSupply must **equal** totalSupply.
- **Token-standard split is the correctness-critical detail (the 5a "Cancel=ZNN" lesson):** Issue/Mint/Update are ZNN; **Burn uses the token's own ZTS and carries the burn amount**. A regression test locks each against the real SDK builders.

5a–5d provide the shared path: `TxService.prepareCall(template, callExpect{to, zts, amount, data}, summary)` runs guard→`zenon.PrepareBlock`(PoW)→hold; `ConfirmPublish` re-asserts the built block's `ToAddress`/`TokenStandard`/`Amount`/`Data` against the held `callExpect` (confirm-what-you-sign), mainnet-gated. NomService holds no key material.

## Architecture

Identical shape to 5a–5d. NomService gains token reads + four action builders; each action builds a `TokenApi` template and delegates to `prepareCall`. The frontend Tokens route drives the shared `tx` flow (`awaitConfirm` → `TxModal` → `ConfirmPublish` → `TxResult`). No new backend pipeline.

## Components

### NomService additions (`app/nom_service.go`)

Reads (active address via WalletService, client via NodeService; not-connected → error, locked → `errLocked`):
- `GetMyTokens() ([]TokenInfo, error)` — `TokenApi.GetByOwner(active, …)` paginated until all `Count` fetched (page size 50); map each via pure `tokenInfoDTO(t *embedded.Token) TokenInfo`.
- `GetTokenByZts(zts string) (TokenInfo, error)` — trim + `types.ParseZTS(zts)` (validate before node use); `TokenApi.GetByZts` → `tokenInfoDTO`. A nil/empty result maps to `TokenInfo{}` (empty TokenStandard = not found).

Actions (build template → `prepareCall`; `data: append([]byte(nil), template.Data...)`; validate all inputs before any node use):
- `PrepareIssueToken(name, symbol, domain, totalSupply, maxSupply string, decimals int, isMintable, isBurnable, isUtility bool) (CallPreview, error)` — validate every field per the on-chain rules above (length, regex, decimals in [0, 18], supplies parse to non-negative big.Int, maxSupply > 0 and ≤ 2^255−1, maxSupply ≥ totalSupply, non-mintable ⇒ maxSupply == totalSupply). `TokenApi.IssueToken(…)` → `callExpect{to: TokenContract, zts: ZnnTokenStandard, amount: template.Amount, data}`; summary `Issue token <SYMBOL>`.
- `PrepareMint(zts, amount, receiver string) (CallPreview, error)` — `types.ParseZTS(zts)`; amount parse > 0; `types.ParseAddress(receiver)`; `TokenApi.Mint(…)` → `callExpect{TokenContract, ZnnTokenStandard, Big0, data}`; summary `Mint <amount> <SYMBOL/zts> to <receiver>`.
- `PrepareBurn(zts, amount string) (CallPreview, error)` — `types.ParseZTS(zts)`; amount parse > 0; `TokenApi.Burn(ztsParsed, amt)` → `callExpect{to: TokenContract, **zts: ztsParsed**, **amount: amt**, data}`; summary `Burn <amount> <zts>`.
- `PrepareUpdateToken(zts, newOwner string, isMintable, isBurnable bool) (CallPreview, error)` — `types.ParseZTS(zts)`; `types.ParseAddress(newOwner)`; `TokenApi.UpdateToken(…)` → `callExpect{TokenContract, ZnnTokenStandard, Big0, data}`; summary `Update token <zts>`.

### DTOs (`app/dto.go`)

- `TokenInfo{ Name, Symbol, Domain, TokenStandard, Owner string; TotalSupply, MaxSupply string; Decimals int; IsMintable, IsBurnable, IsUtility bool }` (supplies base-unit decimal strings; nil big.Int → "0"; TokenStandard is the `zts1…` string; empty ⇒ not found).

## Frontend

- **Tokens route** (`/tokens`, dashboard link + `'tokens'` nav view), mirroring `Pillars.svelte` tx wiring (`awaitConfirm` → `TxModal`/`TxResult`, refresh-on-done). Three sections:
  - **My tokens** (`GetMyTokens`): per owned token, show name/symbol/zts/decimals/totalSupply/maxSupply/flags. **Mint** form (amount + receiver prefilled to own address; shown only if `isMintable`) → `PrepareMint`. **Update** form (new owner prefilled to current owner; "Disable minting" toggle shown only while `isMintable`, "Disable burning" toggle shown only while `isBurnable` — defaults keep current, toggles only disable) → `PrepareUpdateToken(zts, owner, isMintable && !disableMint, isBurnable && !disableBurn)`.
  - **Look up / Burn**: ZTS input → `GetTokenByZts` → show token info; **Burn** form (amount; shown only if `isBurnable`) → `PrepareBurn`.
  - **Issue a token**: form (name, symbol, domain optional, decimals, totalSupply, maxSupply, isMintable, isBurnable, isUtility) → `PrepareIssueToken`; shows the 1 ZNN fee. Client-side disables submit on obvious invalids; Go re-validates authoritatively.
- **Stores:** a `token` store (`myTokens` list + `lookedUpToken` + `refreshTokens()` + `lookupToken(zts)`) using generated `models.ts` types; reuse the `tx` store/`awaitConfirm` bridge, `TxModal`/`TxResult`, `formatAmount`.
- Dashboard "Tokens" button + `App.svelte` `'tokens'` route branch, in the existing style.

## Error handling

- Invalid issue fields (length/regex/decimals/supply relationship) → rejected in Go before node use; the form surfaces the message; submit disabled on obvious client-side invalids.
- Invalid ZTS / address / non-positive amount (mint/burn/update) → rejected in Go before node use.
- Mint offered only when the owned token is mintable; Burn offered only when the looked-up token is burnable; node rejection (e.g. non-owner mint, exceeds maxSupply, insufficient balance to burn) surfaces on the result.
- Update toggles can only disable (true→false); the form never offers to enable a disabled flag.
- Mainnet attempt while `AllowMainnetSend` off → the guard's "mainnet sending is disabled" error.
- Publish failure → `TxResult` error; held block retained for retry/cancel (existing behavior).
- Reads tolerate a not-connected / locked node by leaving the store as-is (same pattern as the other NoM stores).

## Testing

- **Backend (Go, offline):** `tokenInfoDTO` mapper (fields + nil-supply → "0" + empty → not-found); `PrepareIssueToken` validation table (valid case + each rejection: bad name, bad symbol, bad domain, decimals > 18, maxSupply 0, maxSupply < totalSupply, non-mintable with maxSupply ≠ totalSupply); `PrepareMint`/`PrepareBurn`/`PrepareUpdateToken` reject bad zts / address / non-positive amount before any node use; a regression test that builds the **real** SDK templates (via `embedded.NewTokenApi(nil)`, offline — confirm builders don't deref the nil client) and asserts each `ToAddress == TokenContract`, the correct `TokenStandard` per call (**Burn = the passed token zts; Issue/Mint/Update = ZNN**), and Amounts (Issue = `TokenIssueAmount`; Mint/Update = 0; Burn = the passed amount); mainnet guard blocks unless `AllowMainnetSend`. **Integration (`//go:build integration`, opt-in):** extend `internal/spike` read smoke with `GetByOwner`/`GetByZts`.
- **Frontend (Vitest, mocked bindings):** my-tokens list renders; Issue form validation gates submit (e.g. non-mintable + maxSupply ≠ totalSupply disabled/errors); Mint shown only for mintable owned tokens; Burn shown after a successful ZTS lookup only if burnable; Update disable-toggles shown only while the flag is enabled; the prepare→confirm→publish store flow.
- **Acceptance (manual + live read smoke):** testnet — view my tokens; issue a token (1 ZNN fee, confirm-modal shows the built block); mint to an address; look up a token by ZTS and burn (confirm-modal shows **zts = the token, not ZNN**); update (transfer ownership / disable a flag); mainnet-gated. Live read smoke records `GetByOwner`/`GetByZts` output against the testnet node.

## Security

- Reuses the one audited prepare/confirm/publish path (binds to/zts/amount **and** ABI `Data`); mainnet gated by `AllowMainnetSend`; no key material in NomService.
- Each call's token standard verified against the SDK template — **Burn = the token's own ZTS (carries the burn amount); Issue/Mint/Update = ZNN** (Issue carries the 1 ZNN fee from the template) — confirm-what-you-sign shows the action summary + built block. The Burn case is the one where a wrong zts/amount would move the wrong asset, so the regression test + `assertMatches` Data/zts/amount binding are the guardrail.
- Residual (inherited Phase-5 note): full per-method ABI semantic decode of `Data` is bounded — `assertMatches` binds the exact `Data` bytes the template produced (prevents tampering, including the mint receiver / new owner / burned zts) but does not human-decode arbitrary params; tracked as Phase-5/7 hardening.

## Exit criteria (5e → 5f)

- View my tokens + look up by ZTS; issue; mint; burn; update — all through confirm-what-you-sign, mainnet-gated, with Go-side field validation mirroring the on-chain rules.
- `go test ./...` (offline) + frontend unit tests + `svelte-check` pass; the opt-in integration test compiles; live read smoke passes against the testnet node.

## Out of scope (deferred)

- Global all-tokens browser (`GetAll`); token-holdings list for burn discovery (user supplies the ZTS).
- Accelerator-Z (next sub-phase).
- Deeper ABI `Data` semantic decode (Phase-5/7 hardening).
