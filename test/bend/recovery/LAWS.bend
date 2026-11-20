import Base
import ./model.bend as M

law uncertainty_survives_every_unproven_history:
  for +ops: +List<M.Unproven>
  for +payload: Nat
  for +consent: Bool
  {M.unresolved(ops, M.State{M.Uncertain{payload}, consent}) == M.State{M.Uncertain{payload}, consent} : M.State}

law acknowledgement_needs_evidence:
  for +payload: Nat
  for +consent: Bool
  {M.step(M.State{M.Uncertain{payload}, consent}, M.Ack{}) == M.Reply{False{}, False{}, M.State{M.Uncertain{payload}, consent}} : M.Reply}

law found_block_needs_no_consent_or_broadcast:
  for +payload: Nat
  {M.step(M.State{M.Uncertain{payload}, False{}}, M.Found{}) == M.Reply{True{}, False{}, M.State{M.Known{payload}, False{}}} : M.Reply}

law revoked_consent_blocks_rebroadcast:
  for +payload: Nat
  {M.step(M.State{M.Uncertain{payload}, False{}}, M.RetryAccepted{}) == M.Reply{False{}, False{}, M.State{M.Uncertain{payload}, False{}}} : M.Reply}

law lost_reply_retains_exact_payload:
  for +payload: Nat
  {M.step(M.State{M.Uncertain{payload}, True{}}, M.RetryLost{}) == M.Reply{False{}, True{}, M.State{M.Uncertain{payload}, True{}}} : M.Reply}

law accepted_rebroadcast_resolves_exact_payload:
  for +payload: Nat
  {M.step(M.State{M.Uncertain{payload}, True{}}, M.RetryAccepted{}) == M.Reply{True{}, True{}, M.State{M.Known{payload}, True{}}} : M.Reply}

law failed_save_keeps_duplicate_protection:
  for +payload: Nat
  {M.step(M.State{M.Uncertain{payload}, True{}}, M.RetrySaveError{}) == M.Reply{True{}, True{}, M.State{M.Uncertain{payload}, True{}}} : M.Reply}

law published_replay_never_rebroadcasts:
  for +payload: Nat
  for +consent: Bool
  {M.step(M.State{M.Known{payload}, consent}, M.RetryAccepted{}) == M.Reply{True{}, False{}, M.State{M.Known{payload}, consent}} : M.Reply}
