import Base

# Boolean inputs are predicates on the real values, not trusted frontend flags.
# app/tx_service.go assertMatches: sender, destination, token, nonnil amounts,
# exact integer amount, exact ABI bytes. The Go adapter varies the real fields.
def effect(sender: Bool, destination: Bool, token: Bool, present: Bool, amount: Bool, data: Bool) -> Bool:
  sender && (destination && (token && (present && (amount && data))))

# confirmPublishBlock, from preview ownership through the final journal write.
# Network success is NOT modeled as definite publication on a broadcast error.
def publish(identity: Bool, session: Bool, effect: Bool, chain: Bool, consent: Bool, prepared: Bool, durable: Bool) -> Bool:
  identity && (session && (effect && (chain && (consent && (prepared && durable)))))

# guardChain: the caller must also enforce the connected-chain match.
def mainnet(is_mainnet: Bool, consent: Bool) -> Bool:
  Bool.not(is_mainnet) || consent

# retryEmbeddedConnection eligibility, before reserving the new generation.
def reconnect(current: Bool, embedded: Bool, running: Bool, offline: Bool, not_stopping: Bool, not_wedged: Bool) -> Bool:
  current && (embedded && (running && (offline && (not_stopping && not_wedged))))

# walletConnectValidateIntent funding, AFTER envelope/ABI validation.
type Method is Data:
  Wrap{}
  Redeem{}
  Other{}

def funding(method: Method, positive: Bool, zero: Bool, znn: Bool) -> Bool:
  match method:
    case Wrap{}:
      positive
    case Redeem{}:
      zero && znn
    case Other{}:
      False{}

# boundTokenDecimals. Negative Go ints are represented separately from Nat.
def decimals(negative: Bool, value: Nat) -> Bool:
  Bool.not(negative) && Nat.is_le(value, 18n)

# autoLockExpired. Time is an elapsed nonnegative duration in one common unit.
# The real adapter uses seconds, including the exact timeout boundary.
def expired(limit: Nat, elapsed: Nat) -> Bool:
  match limit:
    case 0n:
      False{}
    case 1n+p:
      Nat.is_lt(1n+p, elapsed)

def mark(b: Bool) -> String:
  match b:
    case False{}:
      "0"
    case True{}:
      "1"
