import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.stale_work_never_recovers(events, live, consent):
  match events:
    case Nil{}:
      {==}
    case Con{M.Lock{}, tail}:
      Laws.stale_work_never_recovers(tail, False{}, consent)
    case Con{M.Unlock{}, tail}:
      Laws.stale_work_never_recovers(tail, True{}, consent)
    case Con{M.Switch{}, tail}:
      Laws.stale_work_never_recovers(tail, live, consent)
    case Con{M.Revoke{}, tail}:
      Laws.stale_work_never_recovers(tail, live, False{})
    case Con{M.Grant{}, tail}:
      Laws.stale_work_never_recovers(tail, live, True{})
    case Con{M.IdleTick{}, tail}:
      Laws.stale_work_never_recovers(tail, live, consent)

def Laws.lock_invalidates_every_continuation(events, s):
  match s:
    case M.State{live, consent, fresh}:
      Laws.stale_work_never_recovers(events, False{}, consent)

def Laws.account_change_invalidates_every_continuation(events, s):
  match s:
    case M.State{live, consent, fresh}:
      Laws.stale_work_never_recovers(events, live, consent)

def Laws.revoked_consent_blocks_publication(live, fresh):
  match live fresh:
    case False{} False{}:
      {==}
    case False{} True{}:
      {==}
    case True{} False{}:
      {==}
    case True{} True{}:
      {==}

def Laws.locked_wallet_cannot_publish(consent, fresh):
  match fresh:
    case False{}:
      {==}
    case True{}:
      {==}

def Laws.valid_work_can_publish():
  {==}
