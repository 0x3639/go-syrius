# Bend discovery and focused fixes — 2026-11-20

The expanded checks exposed three defects in production code based on
`8d4704633480c9ea4290546c34d9ebba4e3a25be`. The checks were run before applying
the fixes; their failures were not deliberate mutations. The original local
output is retained at `test/bend/.scratch/discovery-before.log` (ignored).
Selected observations are recorded below so the evidence survives cleanup.

## 1. Receive continued publication after wallet state changed

`Receive` called the SDK's combined prepare/sign/publish operation after its
initial checks. The SDK retained the signing key while doing RPC/PoW work.
Locking the wallet, changing account generation, or revoking mainnet consent
during that work did not prevent the eventual broadcast.

`TestBendLifecycle` uses the actual SDK with a loopback HTTP JSON-RPC server.
The server triggers lifecycle changes at the last pre-signing RPC and records
publication requests. No live node or real funds are involved. The minimal
before-fix failures were:

```text
BEND_MISMATCH receive seed=1 case=1 history=[0]: wallet="1" model="0"
BEND_MISMATCH receive seed=1 case=2 history=[3]: wallet="1" model="0"
BEND_MISMATCH receive seed=1 case=3 history=[0 1]: wallet="1" model="0"
BEND_MISMATCH receive seed=1 case=4 history=[2]: wallet="1" model="0"
--- FAIL: TestBendLifecycle
```

The original numeric events were `0=Lock`, `1=Unlock`, `2=account generation
change`, `3=revoke consent`. Failures now print event names. Account change is
represented by incrementing the wallet generation, rather than the entire
account-selection persistence workflow. The equivalent send cases passed.

**Fix:** `app/tx_service.go` separates SDK preparation from receive publication.
It captures the wallet generation and connection/chain together, then checks
generation, active sender, connection, chain and consent after preparation and
before publication. These histories now produce no broadcast. This does not
interrupt SDK signing already in progress or prove all possible interleavings.

## 2. Token decimals could belong to another token

`clientTokenDecimals` rejected missing/zero-token metadata and out-of-range
decimals, but accepted another valid token's metadata. A faulty node could reply
to a custom-token lookup with ZNN/QSR metadata and cause the strict confirmation
path to use an unrelated decimal scale.

`TestBendMetadataIdentity` exercises the real SDK response decoder with 30
token/decimal combinations. The eight mismatches were ZNN and QSR returned for
a different requested token, at decimals 0, 6, 8 and 18. One example:

```text
BEND_MISMATCH metadata-zts1znnxxxxxxxxxxxxx9z4ulx-6: wallet="1" model="0"
--- FAIL: TestBendMetadataIdentity
```

**Fix:** `app/decimals.go` requires the returned token standard to equal the
requested standard before accepting its decimals. Strict callers return the
error; existing display-only fallback behavior is unchanged. This cannot prove
that a node reports honest decimals for the correct token.

## 3. Malformed publication journals were accepted

The loader used JSON unmarshalling without validating record shape/state.
`null` decoded successfully into a nil map, null/empty records became zero
values, and unknown publication states were accepted. In particular, treating
`null` as an empty journal discarded knowledge needed for replay protection;
later assignment to that nil map could panic.

```text
BEND_MISMATCH null-document: wallet="1" model="0"
BEND_MISMATCH null-record: wallet="1" model="0"
BEND_MISMATCH empty-record: wallet="1" model="0"
BEND_MISMATCH unknown-state: wallet="1" model="0"
--- FAIL: TestBendCorruptJournal
```

**Fix:** `app/walletconnect_journal.go` requires an object with unique canonical
request keys, bounded record count, consistent ownership, a recognized state,
nonempty intent/hash, and a retained block whose saved hash agrees with the
record. A missing file and an empty object remain valid. Legacy records may
derive omitted ownership from their key; conflicting ownership is refused.
Validation also precedes new journal writes.

The expanded 17-case regression checks prove rejected writes/acknowledgements
leave the original corrupt file untouched. The fix does not silently delete or
repair records. It validates schema and consistency, not cryptographic integrity
or every possible corruption pattern. Invalid existing records now require
investigation instead of being treated as fresh requests.

## Expanded discovery and validation

The suite now has 76 laws, including induction over arbitrary model histories
for stale work and unresolved outcomes. The Go adapters add seeded histories of
up to 24 operations, exact recovery-payload checks, failed-save/restart cases,
and malformed storage/metadata inputs. Two bounded Go fuzz targets explore
journal JSON and bridge request decoding. All seeds are synthetic public data.

The default gate passed both proof checkers, 874 comparisons, all 19 model
mutations and 19 Go overlay mutations, and both 10-second fuzz searches. The
first searches executed 140,218 journal cases and 267,635 bridge cases without
another failure; these counts are observations from one run, not guarantees.
The full Go tests, Go vet, frontend type checking and 494 frontend tests passed.
The four added Bend comparison families also passed the race detector.
The deeper gate also passed: depth 4, seed 42, 200 random histories per family,
3,275 comparisons, all 38 deliberate faults, and 30 seconds of fuzzing per target.
The full-project Go build passed as well.

```bash
test/bend/run.sh
BEND_DEPTH=4 BEND_SEED=42 BEND_RANDOM=200 BEND_FUZZ_SECONDS=30 test/bend/run.sh
GOWORK=off GOTOOLCHAIN=auto go test -race -tags bendrules ./app \
  -run '^TestBend(Lifecycle|MetadataIdentity|CorruptJournal|Recovery)$' -count=1
```

The GitHub workflow runs the same gate and targeted race checks. PR/push runs
use the run number as the logged history seed; manual runs accept explicit
depth, seed, history count and fuzz budget. Failure logs and reproducing fuzz
inputs are retained as a seven-day artifact. A local pass is not a hosted CI run:
the workflow takes effect after these changes are pushed.

The deliberate fault checks show that the gate detects the tested mistakes.
They do not establish that all defects have been found. See
[BEND_LAWS.md](BEND_LAWS.md) for the complete contract and coverage limits.
