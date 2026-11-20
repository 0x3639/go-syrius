import Base

# TxService's single pending slot. counter abstracts uint64 without overflow.
# target is derived from nonzero hold ID equality by the Go adapter.
# busy represents the publishingHold latch; it is cleared by publish's defer,
# independently of clearing the slot. No scheduling/fairness claim is made.
type State is Data:
  State{counter: Nat, held: Bool, busy: Bool}
type Reply is Data:
  Reply{ok: Bool, state: State}

def hold(s: State) -> Reply:
  match s:
    case State{counter, False{}, busy}:
      Reply{True{}, State{1n+counter, True{}, busy}}
    case State{counter, True{}, busy}:
      Reply{False{}, State{counter, True{}, busy}}

def cancel_target(target: Bool, s: State) -> Reply:
  match target s:
    case False{} _:
      Reply{True{}, s}
    case True{} State{counter, held, True{}}:
      Reply{False{}, State{counter, held, True{}}}
    case True{} State{counter, held, False{}}:
      Reply{True{}, State{counter, False{}, False{}}}

def cancel(s: State, all: Bool, matches: Bool) -> Reply:
  cancel_target(all || matches, s)

# clearPendingIf is internal cleanup, not user cancellation. It may clear a
# publishing hold; it never clears a newer one and does not release the latch.
def clear(s: State, matches: Bool) -> State:
  match s matches:
    case State{counter, held, busy} False{}:
      State{counter, held, busy}
    case State{counter, held, busy} True{}:
      State{counter, False{}, busy}

def state(r: Reply) -> State:
  match r:
    case Reply{ok, s}:
      s

def bit(b: Bool) -> String:
  match b:
    case False{}:
      "0"
    case True{}:
      "1"

def show(s: State) -> String:
  match s:
    case State{counter, held, busy}:
      Nat.show(counter) ++ ":" ++ bit(held) ++ ":" ++ bit(busy)

def show_reply(r: Reply) -> String:
  match r:
    case Reply{ok, s}:
      bit(ok) ++ ":" ++ show(s)

type Op is Data:
  Place{}
  CancelCurrent{}
  CancelStale{}
  CancelAll{}
  ClearCurrent{}
  ClearStale{}

def step(s: State, op: Op) -> Reply:
  match op:
    case Place{}:
      hold(s)
    case CancelCurrent{}:
      cancel(s, False{}, True{})
    case CancelStale{}:
      cancel(s, False{}, False{})
    case CancelAll{}:
      cancel(s, True{}, False{})
    case ClearCurrent{}:
      Reply{True{}, clear(s, True{})}
    case ClearStale{}:
      Reply{True{}, clear(s, False{})}

def trace(ops: +List<Op>, +s: State) -> String:
  match ops:
    case Nil{}:
      ""
    case Con{+op, tail}:
      show_reply(step(s, op)) ++ ";" ++ trace(tail, state(step(s, op)))
