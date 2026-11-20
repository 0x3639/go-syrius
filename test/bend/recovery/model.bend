import Base

# Public recovery protocol for one request, starting after a lost broadcast
# reply. Only positive evidence resolves it. Retained payloads cannot be rebuilt.
type Stage is Data:
  Missing{}
  Uncertain{payload: Nat}
  Known{payload: Nat}
type State is Data:
  State{stage: Stage, consent: Bool}
type Op is Data:
  QueryError{}
  Found{}
  RetryAccepted{}
  RetryLost{}
  FoundSaveError{}
  RetrySaveError{}
  Ack{}
  Restart{}
  Revoke{}
  Grant{}
type Reply is Data:
  Reply{ok: Bool, sent: Bool, state: State}

def retry(consent: Bool, payload: Nat, accepted: Bool, saved: Bool) -> Reply:
  match consent accepted saved:
    case False{} _ _:
      Reply{False{}, False{}, State{Uncertain{payload}, False{}}}
    case True{} False{} _:
      Reply{False{}, True{}, State{Uncertain{payload}, True{}}}
    case True{} True{} False{}:
      Reply{True{}, True{}, State{Uncertain{payload}, True{}}}
    case True{} True{} True{}:
      Reply{True{}, True{}, State{Known{payload}, True{}}}

def query(stage: Stage, consent: Bool, op: Op) -> Reply:
  match stage op:
    case Missing{} _:
      Reply{False{}, False{}, State{Missing{}, consent}}
    case Known{payload} _:
      Reply{True{}, False{}, State{Known{payload}, consent}}
    case Uncertain{payload} Found{}:
      Reply{True{}, False{}, State{Known{payload}, consent}}
    case Uncertain{payload} FoundSaveError{}:
      Reply{True{}, False{}, State{Uncertain{payload}, consent}}
    case Uncertain{payload} RetryAccepted{}:
      retry(consent, payload, True{}, True{})
    case Uncertain{payload} RetrySaveError{}:
      retry(consent, payload, True{}, False{})
    case Uncertain{payload} RetryLost{}:
      retry(consent, payload, False{}, True{})
    case Uncertain{payload} _:
      Reply{False{}, False{}, State{Uncertain{payload}, consent}}

def ack(stage: Stage, consent: Bool) -> Reply:
  match stage:
    case Uncertain{payload}:
      Reply{False{}, False{}, State{Uncertain{payload}, consent}}
    case _:
      Reply{True{}, False{}, State{Missing{}, consent}}

def step(s: State, op: Op) -> Reply:
  match s op:
    case State{stage, consent} Ack{}:
      ack(stage, consent)
    case State{stage, consent} Restart{}:
      Reply{True{}, False{}, State{stage, consent}}
    case State{stage, consent} Revoke{}:
      Reply{True{}, False{}, State{stage, False{}}}
    case State{stage, consent} Grant{}:
      Reply{True{}, False{}, State{stage, True{}}}
    case State{stage, consent} _:
      query(stage, consent, op)

def state(r: Reply) -> State:
  match r:
    case Reply{ok, sent, s}:
      s

def bit(b: Bool) -> String:
  match b:
    case False{}:
      "0"
    case True{}:
      "1"

def show_stage(s: Stage) -> String:
  match s:
    case Missing{}:
      "none"
    case Uncertain{payload}:
      "unknown:" ++ Nat.show(payload)
    case Known{payload}:
      "published:" ++ Nat.show(payload)

def show(r: Reply) -> String:
  match r:
    case Reply{ok, sent, State{stage, consent}}:
      bit(ok) ++ ":" ++ bit(sent) ++ ":" ++ show_stage(stage) ++ ";"

def trace(ops: +List<Op>, +s: State) -> String:
  match ops:
    case Nil{}:
      ""
    case Con{+op, tail}:
      show(step(s, op)) ++ trace(tail, state(step(s, op)))

# Histories with no positive publication evidence must retain uncertainty,
# however long they run and however many acknowledgements/restarts intervene.
type Unproven is Data:
  ReadError{}
  LostReply{}
  Acknowledge{}
  Reopen{}

def lift(op: Unproven) -> Op:
  match op:
    case ReadError{}:
      QueryError{}
    case LostReply{}:
      RetryLost{}
    case Acknowledge{}:
      Ack{}
    case Reopen{}:
      Restart{}

def unresolved(ops: +List<Unproven>, s: State) -> State:
  match ops:
    case Nil{}:
      s
    case Con{op, tail}:
      unresolved(tail, state(step(s, lift(op))))
