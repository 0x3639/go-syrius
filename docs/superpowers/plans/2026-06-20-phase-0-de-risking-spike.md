# Phase 0 — De-risking Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Empirically prove syrius wallet-file compatibility and end-to-end transaction validity (build → autofill → PoW → sign → publish) before any UI work begins.

**Architecture:** A new `github.com/0x3639/go-syrius` Go module that consumes `znn-sdk-go` and `go-zenon` as imports. Phase 0 produces (a) a committed keystore-compatibility test against a real syrius `.dat`, (b) read-only RPC verification against a live node, and (c) a confirmed testnet transaction. The transaction-assembly orchestration (autofill → required-PoW → canonical PoW → sign → publish) **already exists in `znn-sdk-go`** as the `zenon` package facade (`zenon.NewZenon(client).Send(template, kp)`); go-syrius consumes it directly. No SDK code is written in Phase 0.

> **Revision (2026-06-20):** This plan originally targeted `znn-sdk-go@v0.1.12`, which lacked any send orchestration, so Tasks 3–4 hand-wrote `utils.AutofillBlock` + `rpc_client.Send` in a local SDK clone consumed via `replace`. Discovery during execution: the SDK `master` branch already provides a tested `zenon` facade doing exactly this (PR #3; `feat(zenon): add high-level transaction send flow`, plus version/chainId-normalization and canonical-PoW fixes). Per the SDK-first rule, the fix lives in the SDK — so **Tasks 3 & 4 are removed**, the `replace`/clone is dropped, and go-syrius pins the released tag (**v0.1.16**, cut from master) that contains the `zenon` package. Task 6 uses `zenon.Send`.

**Tech Stack:** Go 1.24+, `github.com/0x3639/znn-sdk-go`, `github.com/zenon-network/go-zenon` (via the SDK), WebSocket RPC.

## Global Constraints

- Module path: `github.com/0x3639/go-syrius`; Go directive `go 1.24` (matches SDK `go.mod`; local toolchain is go1.25.4).
- **SDK dependency:** pin `github.com/0x3639/znn-sdk-go v0.1.16` (the release cut from master that contains the `zenon` send-flow facade). No `replace` directive; no local SDK clone.
- **SDK-first rule (user directive):** any missing or broken SDK behavior is fixed in `znn-sdk-go` and tested there — never worked around in this repo. (The send-flow gap that prompted this rule is already resolved upstream; see the Architecture revision note.)
- **Throwaway only:** the reference keystore committed to this repo is a disposable wallet with no real funds; its password may be hardcoded in the compat test because the wallet is worthless. Never commit a `.dat` holding real funds.
- **Secrets discipline:** never log mnemonics, private keys, or decrypted keystore contents.
- **Amounts:** ZNN/QSR/ZTS use 8 decimals — `1 ZNN = 100_000_000` base units (`*big.Int`).
- **Test isolation:** unit tests are offline and deterministic and run under plain `go test ./...`. Anything needing a live node or real funds is guarded by the `//go:build integration` tag and reads node URLs from environment variables, so the default suite never touches the network.

## Verified SDK API surface (from `znn-sdk-go` master / v0.1.16 + `go-zenon`)

These are confirmed signatures this plan builds on (read from the SDK source):

- **`zenon` facade (the send orchestration — use this instead of hand-assembling blocks):**
  - `zenon.NewZenon(client *rpc_client.RpcClient) *zenon.Zenon`
  - `(*zenon.Zenon) Send(template *nom.AccountBlock, kp *wallet.KeyPair) (*nom.AccountBlock, error)` — autofill → required-PoW → canonical PoW → sign → publish; returns the finalized published block
  - `(*zenon.Zenon) PrepareBlock(template, kp) (*nom.AccountBlock, error)` (everything except publish) · `RequiresPoW(template, kp) (bool, error)`
  - field `Zenon.PowCallback func(pow.PowStatus)` — PoW progress hook (for future UI events)
- `wallet.NewKeyStoreManager(walletPath string) (*KeyStoreManager, error)`
- `(*KeyStoreManager) ReadKeyStore(password, keyStoreFile string) (*KeyStore, error)`
- `(*KeyStore) GetKeyPair(account int) (*KeyPair, error)`
- `(*KeyPair) GetAddress() (*types.Address, error)` · `GetPublicKey() ([]byte, error)` · `Sign(message []byte) ([]byte, error)`
- `rpc_client.NewRpcClient(url string) (*RpcClient, error)` · `(*RpcClient) Stop()`; fields `.LedgerApi`, `.PlasmaApi`, `.SubscriberApi`
- `(*LedgerApi) GetFrontierMomentum() (*api.Momentum, error)` — `api.Momentum` embeds `*nom.Momentum`, exposing `.ChainIdentifier`, `.Hash`, `.Height`
- `(*LedgerApi) GetFrontierAccountBlock(address types.Address) (*api.AccountBlock, error)` — `.Hash`, `.Height`
- `(*LedgerApi) GetAccountInfoByAddress(address types.Address) (*api.AccountInfo, error)` — `.Address`, `.BalanceInfoMap`
- `(*LedgerApi) GetUnreceivedBlocksByAddress(address types.Address, pageIndex, pageSize uint32) (*api.AccountBlockList, error)`
- `(*LedgerApi) SendTemplate(toAddress types.Address, tokenStandard types.ZenonTokenStandard, amount *big.Int, data []byte) *nom.AccountBlock` (sets only `BlockType`, `ToAddress`, `TokenStandard`, `Amount`, `Data`)
- `(*LedgerApi) PublishRawTransaction(*nom.AccountBlock) error`
- `(*PlasmaApi) GetRequiredPoWForAccountBlock(embedded.GetRequiredParam) (*embedded.GetRequiredResult, error)` — result has `RequiredDifficulty uint64`
- `pow.GeneratePoW(dataHash types.Hash, difficulty uint64) string` · `pow.GeneratePowAsync(ctx, dataHash, difficulty) <-chan pow.PowResult` (`PowResult{Nonce string; Error error}`, `pow.ErrCancelled`)
- `utils.GetPoWData(block *nom.AccountBlock) types.Hash` (= SHA3-256(address‖previousHash)) · `utils.GetTransactionHash(block *nom.AccountBlock) types.Hash`
- `nom.AccountBlock` fields to populate: `Version`(must be 1), `ChainIdentifier`, `BlockType`, `PreviousHash`, `Height`, `MomentumAcknowledged types.HashHeight`, `Address`, `Nonce nom.Nonce`(`Data [8]byte`, `UnmarshalText` accepts 16 hex chars), `Difficulty`, `Hash`, `PublicKey ed25519.PublicKey`, `Signature []byte`
- `nom.BlockTypeUserSend`, `nom.BlockTypeUserReceive`; `nom.DeSerializeNonce([]byte) Nonce`
- `types.HashHeight{Hash types.Hash; Height uint64}`; `types.ParseAddressPanic(string) types.Address`; `types.HexToHashPanic(string) types.Hash`

The `nom.AccountBlock` field/PoW/utils details above are retained for reference (they explain what `zenon.Send` does internally and inform Phase 0's compatibility note), but Phase 0 code calls `zenon.Send` rather than touching these directly.

---

## Task 1: Initialize the module and pin the SDK

> **Status: done, with a revision applied.** The module + smoke test were implemented (commits `b885711`, `fc4c3e4`). The original Step 1/2 cloned the SDK and added a `replace` directive; per the Architecture revision this is dropped in favor of pinning the released `zenon`-bearing tag. The completing change: drop the `replace`, pin `v0.1.16`, re-tidy.

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `internal/version/version.go` (trivial, gives the smoke build something real to compile)
- Create: `internal/version/version_test.go`

**Interfaces:**
- Produces: a buildable module at `github.com/0x3639/go-syrius` depending on `github.com/0x3639/znn-sdk-go v0.1.16` (no `replace`).

- [ ] **Step 1: Initialize the go-syrius module and pin the SDK** (requires the SDK `v0.1.16` tag to be pushed first)

```bash
cd /Users/dfriestedt/Documents/go-syrius
go mod init github.com/0x3639/go-syrius   # (already done)
go mod edit -dropreplace github.com/0x3639/znn-sdk-go   # remove the Phase-0 replace if present
go get github.com/0x3639/znn-sdk-go@v0.1.16
```

- [ ] **Step 3: Write the failing smoke test**

`internal/version/version_test.go`:
```go
package version

import "testing"

func TestPhase(t *testing.T) {
	if Phase != "0" {
		t.Fatalf("Phase = %q, want \"0\"", Phase)
	}
}
```

- [ ] **Step 4: Run it to verify it fails**

Run: `go test ./internal/version/`
Expected: FAIL — `undefined: Phase` (package doesn't compile yet).

- [ ] **Step 5: Implement the minimal package**

`internal/version/version.go`:
```go
// Package version records which delivery phase this build corresponds to.
package version

// Phase is the syrius-wails delivery phase implemented by this build.
const Phase = "0"
```

- [ ] **Step 6: Tidy and verify**

Run: `go mod tidy && go test ./internal/version/ && go build ./...`
Expected: PASS. After a real SDK import is added (Task 5/6), `go mod tidy` populates `go.sum` with `znn-sdk-go v0.1.16` + transitive `go-zenon` hashes, proving the pinned release resolves.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/version/
git commit -m "chore: initialize go module and pin znn-sdk-go v0.1.16"
```

---

## Task 2: Keystore round-trip compatibility test (headline de-risk) — DONE

> **Revision (2026-06-20):** Done, but NOT via the SDK. Discovery: `znn-sdk-go`'s
> wallet **cannot read real syrius keystores** — its crypto is byte-identical to
> go-zenon (so decryption succeeds) but it JSON-wraps the entropy payload, while
> syrius/go-zenon encrypt **raw BIP-39 entropy**, so `FromEncryptedFile` fails with
> `invalid character 'ù'` when JSON-parsing raw entropy. Per the user's directive to
> **not modify the SDK**, go-syrius reads/derives keystores through **go-zenon's
> canonical `wallet` package directly** (`wallet.ReadKeyFile` → `(*KeyFile).Decrypt`
> → `(*KeyStore).DeriveForIndexPath`, compared to `KeyFile.BaseAddress`). The SDK is
> used only for RPC, PoW, and `zenon.Send`. Write-compat (Phase 3) likewise uses
> go-zenon's `(*KeyStore).Encrypt` + `(*KeyFile).Write`.

**Files:**
- Create: `internal/compat/keystore_compat_test.go`
- Create: `internal/compat/doc.go`
- (No committed keystore: the real keystore is read from the gitignored `secrets/` folder at runtime; `.gitignore` added.)

**Interfaces:**
- Consumes: `github.com/zenon-network/go-zenon/wallet` — `ReadKeyFile`, `(*KeyFile).Decrypt`, `(*KeyStore).DeriveForIndexPath`, `KeyFile.BaseAddress`, `KeyPair.Address`.
- Produces: a secret-free, skip-if-absent test asserting go-syrius derives the same index-0 address syrius recorded. Verified against a real keystore (`baseAddress z1qrr0…8wpcjmg`).

- [ ] **Step 1: (Manual, P0-a) Create the reference wallet in real syrius**

Install the current Flutter syrius release. Create a new wallet. Record three things:
1. The wallet’s index-0 address as syrius displays it (`z1…`).
2. The password you set.
3. The keystore filename syrius wrote (under syrius’s wallet directory).

Copy that keystore file into this repo:
```bash
mkdir -p internal/compat/testdata
cp "<syrius-wallet-dir>/<keystore-file>" internal/compat/testdata/reference-wallet.dat
```
Do **not** fund this wallet, and never fund it later. The committed compat keystore exists only to prove file-format compatibility and must stay at zero balance forever. (Task 6's testnet send uses a *separate, uncommitted* wallet supplied via env vars — see Task 6 Step 1.)

- [ ] **Step 2: Write the failing compat test**

`internal/compat/keystore_compat_test.go` (replace the two `REPLACE_ME` constants with the values from Step 1):
```go
package compat

import (
	"path/filepath"
	"testing"

	"github.com/0x3639/znn-sdk-go/wallet"
)

// Throwaway reference wallet — see internal/compat/doc.go. No real funds.
const (
	referenceKeystoreFile = "reference-wallet.dat"
	referencePassword     = "REPLACE_ME_password"
	referenceAddress0     = "REPLACE_ME_z1address"
)

func TestSyriusKeystoreRoundTrip(t *testing.T) {
	dir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}

	mgr, err := wallet.NewKeyStoreManager(dir)
	if err != nil {
		t.Fatalf("NewKeyStoreManager: %v", err)
	}

	ks, err := mgr.ReadKeyStore(referencePassword, referenceKeystoreFile)
	if err != nil {
		t.Fatalf("ReadKeyStore (SDK cannot open a real syrius .dat — fix in znn-sdk-go/wallet): %v", err)
	}

	kp, err := ks.GetKeyPair(0)
	if err != nil {
		t.Fatalf("GetKeyPair(0): %v", err)
	}

	addr, err := kp.GetAddress()
	if err != nil {
		t.Fatalf("GetAddress: %v", err)
	}

	if got := addr.String(); got != referenceAddress0 {
		t.Fatalf("index-0 address mismatch:\n  got  %s\n  want %s\n(address derivation is NOT syrius-compatible — fix in znn-sdk-go/wallet)", got, referenceAddress0)
	}
}
```

- [ ] **Step 3: Run it to verify it fails first** (before adding the `.dat`/constants, or with placeholder constants)

Run: `go test ./internal/compat/ -run TestSyriusKeystoreRoundTrip -v`
Expected: FAIL (missing file or address mismatch). This confirms the test actually checks something.

- [ ] **Step 4: Fill in the real constants and `.dat`, then run to pass**

Run: `go test ./internal/compat/ -run TestSyriusKeystoreRoundTrip -v`
Expected: PASS.
**If `ReadKeyStore` fails or the address mismatches:** STOP. This is a real SDK compatibility defect. Diagnose in `../znn-sdk-go/wallet/` (compare `encryptedfile.go` / `derivation.go` against `go-zenon/wallet/keyfile.go`), fix and unit-test it **in the SDK**, then rerun this test. Do not adjust this test to mask a mismatch.

- [ ] **Step 5: Document the testdata**

`internal/compat/doc.go`:
```go
// Package compat holds tests proving byte/behaviour compatibility with the
// original Flutter syrius wallet.
//
// testdata/reference-wallet.dat is a THROWAWAY syrius keystore with no funds,
// committed solely so TestSyriusKeystoreRoundTrip can prove that znn-sdk-go
// opens real syrius keystores and derives identical addresses. Never place a
// funded keystore here.
package compat
```

- [ ] **Step 6: Commit**

```bash
git add internal/compat/
git commit -m "test: prove SDK opens real syrius keystore with matching address"
```

---

## Tasks 3 & 4: REMOVED (SDK already provides the send flow)

> These tasks hand-wrote `utils.AutofillBlock` and `(*RpcClient).Send` in a local SDK clone, against `znn-sdk-go@v0.1.12` which had no send orchestration. **Removed during execution:** the SDK `master` branch (released as `v0.1.16`) already provides a tested `zenon` package facade that does the entire autofill → required-PoW → canonical PoW → sign → publish flow — `zenon.NewZenon(client).Send(template, kp)`, plus `PrepareBlock`, `RequiresPoW`, and a `PowCallback` progress hook. Per the SDK-first rule, that orchestration belongs in the SDK, and it is there. go-syrius pins `v0.1.16` (Task 1) and calls `zenon.Send` (Task 6). No autofill/send code is written in this repo.

---

## Task 5: Read-only RPC verification (integration, live node)

**Files:**
- Create: `internal/spike/readonly_integration_test.go`

**Interfaces:**
- Consumes: `rpc_client.NewRpcClient`, `(*LedgerApi).GetFrontierMomentum`, `(*LedgerApi).GetAccountInfoByAddress`.
- Produces: evidence the SDK connects to a live node and reads momentum height + balances.

- [ ] **Step 1: Write the integration test**

`internal/spike/readonly_integration_test.go`:
```go
//go:build integration

package spike

import (
	"os"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
)

// Env:
//   ZNN_NODE_URL  — wss:// or ws:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address to read balances for (required)
func TestReadOnlyRPC(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if url == "" || addrStr == "" {
		t.Skip("set ZNN_NODE_URL and ZNN_TEST_ADDR to run")
	}

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()

	momentum, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		t.Fatalf("GetFrontierMomentum: %v", err)
	}
	if momentum.Height == 0 {
		t.Fatalf("frontier momentum height is 0, expected a live chain")
	}
	t.Logf("frontier height=%d chainId=%d", momentum.Height, momentum.ChainIdentifier)

	addr := types.ParseAddressPanic(addrStr)
	info, err := client.LedgerApi.GetAccountInfoByAddress(addr)
	if err != nil {
		t.Fatalf("GetAccountInfoByAddress: %v", err)
	}
	for zts, bal := range info.BalanceInfoMap {
		t.Logf("balance %s = %v", zts, bal.Balance)
	}
}
```

- [ ] **Step 2: Run it against your mainnet node**

Run:
```bash
ZNN_NODE_URL="<your wss:// mainnet node>" \
ZNN_TEST_ADDR="z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7" \
go test ./internal/spike/ -tags integration -run TestReadOnlyRPC -v
```
Expected: PASS; logs a non-zero frontier height and any balances. Confirm the default suite still ignores it: `go test ./...` (no `-tags`) does not run this file.

- [ ] **Step 3: Commit**

```bash
git add internal/spike/readonly_integration_test.go
git commit -m "test: integration read-only RPC against a live node"
```

---

## Task 6: Testnet end-to-end transaction (integration, the PoW+sign proof)

> **Keypair bridge (per the no-SDK-modification + keystore findings):** the SDK
> can't read syrius keystores, and `zenon.Send` needs an SDK `*wallet.KeyPair`.
> So Task 6's flow is: read the keystore with **go-zenon** (`wallet.ReadKeyFile`
> → `(*KeyFile).Decrypt`) to recover the mnemonic, then build the SDK keypair
> via `znnsdkwallet.NewKeyStoreFromMnemonic(mnemonic)` → `GetKeyPair(0)`, and
> pass that to `zenon.NewZenon(client).Send(template, kp)`. The two derivations
> must yield the same address — assert that equality (it cross-checks SDK vs
> go-zenon BIP-44 derivation). The test code below predates this and must be
> updated accordingly when Task 6 runs.

**Files:**
- Create: `internal/spike/send_integration_test.go`

**Interfaces:**
- Consumes: `rpc_client.NewRpcClient`, `zenon.NewZenon` + `(*zenon.Zenon).Send`, `(*KeyStoreManager)`/`KeyStore`/`KeyPair`, `(*LedgerApi).GetAccountBlockByHash`.
- Produces: a confirmed on-chain testnet transaction — proving the SDK's autofill + PoW + signing + publish flow end-to-end.

- [ ] **Step 1: (Manual, P0-b) Acquire testnet access and funds**

Obtain a testnet node URL (`wss://`/`ws://`). Create a throwaway testnet wallet — a **separate, uncommitted** keystore, *not* the committed Task 2 compat `.dat`, which must never hold funds — stored outside the repo and referenced only through env vars (Step 2). Fund its index-0 address from the Zenon testnet faucet, then confirm a non-zero balance via the Task 5 test pointed at the testnet node.

- [ ] **Step 2: Write the end-to-end test**

`internal/spike/send_integration_test.go`:
```go
//go:build integration

package spike

import (
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/0x3639/znn-sdk-go/wallet"
	"github.com/0x3639/znn-sdk-go/zenon"
	"github.com/zenon-network/go-zenon/common/types"
)

// Env:
//   ZNN_TESTNET_URL  — testnet node URL (required)
//   ZNN_WALLET_DIR   — directory holding the keystore (required)
//   ZNN_WALLET_FILE  — keystore filename (required)
//   ZNN_WALLET_PASS  — keystore password (required)
//   ZNN_SEND_TO      — recipient z1… on testnet (required)
func TestTestnetSend(t *testing.T) {
	url := os.Getenv("ZNN_TESTNET_URL")
	dir := os.Getenv("ZNN_WALLET_DIR")
	file := os.Getenv("ZNN_WALLET_FILE")
	pass := os.Getenv("ZNN_WALLET_PASS")
	to := os.Getenv("ZNN_SEND_TO")
	if url == "" || dir == "" || file == "" || pass == "" || to == "" {
		t.Skip("set ZNN_TESTNET_URL, ZNN_WALLET_DIR, ZNN_WALLET_FILE, ZNN_WALLET_PASS, ZNN_SEND_TO to run")
	}

	mgr, err := wallet.NewKeyStoreManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	ks, err := mgr.ReadKeyStore(pass, file)
	if err != nil {
		t.Fatal(err)
	}
	kp, err := ks.GetKeyPair(0)
	if err != nil {
		t.Fatal(err)
	}

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Stop()

	toAddr := types.ParseAddressPanic(to)
	amount := big.NewInt(1 * 100_000_000) // 1 ZNN

	template := client.LedgerApi.SendTemplate(toAddr, types.ZnnTokenStandard, amount, nil)

	z := zenon.NewZenon(client)
	published, err := z.Send(template, kp)
	if err != nil {
		t.Fatalf("zenon.Send: %v", err)
	}
	t.Logf("published tx hash=%s height=%d", published.Hash, published.Height)

	// Poll for on-chain confirmation.
	deadline := time.Now().Add(90 * time.Second)
	for {
		got, err := client.LedgerApi.GetAccountBlockByHash(published.Hash)
		if err == nil && got != nil && got.ConfirmationDetail != nil {
			t.Logf("confirmed at momentum height %d", got.ConfirmationDetail.MomentumHeight)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tx %s not confirmed within deadline", published.Hash)
		}
		time.Sleep(3 * time.Second)
	}
}
```

- [ ] **Step 3: Run it against testnet**

Run:
```bash
ZNN_TESTNET_URL="<testnet node>" \
ZNN_WALLET_DIR="$PWD/internal/compat/testdata" \
ZNN_WALLET_FILE="reference-wallet.dat" \
ZNN_WALLET_PASS="<password>" \
ZNN_SEND_TO="<a second testnet z1 you control>" \
go test ./internal/spike/ -tags integration -run TestTestnetSend -v
```
Expected: PASS; logs a tx hash and confirmation. This is the Gate 0→1 transaction proof.
**If publish is rejected:** the failure mode points at the SDK's send flow (e.g. wrong `ChainIdentifier`, hash-field ordering, nonce encoding). Per the SDK-first rule, reproduce it as a failing test in `znn-sdk-go/zenon`, fix it there, cut a new SDK release, re-pin in Task 1, then retry — do not patch around it in this repo.

- [ ] **Step 4: Commit**

```bash
git add internal/spike/send_integration_test.go
git commit -m "test: end-to-end testnet send proving PoW + signing path"
```

---

## Task 7: Compatibility note + Phase 0 exit checklist

**Files:**
- Create: `docs/compatibility-notes.md`

**Interfaces:**
- Produces: the durable record Phase 0 is meant to leave behind (Argon2/keystore params, chain identifiers, the SDK changes made).

- [ ] **Step 1: Write the compatibility note**

`docs/compatibility-notes.md`:
```markdown
# syrius Compatibility Notes (Phase 0 findings)

## Keystore format (verified against a real syrius .dat)
- KDF: Argon2id. Default params (znn-sdk-go `crypto/argon2.go`): Memory 64*1024 KB (64 MB),
  SaltLength 16 bytes, KeyLength 32 bytes. (Record the time/parallelism values observed in
  the actual file here.)
- Cipher: AES-256-GCM (`cipherName: "aes-256-gcm"`).
- File layout: JSON with `crypto.argon2Params.salt`, `crypto.cipherData`, `crypto.cipherName`,
  `crypto.nonce` (all `0x`-prefixed hex). Confirmed `wallet.ReadKeyStore` opens it and derives
  the same index-0 address syrius shows (see internal/compat).

## Address derivation
- BIP39 → BIP44 → Ed25519 → z1…; index-0 address matches syrius byte-for-byte.

## Transaction assembly (provided by the SDK `zenon` facade, v0.1.16)
- go-syrius calls `zenon.NewZenon(client).Send(template, kp)`; the SDK performs autofill →
  required-PoW → canonical PoW → hash → sign → publish.
- For reference, internally: hash preimage field order is in `utils/block.go` GetTransactionBytes;
  PoW data = SHA3-256(address ‖ previousHash); required difficulty via
  `embedded.plasma.getRequiredPoWForAccountBlock`; ChainIdentifier comes from the frontier momentum
  (mainnet vs testnet differ — record both observed values here once known).

## SDK dependency
- Pinned `github.com/0x3639/znn-sdk-go v0.1.16` (release cut from master; first tag containing the
  `zenon` send-flow facade). No `replace`; no SDK code authored in this repo.
```

Fill the parenthesized blanks with the concrete values observed during Tasks 2/5/6.

- [ ] **Step 2: Commit**

```bash
git add docs/compatibility-notes.md
git commit -m "docs: record Phase 0 keystore and transaction compatibility findings"
```

- [ ] **Step 3: (No SDK changes to upstream)**

The send-flow orchestration this plan once added is already upstream and released as `znn-sdk-go v0.1.16` (the `zenon` facade), which go-syrius pins in Task 1. No SDK branch, PR, or `replace` removal is needed — that work pre-dates this plan's revision. This step is a no-op retained only to mark that the SDK dependency is a clean pinned release.

- [ ] **Step 4: Confirm Phase 0 exit criteria (Gate 0→1)**

All must hold:
- `go test ./...` (offline) passes, including the keystore round-trip and the SDK facade smoke test.
- Read-only RPC integration test passes against the live node.
- Testnet send integration test confirms a transaction on-chain.
- Compatibility note committed; go-syrius pins a released SDK version (`v0.1.16`, no `replace`).

---

## Self-Review

**Spec coverage** (against `docs/superpowers/specs/2026-06-20-syrius-wails-roadmap-design.md`, Phase 0 + adaptation):
- P0-a acquire reference keystore → Task 2 Step 1. ✓
- P0-b acquire testnet access/funds → Task 6 Step 1. ✓
- New Go module importing the SDK → Task 1. ✓
- Keystore round-trip (open real .dat, derive index 0, match address) → Task 2. ✓
- Read-only RPC (frontier momentum + balances) → Task 5. ✓
- Testnet tx (build→autofill→PoW→sign→publish, confirm) → Task 6 (e2e) via the SDK's `zenon.Send` (Tasks 3/4 removed — the SDK already provides this). ✓
- Record Argon2 params / keystore layout → Task 7. ✓
- SDK-first directive (fix in SDK, not this repo) → Tasks 3, 4, and the "fix in znn-sdk-go" stop-conditions in Tasks 2/6. ✓

**Placeholder scan:** The only intentional fill-ins are the throwaway wallet's real `password`/`address`/`.dat` (Task 2) and observed numeric params in the compat note (Task 7) — these are runtime-acquired values, not design gaps, and each has an explicit step to supply them. No `TODO`/`TBD`/"handle edge cases".

**Type consistency:** `zenon.NewZenon(client) *zenon.Zenon` and `(*zenon.Zenon).Send(template, kp) (*nom.AccountBlock, error)` match the SDK facade read from source (Task 6 call site). `wallet`, `rpc_client`, and `types` usages match the verified API surface.

**Known runtime risk (flagged, not hidden):** Phase 0 now relies on the SDK's `zenon.Send` rather than hand-written assembly, which removes the prior nonce-encoding uncertainty. The residual unknowns are external and gated by the integration tests: keystore-format compatibility (Task 2) and a live testnet publish actually confirming (Task 6). Either failing routes a fix into `znn-sdk-go` per the SDK-first rule.
