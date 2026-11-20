# Bend laws for go-syrius

The wallet must sign only what the user approved, preserve uncertain publication
outcomes, and keep obsolete work from taking ownership of current state.
`test/bend/` makes selected parts of these contracts machine-checkable.

This follows [go-zenon PR #314](https://github.com/zenon-network/go-zenon/pull/314):
prove a small model, compare it with real code, then deliberately break both to
check that failures are detected. The [linked review](https://github.com/zenon-network/go-zenon/pull/314#discussion_r4628504012)
also motivates checking the entire effect and journal payload, including sender,
recipient, token, and call data, rather than checking only an amount.

## Run locally

Requirements: Docker (running), Go from `go.mod`, Python 3.9+ with its standard
library, and Bash. No host Node, Bend, Lean, or Python packages are installed.
With Colima, keep the checkout under `/Users/user` so the VM can mount it.

```bash
test/bend/run.sh
BEND_DEPTH=4 test/bend/run.sh             # longer exhaustive sequences
BEND_SEED=42 BEND_RANDOM=200 test/bend/run.sh # different/longer sampled histories
BEND_FUZZ_SECONDS=60 test/bend/run.sh      # longer parser fuzz budget per target
OUT=/tmp/syrius-bend-logs test/bend/run.sh # a fresh run-* log directory each run
```

The script sets `GOWORK=off GOTOOLCHAIN=auto`. The first run builds
`go-syrius-bend:2.0.40` and `go-syrius-bend:2.0.40-verdict`; subsequent builds use
Docker's layer cache. The second image contains Lean and is several GB.
Neither image is shipped in the wallet.

Bend's release archive is pinned to 2.0.40, with separate SHA-256 checks for Linux
arm64 and x64. The base image is pinned by digest. The verdict image pins and
checks the elan installer script and requests Lean 4.34.0; the downloaded elan
binary is selected by that installer, and Lean is version-pinned rather than
independently checksummed. These are toolchain trust assumptions, not an entirely
hermetic build. Upgrades require reviewing the pins/checksums together.

Checks run with no network, a non-root image user, a read-only filesystem and
model mount, dropped capabilities, no new privileges, and CPU-process/memory
limits. Only `test/bend` is mounted, never wallet data or host credentials.
Temporary generated traces and faulty copies live under its ignored `.scratch/`.
Go uses synthetic test wallets and temporary data directories; the new tests
never contact a live node or move funds.

| Setting | Default | Meaning |
|---|---|---|
| `BEND_DEPTH` | `3` | Every sequence of 1 through this many operations; accepts exactly `1`–`4` |
| `BEND_SEED` | `1` | Positive signed 64-bit seed for repeatable lifecycle/recovery histories |
| `BEND_RANDOM` | `40` | Additional histories per family, length 1–24; accepts `1`–`1000` |
| `BEND_FUZZ_SECONDS` | `10` | Go fuzz budget per target, two workers; accepts `1`–`300` |
| `OUT` | `test/bend/.scratch/out` | Parent for a unique run's logs |
| `VERDICT` | `1` | `0` is a partial developer check; prints **PARTIAL PASS**, never the full gate |
| `BEND_IMAGE` | `go-syrius-bend:2.0.40` | Explicit override reuses an existing image without rebuilding it |
| `BEND_VERDICT_IMAGE` | `go-syrius-bend:2.0.40-verdict` | Same override for the independent checker |

For debugging an already-built installation:

```bash
GOWORK=off GOTOOLCHAIN=auto go test -tags bendrules ./app -run '^TestBend' -count=1 -v
```

The build tag keeps Docker out of normal `go test ./...`. Missing tools, invalid
inputs, empty comparisons, stale fault targets, malformed output, crashes,
timeouts, and compiler errors fail the gate. They are not successful fault
detection. Each comparison test must appear as passed, and every declared law
must have a corresponding proof definition.

## What the gate checks

1. **Proofs:** every `PROOF.bend` passes Bend's checker.
2. **Independent verdict:** the same proofs pass Bend's separate kernel check.
3. **Model faults:** 19 type-correct model mutations must break named laws.
4. **Wallet comparisons:** Go calls real wallet methods, executes Bend on the
   corresponding inputs, and compares results. There is no substitute Go model.
5. **Wallet faults:** 19 changes to copies of real Go source must cause a named
   comparison failure. `go test -overlay` substitutes each copy during compilation;
   the working source files are never edited by these experiments.
6. **Parser fuzzing:** Go mutates journal JSON and bridge calldata/amount/token
   inputs, checking for crashes, loss during journal round trips, and accepted
   requests that violate effect/funding rules. These are additional executable
   properties, not Bend proofs. Go writes a reproducing corpus entry on failure.

See `run.py` for the exact faults. They remove effect checks for each of sender,
recipient, token, amount and data; bypass consent and journal persistence; loosen
decimal/auto-lock boundaries; overwrite a pending hold; cancel during publish;
forget an unresolved outcome; or admit a stale reconnect generation. The model
faults independently test the laws' sensitivity to comparable mistakes.

## Formal law catalog

There are **76 laws** across six small models. `LAWS.bend` is the reviewable
contract, `model.bend` is its implementation, and `PROOF.bend` supplies evidence.
Laws with `for` quantify over their typed model inputs; the explicit valid/invalid
constant cases are computed examples, not universal claims about the whole app.

| Family / laws | Contract | Real implementation and comparison |
|---|---|---|
| Effect (7): `effect_requires_*`, `effect_accepts_valid` | Any mismatch of sender, destination, token, amount presence, exact integer amount, or ABI data prevents acceptance; a matching effect succeeds | `assertMatches`; `TestBendEffect` checks all 64 predicate combinations using real field mutations, plus a missing expected amount. Amounts exceed floating-point precision |
| Publication (8): `publish_requires_*`, `publish_accepts_valid` | The identity, wallet session, effect, chain, consent, preparation and durable-record gates must all pass | `confirmPublishBlock`; `TestBendPublication` injects changes during preparation, persistence failure, competing cancel/confirm, and a repeated completed confirm. Broadcast callbacks inspect the saved signed record |
| Mainnet (3) | Chain 1 requires explicit consent; enabled mainnet and other chains pass this particular gate | `guardChain`; `TestBendPolicy`. This gate alone does not authorize unknown chains; the publication chain check is separate |
| Reconnect (7): `reconnect_requires_*`, `reconnect_accepts_valid` | Only the current generation in embedded mode with a running node, no wallet client, and no stop/wedge may attempt recovery | `retryEmbeddedConnection`; `TestBendReconnect` checks all 64 predicate combinations, with an invalid URL preventing external I/O |
| Bridge funding (6) | Wrap attaches positive value; Redeem attaches zero ZNN; other methods are refused by this funding model | `walletConnectValidateIntent`; `TestBendPolicy` compares Wrap/Redeem with negative, zero and positive amounts and ZNN/QSR. Privileged-method/envelope checks additionally have ordinary regression tests |
| Decimals (4) | Negative and above-18 decimals are refused; endpoints 0 and 18 are valid | `boundTokenDecimals`; `TestBendPolicy` covers -2 through 25 |
| Auto-lock (1) | Disabled timeout never expires, for any modeled elapsed duration | `autoLockExpired`; `TestBendPolicy` also compares all five presets around exact expiry boundaries. The strict boundary has a Go fault test; it is not an additional universal Bend theorem |
| Pending slot (10) | An occupied slot cannot be replaced; stale cancellation/cleanup cannot clear it; a publishing latch refuses matching/all cancellation; a successful cancel followed by hold allocates the next identity | `holdPending`, `CancelPending`, `clearPendingIf`; `TestBendPending` compares state after every operation and tests busy cancellation separately |
| Journal (10) | Acknowledging unresolved signed data fails without loss; publishing retains payload; only published outcomes can be removed; restart preserves state; repeated publish/ack behave as specified | `wcJournal.put/markPublished/deletePublished/get`; `TestBendJournal` reopens the actual disk journal and checks all retained fields and signed-block JSON content |
| Lifecycle (6) | Lock/account change invalidates every later modeled continuation, including unlock; revoked consent and locked state block publication | `TestBendLifecycle` drives real SDK preparation/signing against a loopback RPC server. Lock/unlock/consent changes occur at the last pre-signing RPC; account selection is represented by its generation increment. Both send and receive are compared |
| Input trust (6) | Metadata must exist, identify the requested token and have bounded decimals; corrupt storage cannot be treated as empty | `TestBendMetadataIdentity` exercises SDK token-response decoding; `TestBendCorruptJournal` checks 17 storage fixtures, rejected writes/acks and preservation of corrupt bytes |
| Public recovery (8) | Any modeled history without positive evidence retains uncertainty and the exact payload; revoked consent prevents rebroadcast, found evidence needs no consent, failed saves preserve duplicate protection | `TestBendRecovery` starts with a lost publication reply, then drives public reconcile/ack/lookup and service restart against the disk journal, with node/signing seams and injected save failures |

The policy model's Boolean inputs mean predicates on real values (for example,
“built sender equals approved sender”), not Booleans supplied by the WebView.
The adapter varies those actual Go values. Predicate truth tables are exhaustive;
this does not exhaust the space of all addresses, ABI encodings, or integer values.

### Exhaustive sequence bounds

The original exhaustive/fixed families compare **683 observations** at depth 3:

- 65 effect cases, 122 policy/funding/expiry cases, 16 publication cases, and
  64 reconnect eligibility cases.
- Every one of 258 pending sequences over six operations: place, cancel current,
  cancel stale, cancel all, cleanup current, cleanup stale; plus three busy cases.
- Every one of 155 journal sequences over five operations: write payload 1,
  write payload 2, mark published, acknowledge, restart.

Each sequence starts fresh; the comparison includes every intermediate outcome
and state, not merely the final balance/status. At depth 4 there are 1,554 pending
and 780 journal sequences, for **2,604 observations** in those families. No sequence within
these alphabets/bounds is sampled or skipped. Longer sequences and different
operations remain outside this exhaustive check. Fixtures for in-flight
publication/cancellation are separate interleavings, not all possible schedules.

The added families contribute **191 comparisons** with the default random budget:
98 lifecycle histories (9 fixed + 40 seeded histories for each of send/receive),
30 metadata cases, 17 corrupt/valid storage cases, and 46 recovery histories
(6 fixed + 40 seeded). Recovery traces compare every intermediate return value,
broadcast count, stored state, lookup outcome and retained block content.
Totals are **874** at depth 3 and **2,795** at depth 4 with `BEND_RANDOM=40`.
Random histories reach 24 steps independently of `BEND_DEPTH`; increasing depth
does not automatically expand that sample. The lifecycle and recovery proofs
include induction over arbitrary model lists, while Go execution remains bounded.

### Real defect evidence

The expanded checks failed against the original production code before fixes.
They found receive publication after lock/account-generation/consent changes,
acceptance of another token's metadata, and acceptance of malformed journal
records. Minimal failures and fixes are recorded in
[the discovery report](bend-discovery-2026-11-20.md).

A real comparison failure prints `BEND_MISMATCH`, the case/history and seed,
and `wallet=... model=...`, followed by the named `--- FAIL: TestBend...` and a
nonzero gate exit. A proof failure prints the failed law. `caught ... fault`
messages are expected failures from deliberate mutations, not defects found in
the current production source. Repeat a failing seeded run with the printed
`BEND_SEED` and `BEND_RANDOM`; the run's `00-settings.log` retains both.

## Wallet-wide contract coverage

Formal models intentionally stay small. These additional wallet requirements
remain part of the comprehensive contract, with executable evidence in the
ordinary Go/frontend CI jobs. They must not be advertised as Bend-proven:

| Required invariant | Existing executable evidence / remaining boundary |
|---|---|
| No private keys, seeds, or decrypted keystores cross Wails; mnemonic exposure stays on explicit lifecycle/reveal paths | Backend-only signing architecture, DTO/binding review, wallet lifecycle/reveal/entropy tests in `app/wallet_service_test.go`; no information-flow proof of the WebView boundary |
| Keystores and derived addresses remain syrius-compatible | `internal/compat/keystore_compat_test.go`, wallet round trips and frozen entropy vectors; real-file acceptance is skipped when the private fixture is absent |
| Confirmation displays exact amount and decoded built effect | `app/tx_effect_test.go`, `app/tx_methods_test.go`, strict custom-token decimal tests, confirmation component tests; frontend `format.test.ts` round-trips 247 boundary/large-value cases across decimals 0–18 and rejects extra nonzero precision |
| Untrusted requests are revalidated in Go, including NoM calls | `app/nom_service_test.go`, `app/nom_accelerator_test.go`, bridge envelope/ABI/method tests in `app/walletconnect_test.go`; malformed formats and all ABI space are not exhaustively modeled |
| Lock/account switch invalidates signing; inactivity lock is backend-enforced | `app/tx_service_test.go`, `app/wallet_autolock_test.go`, selection/lock race tests; publication fixtures also change generation during signing |
| Ambiguous network failures retain a signed block; retries reconcile or rebroadcast that block | `app/walletconnect_journal_test.go` persistence-before-broadcast, exact rebroadcast, intent matching across request IDs/topics, full-journal refusal, chain gates and replay without live wallet access |
| Stale node workers cannot replace a newer connection or stop the syncing node | `app/node_service_reconnect_test.go` blocked-recovery interleavings, shutdown supersession and locked-wallet recovery; `app/node_service_autoreceive_lifecycle_test.go` worker lifecycle |
| Old account/connection responses cannot overwrite current UI state | Store/router tests, including balances, unreceived, wallet, and node; successful empty responses stay distinct from unavailable data |
| Hostile node pagination and metadata cannot grow work without bounds | `app/paging_test.go`, decimal and timestamp-bound tests; the model does not prove memory allocation or resource exhaustion bounds |
| Crypto/signature/PoW/ABI compatibility and live write acceptance | SDK smoke and integration tests; live testnet acceptance and independent crypto review remain required. Bend does not replace them |
| Dependencies and releases stay reviewed/pinned | Existing `security`, build, race and frontend CI jobs; release signing/notarization remains a separate project milestone |

## Limits reviewers must retain

A pass proves claims about Bend models and demonstrates agreement on the tested
Go cases. It is not a proof of Go, TypeScript, the SDK, cryptography, the network,
filesystem durability under power loss, or all concurrent schedules. The Bend
checker/kernel/toolchain and the comparison adapter are trusted components.

Pending IDs use unbounded model naturals; actual `uint64` counter overflow is
outside the model. Elapsed times are nonnegative durations, not wall-clock
arithmetic. The reconnect model is the eligibility decision, not all connection
installation behavior. The publication model is a gate, not a liveness proof or
a claim that a network error means rejection.

The journal model covers one key and successful disk operations. Its private
`Write` operation can replace an existing entry, matching `put`; it makes no
claim that any arbitrary caller may safely replay or overwrite a request.
Cross-key duplicate prevention and capacity have separate regression tests.
The added recovery model covers one already-journaled request and selected read,
broadcast and save failures. The input model assumes classification into missing,
valid and corrupt; concrete JSON fixtures and fuzzing test the Go classification.
Validation checks record schema, ownership and saved-hash consistency; it is not
a cryptographic integrity check or proof of all possible corruption patterns.
JSON formatting may change on disk; retained signed-block content must not.

The lifecycle checkpoint exercises one specific ordering during RPC preparation
without sleeps. It does not prove every thread interleaving or cancel SDK signing
already underway; the fix blocks stale publication at the final boundary. Go
fuzzing has separate randomness/corpus from `BEND_SEED`; its time budget is a
search limit, not a coverage guarantee.

## GitHub Actions and maintenance

`.github/workflows/bend.yml` runs the full gate on PRs to `main` and pushes to
`main`, without path filters, plus manual depth/seed/history/fuzz-budget inputs.
PR/push runs use the GitHub run number as a logged seed, so new runs explore new
samples. It also runs race checks on the added Bend comparison families. It uses read-only
repository permission, non-persisted checkout credentials and commit-pinned
actions; it requires no secrets. Failure logs appear in grouped runner output.
Logs and any failing fuzz corpus are also kept as a seven-day failure artifact.
The workflow becomes active when this local change is pushed. Repository branch
protection is a separate setting; require **Bend laws** there if it should block
merging, alongside the existing required jobs.

When behavior changes: update the law catalog, model and real-code adapter
together; add a fault that demonstrates the new protection; run the gate and
ordinary affected tests. Review a changed law as a contract change. Do not weaken
claims or broaden success classification merely to obtain a passing build.
