import Base

# Requirement, independent of the implementation: work started under an old
# wallet session must not publish after lock/account change, even if unlocked
# again. Consent is checked at the final publication boundary.
# The trace begins after a request has captured a valid signing context.
type State is Data:
  State{live: Bool, consent: Bool, fresh: Bool}
type LifeEvent is Data:
  Lock{}
  Unlock{}
  Switch{}
  Revoke{}
  Grant{}
  IdleTick{}

def apply(s: State, e: LifeEvent) -> State:
  match s e:
    case State{live, consent, fresh} Lock{}:
      State{False{}, consent, False{}}
    case State{live, consent, fresh} Unlock{}:
      State{True{}, consent, fresh}
    case State{live, consent, fresh} Switch{}:
      State{live, consent, False{}}
    case State{live, consent, fresh} Revoke{}:
      State{live, False{}, fresh}
    case State{live, consent, fresh} Grant{}:
      State{live, True{}, fresh}
    case _ IdleTick{}:
      s

def run(events: +List<LifeEvent>, s: State) -> State:
  match events:
    case Nil{}:
      s
    case Con{event, tail}:
      run(tail, apply(s, event))

def permit(s: State) -> Bool:
  match s:
    case State{live, consent, fresh}:
      fresh && (live && consent)
