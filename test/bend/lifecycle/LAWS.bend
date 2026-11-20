import Base
import ./model.bend as M

# All later modeled activity, of any length, cannot revive old signing work.
law stale_work_never_recovers:
  for +events: +List<M.LifeEvent>
  for +live: Bool
  for +consent: Bool
  {M.permit(M.run(events, M.State{live, consent, False{}})) == False{} : Bool}

law lock_invalidates_every_continuation:
  for +events: +List<M.LifeEvent>
  for +s: M.State
  {M.permit(M.run(events, M.apply(s, M.Lock{}))) == False{} : Bool}

law account_change_invalidates_every_continuation:
  for +events: +List<M.LifeEvent>
  for +s: M.State
  {M.permit(M.run(events, M.apply(s, M.Switch{}))) == False{} : Bool}

law revoked_consent_blocks_publication:
  for +live: Bool
  for +fresh: Bool
  {M.permit(M.State{live, False{}, fresh}) == False{} : Bool}

law locked_wallet_cannot_publish:
  for +consent: Bool
  for +fresh: Bool
  {M.permit(M.State{False{}, consent, fresh}) == False{} : Bool}

law valid_work_can_publish:
  {M.permit(M.State{True{}, True{}, True{}}) == True{} : Bool}
