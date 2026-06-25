# Network Configuration (Chain ID) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add a configurable `Settings.chainId` (default 1) that the wallet stamps onto every built block, with a Settings Chain ID field + a connected-node mismatch warning, so the wallet builds/signs for the configured chain over an RPC node.

**Architecture:** `TxService` reads `ConfigService` settings and sets `template.ChainIdentifier = configuredChainID()` at the three block-building sites (`PrepareSend`, the shared `prepareCall`, `Receive`) before `PrepareBlock`/`Send`; the existing node-match validation stays as the safety gate. `NodeStatus` carries the node's chain id for the Settings warning. Embedded node unchanged (mainnet only).

**Tech Stack:** Go 1.25.11 (znn-sdk-go v0.1.19), Wails v2; Vue 3 + Pinia frontend.

## Global Constraints

- **Branch `network-chain-id`** (off `main` `bb7ee9c`). This is a **backend feature** — `app/*.go` changes ARE expected here (unlike the migration).
- **Funds-safety:** the chain id is committed in the **signed block**; the node-match validation (`b.ChainIdentifier != node.currentChainID()`) MUST stay as the safety gate so a misconfigured chain id can't publish on the wrong chain. The crypto path (keystore/derivation/hash/sign/PoW) is untouched — only which `ChainIdentifier` integer is set on the block template.
- **Default chain id = `1` (mainnet):** `configuredChainID()` normalizes unset/`0` → `1`. Preserves today's mainnet behavior.
- **All THREE block-building sites** must be stamped (`PrepareSend` ~L137, `prepareCall` ~L226, `Receive` ~L284 in `app/tx_service.go`) — missing one leaves that path deriving a different chain id.
- **Embedded node:** do NOT touch `internal/embeddednode` — mainnet only.
- **Commands (local):** `GOWORK=off GOTOOLCHAIN=auto go test ./... && go vet ./...`; bindings regen: `GOWORK=off ~/go/bin/wails generate module`; frontend in `frontend/`: `pnpm test`/`pnpm run typecheck`/`pnpm run build`. Commits GPG-signed: implementers STAGE only; keep the `wails dev` `go.mod` 2.12.0 churn out (but `wails generate module`'s `models.ts`/bindings regen for the DTO changes IS wanted).

## File Structure

- `app/dto.go` — `Settings.ChainID`, `NodeStatus.ChainID`.
- `app/node_service.go` — `NodeStatus()` + `emitStatus()` include `ChainID`.
- `app/tx_service.go` — `configuredChainID()` helper; stamp the 3 sites; clearer validation error.
- `frontend/wailsjs/**` — regenerated (DTO fields).
- `frontend/src/stores/node.ts` — surface `chainId` from `node:status`.
- `frontend/src/views/Settings.vue` — Network Configuration section.
- Tests: `app/*_test.go`, `frontend/src/views/Settings.test.ts`.

---

## Task 1: Backend DTOs + NodeStatus chain id + regenerate bindings

**Files:** Modify `app/dto.go`, `app/node_service.go`; regenerate `frontend/wailsjs`; test `app/node_service_test.go` (or the existing node test file).

**Interfaces:**
- Produces: `Settings.ChainID uint64` (json `chainId`); `NodeStatus.ChainID uint64` (json `chainId`), populated from `currentChainID()`.

- [ ] **Step 1: Add the DTO fields** in `app/dto.go` — in `Settings` add `ChainID uint64 \`json:"chainId"\`` (near `AllowMainnetSend`); in `NodeStatus` add `ChainID uint64 \`json:"chainId"\``.

- [ ] **Step 2: Populate `NodeStatus.ChainID`** in `app/node_service.go`. In `NodeStatus()`, read the chain id under the existing `RLock` and include it:

```go
func (n *NodeService) NodeStatus() NodeStatus {
	n.mu.RLock()
	connected := n.client != nil
	height := n.height
	mode := n.mode
	chainID := n.chainID
	n.mu.RUnlock()
	if mode == "" {
		mode = "remote"
	}
	return NodeStatus{Mode: mode, Connected: connected, Syncing: false, Height: height, Peers: 0, ChainID: chainID}
}
```
And in `emitStatus()` (which builds a `NodeStatus` for the `node:status` event), read `chainID := n.chainID` under its `RLock` and set `ChainID: chainID` on the emitted `st`.

- [ ] **Step 3: Test** — add to the node test: `NodeStatus()` returns `ChainID` equal to the service's `chainID` (set `n.chainID` via the test seam or after a mocked connect). Keep it a unit test (no live node).

- [ ] **Step 4: Regenerate bindings** — from repo root: `GOWORK=off ~/go/bin/wails generate module` (updates `frontend/wailsjs/go/models.ts` with the new `chainId` fields). If it errors, run `GOWORK=off ~/go/bin/wails dev` once then stop it. Revert any wails-2.12.0 `go.mod`/`go.sum` churn (`git checkout HEAD -- go.mod go.sum`); the `models.ts` regen IS wanted.

- [ ] **Step 5: Verify** — `GOWORK=off GOTOOLCHAIN=auto go test ./app/ && go vet ./app/` → pass. **Stage** `app/dto.go`, `app/node_service.go`, the test, `frontend/wailsjs` (models.ts). No commit.

---

## Task 2: TxService stamps the configured chain id (the 3 sites) + clearer validation

**Files:** Modify `app/tx_service.go`; test `app/tx_service_test.go`.

**Interfaces:**
- Consumes: `Settings.ChainID` (Task 1), `t.config.GetSettings()`, `mainnetChainID`.
- Produces: `func (t *TxService) configuredChainID() uint64`; every built block carries `ChainIdentifier == configuredChainID()`.

- [ ] **Step 1: Add the helper** to `app/tx_service.go`:

```go
// configuredChainID returns the chain identifier the wallet builds transactions
// for, from settings; unset/0 normalizes to mainnet. The built block is still
// validated against the connected node's chain before publish.
func (t *TxService) configuredChainID() uint64 {
	s, err := t.config.GetSettings()
	if err != nil || s.ChainID == 0 {
		return mainnetChainID
	}
	return s.ChainID
}
```

- [ ] **Step 2: Stamp the chain id at the three sites.**
  - `PrepareSend`: right after `template := client.LedgerApi.SendTemplate(to, zts, amount, nil)`, add `template.ChainIdentifier = t.configuredChainID()`.
  - `prepareCall`: right after the client/keypair are obtained and before `z.PrepareBlock(template, kp)`, add `template.ChainIdentifier = t.configuredChainID()` (the template is the `*nom.AccountBlock` param — this covers every NoM + token op).
  - `Receive`: right after `template := client.LedgerApi.ReceiveTemplate(hash)`, add `template.ChainIdentifier = t.configuredChainID()`.

- [ ] **Step 3: Clearer validation error** — at the publish-time check (`if b.ChainIdentifier != t.node.currentChainID()`), replace the message:

```go
	if b.ChainIdentifier != t.node.currentChainID() {
		return "", fmt.Errorf("configured Chain ID (%d) does not match the connected node's chain (%d); set the correct Chain ID in Settings or connect to a matching node", b.ChainIdentifier, t.node.currentChainID())
	}
```
(Ensure `fmt` is imported — it already is in `tx_service.go`.)

- [ ] **Step 4: Tests** in `app/tx_service_test.go`:
  - `configuredChainID()`: with `Settings.ChainID == 0` (or GetSettings error) → returns `1`; with `ChainID == 73404` → returns `73404`. (Use the existing ConfigService test seam / a temp data dir.)
  - The built template gets the configured chain id: assert that after setting `Settings.ChainID = 73404`, a built `SendTemplate`/`ReceiveTemplate` has `ChainIdentifier == 73404` before publish. If the existing tests build blocks with a mock client, assert the stamp; otherwise add a focused unit test on `configuredChainID()` + a test that the stamp line sets the field (a small helper test constructing a template and calling the stamp). Cover all three paths at least at the helper level.
  - Keep the live-node block-building behind the existing `//go:build integration` tests.

- [ ] **Step 5: Verify** — `GOWORK=off GOTOOLCHAIN=auto go test ./app/ && go vet ./app/` → pass. **Stage** `app/tx_service.go` + test. No commit.

---

## Task 3: Frontend — node store chainId + Settings Network Configuration section

**Files:** Modify `frontend/src/stores/node.ts`, `frontend/src/views/Settings.vue`, `frontend/src/views/Settings.test.ts`.

**Interfaces:**
- Consumes: `NodeStatus.chainId` (via `node:status`), `ConfigService.GetSettings`/`SetSettings` (carry `chainId`).
- Produces: `useNodeStore().chainId`; a Network Configuration section in Settings.

- [ ] **Step 1: Surface `chainId` in the node store** — in `frontend/src/stores/node.ts`, add `chainId: 0` to state and set it in the `node:status` handler: `EventsOn('node:status', (s) => { ...; this.chainId = s?.chainId ?? this.chainId })`. Also set it from `NodeStatus()` if the store reads it on connect.

- [ ] **Step 2: Add the Network Configuration section to `Settings.vue`** — a new section (near the Node section): a `chainId` ref loaded `onMounted` from `(await Cfg.GetSettings()).chainId` (default 1 if falsy); a nom-ui `Input` (numeric) bound to it; an Apply button that merges into settings and calls `Cfg.SetSettings` (read-modify-write: `const s = await Cfg.GetSettings(); s.chainId = Number(chainId); await Cfg.SetSettings(s)`); show the connected node's chain id (`node.chainId`) and a **mismatch warning** when `node.connected && node.chainId !== 0 && Number(chainId) !== node.chainId`: "Configured Chain ID {{chainId}} differs from the connected node's chain {{node.chainId}} — sends will be rejected until they match." Use the established theme classes; nom-ui Input/Button + local Field. `Cfg` = `import * as Cfg from '../../wailsjs/go/app/ConfigService'`.

- [ ] **Step 3: Test** in `Settings.test.ts` — add: the Chain ID field loads from `GetSettings().chainId` and Apply calls `SetSettings` with the entered chain id (read-modify-write); the mismatch warning renders when the mocked `node.chainId` differs from the configured value. Mock `ConfigService` + the node store.

- [ ] **Step 4: Verify** — `cd frontend && pnpm test -- src/views/Settings && pnpm run typecheck` → pass + clean. **Stage** node.ts, Settings.vue, Settings.test.ts. No commit.

---

## Task 4: Integration + full gate

- [ ] **Step 1: Backend gate** — `GOWORK=off GOTOOLCHAIN=auto go test ./... && go vet ./... && go build ./...` → pass (ignore the gopsutil/IOKit cgo warning).
- [ ] **Step 2: Frontend gate** — `cd frontend && pnpm test && pnpm run typecheck && pnpm run build` → all pass + clean + build OK.
- [ ] **Step 3: Security gates** (the feature touches Go) — `bash scripts/govulncheck-gate.sh` and `gosec -conf .gosec.json ./...` → pass (no new findings).
- [ ] **Step 4: Stage** any glue. No commit.

---

## Self-Review / Verification

- `go test ./...` + `go vet` + frontend `pnpm test`/`typecheck`/`build` green; govulncheck/gosec clean.
- All three block-building sites stamp `configuredChainID()`; the node-match validation stays with the clearer error; `NodeStatus.ChainID` surfaces to the frontend.
- **Live `wails dev` gate (controller, on testnet):** in Settings set Chain ID `73404`, connect to a testnet RPC node, send a small tx end-to-end (confirm modal → published); set Chain ID to `1` while on the testnet node → the send is rejected with the clear "configured Chain ID … does not match …" error + the Settings mismatch warning shows. Reset to `73404`. A NoM action (e.g. a small fuse) and a receive also publish on testnet (all three paths).
- Embedded node untouched (mainnet only).

## Closeout

After Task 4 green + a final review: merge `network-chain-id` → `main` (signed `--no-ff`, push), delete the branch, confirm CI green. Update memory.
