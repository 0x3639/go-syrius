# syrius Compatibility Notes (Phase 0 findings)

Empirical results from the Phase 0 de-risking spike. These are verified against
real keystores and live nodes, not assumptions.

## Keystore format

- **Canonical source:** go-zenon `wallet/keyfile.go` + `wallet/crypto.go`. syrius
  produces this same format (plus a few extra Argon2 fields, see below).
- **Payload:** the encrypted blob is the **raw BIP-39 entropy bytes** — not JSON.
- **Cipher:** AES-256-GCM, 12-byte nonce, additional-authenticated-data = `"zenon"`,
  key = Argon2id-derived `[:32]`.
- **KDF:** Argon2id (`kdf: "argon2.IDKey"`), Memory 64 MiB, Iterations 1,
  Parallelism 4, 16-byte salt, 32-byte key.
- **File JSON:** `baseAddress`, `crypto{cipherName:"aes-256-gcm", kdf, cipherData,
  nonce, argon2Params}`, `version: 1`, `timestamp`. A syrius-written keystore also
  carries `argon2Params.{timeCost, memoryCost, hashLength, parallelism}`; go-zenon
  stores only `salt` and uses fixed params (the extra fields are ignored on read).

> **Historical Phase 0 finding:** the SDK version used during the spike could not
> read real syrius keystores. Its crypto layer was byte-identical
> to go-zenon (decryption of a real keystore succeeds), but `wallet/keystore.go`
> JSON-wraps the entropy on write and `json.Unmarshal`s the decrypted payload on read,
> so it fails on raw-entropy keystores with `invalid character 'ù'`. **go-syrius therefore
> uses `github.com/zenon-network/go-zenon/wallet` directly** for keystore read/derive/write
> (`ReadKeyFile` → `Decrypt` → `DeriveForIndexPath`; `Encrypt`/`Write` for write-compat).
> As of `znn-sdk-go` v0.2.1, the SDK reads interoperable raw-entropy key files, persists
> all Argon2 parameters, validates `baseAddress` during decryption, and exposes legacy
> upgrade detection. The wallet retains its already-reviewed direct go-zenon keystore path.
>
> **Verified:** opening a real syrius keystore via go-zenon derives index-0 =
> the recorded `baseAddress` (`internal/compat`).

## Address derivation

- BIP-44 path `m/44'/73404'/index'`, SLIP-0010 ed25519 (hardened-only), address =
  `types.PubKeyToAddress(pubkey)` → `z1…`.
- **SDK and go-zenon derivations agree:** Task 6 builds the signing keypair from the
  mnemonic via `znn-sdk-go/wallet` and asserts its index-0 address equals go-zenon's
  `baseAddress` before sending.

## Chain identifiers

- **Mainnet:** `chainIdentifier = 1` (observed on my.hc1node.com, frontier ~13.5M).
- **Testnet:** `chainIdentifier = 73404` (observed on the testnet nodes).
- `zenon.Send` autofills `ChainIdentifier` from the node's frontier momentum, so it is
  not set by client code.

## Transaction send flow

- go-syrius uses the SDK's `zenon` facade: `zenon.NewZenon(client).Send(template, kp)`
  performs autofill → required-PoW query → PoW (or plasma) → hash → sign → publish.
  First released in **znn-sdk-go v0.1.16**; go-syrius now pins v0.2.1.
- **Verified on testnet:** a 0.1 ZNN self-send confirmed on-chain — tx
  `80d6f0b04fc7cc76482125ab8d99080df50d5528da807097d8c1f351a3caff00`, confirmed at
  momentum height 440.
- That send used the **plasma** path (`requiredDifficulty: 0`, wallet had 1000 QSR
  fused), so on-chain PoW was **not** exercised in this run. What Gate 0→1 actually
  proves is the autofill → sign → publish → confirm path end-to-end against a live
  testnet. PoW *algorithm* correctness is independently covered by the SDK's `pow`
  tests (canonical go-zenon match).
- **Carried forward as a REQUIRED Gate 2→mainnet item (not optional):** a real
  end-to-end PoW send — from an address with no fused plasma so `requiredDifficulty > 0`
  — must pass before any mainnet send path is enabled. Until then the integration-level
  PoW path is unproven for go-syrius.

## Node RPC requirements

- Method names use the **`embedded.` prefix**: e.g. `embedded.plasma.getRequiredPoWForAccountBlock`,
  `embedded.plasma.get` (confirmed against a node that has it; the prefix is correct — do not strip it).
- The node must enable the **`embedded`** namespace in `RPC.Endpoints` (alongside
  `ledger`/`stats`) **and be restarted** — go-zenon registers RPC namespaces only at
  startup. A node missing it returns JSON-RPC `-32601` ("method does not exist") for all
  `embedded.*` calls while `ledger.*` still works. Confirm a node's live namespaces with
  the `rpc.modules` introspection call.
- `zenon.Send` depends on `embedded.plasma.getRequiredPoWForAccountBlock`; it cannot run
  against a node without the `embedded` namespace.

## SDK dependency

- Pinned `github.com/0x3639/znn-sdk-go v0.2.1`, no SDK `replace`. The stable SDK omits
  the testnet governance extension used by this wallet, so `internal/governance` owns
  that narrow adapter on top of the SDK's public transport and contract-template APIs.
  go-zenon (`v0.0.8-alphanet…`, replaced by the pinned project fork) remains a direct
  dependency and is used for keystore operations and canonical governance ABI definitions.

## Phase 0 exit criteria (Gate 0 → 1)

- [x] `go test ./...` (offline) passes, including the keystore round-trip and SDK smoke test.
- [x] Read-only RPC integration test passes against a live node (mainnet, frontier height + balances).
- [x] Testnet send integration test confirms a transaction on-chain.
- [x] Compatibility note committed; go-syrius pins a released SDK version (v0.1.16, no `replace`).

**Gate 0 → 1: PASSED** (scoped). Proven: keystore compatibility, and a testnet
transaction confirmed end-to-end via the autofill→sign→publish path (plasma-funded).
Not yet proven at integration level: an end-to-end PoW send — this is a **required**
Gate 2→mainnet item (see above), not a Gate 0→1 blocker. The foundation holds —
Phase 1 (Wails skeleton + read-only wallet) may begin.
