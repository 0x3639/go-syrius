import Base

# One journal key. payload identifies the ENTIRE signed block + intent and
# ownership, not merely amount or hash. Tests compare the full retained record.
# Write is the private put operation, not permission to replay a request.
type State is Data:
  Absent{}
  Signed{payload: Nat}
  Published{payload: Nat}
type Op is Data:
  Write{payload: Nat}
  Publish{}
  Ack{}
  Restart{}
type Reply is Data:
  Reply{ok: Bool, state: State}

def step(s: State, op: Op) -> Reply:
  match s op:
    case _ Write{payload}:
      Reply{True{}, Signed{payload}}
    case Absent{} Publish{}:
      Reply{False{}, Absent{}}
    case Signed{payload} Publish{}:
      Reply{True{}, Published{payload}}
    case Published{payload} Publish{}:
      Reply{True{}, Published{payload}}
    case Absent{} Ack{}:
      Reply{True{}, Absent{}}
    case Signed{payload} Ack{}:
      Reply{False{}, Signed{payload}}
    case Published{payload} Ack{}:
      Reply{True{}, Absent{}}
    case _ Restart{}:
      Reply{True{}, s}

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
    case Absent{}:
      "absent"
    case Signed{payload}:
      "signed:" ++ Nat.show(payload)
    case Published{payload}:
      "published:" ++ Nat.show(payload)

def show_reply(r: Reply) -> String:
  match r:
    case Reply{ok, s}:
      bit(ok) ++ ":" ++ show(s)

def run(ops: +List<Op>, s: State) -> State:
  match ops:
    case Nil{}:
      s
    case Con{op, tail}:
      run(tail, state(step(s, op)))

def trace(ops: +List<Op>, +s: State) -> String:
  match ops:
    case Nil{}:
      ""
    case Con{+op, tail}:
      show_reply(step(s, op)) ++ ";" ++ trace(tail, state(step(s, op)))
