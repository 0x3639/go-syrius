import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.uncertainty_survives_every_unproven_history(ops, payload, consent):
  match ops consent:
    case Nil{} _:
      {==}
    case Con{M.ReadError{}, tail} _:
      Laws.uncertainty_survives_every_unproven_history(tail, payload, consent)
    case Con{M.Acknowledge{}, tail} _:
      Laws.uncertainty_survives_every_unproven_history(tail, payload, consent)
    case Con{M.Reopen{}, tail} _:
      Laws.uncertainty_survives_every_unproven_history(tail, payload, consent)
    case Con{M.LostReply{}, tail} False{}:
      Laws.uncertainty_survives_every_unproven_history(tail, payload, False{})
    case Con{M.LostReply{}, tail} True{}:
      Laws.uncertainty_survives_every_unproven_history(tail, payload, True{})

def Laws.acknowledgement_needs_evidence(payload, consent):
  {==}

def Laws.found_block_needs_no_consent_or_broadcast(payload):
  {==}

def Laws.revoked_consent_blocks_rebroadcast(payload):
  {==}

def Laws.lost_reply_retains_exact_payload(payload):
  {==}

def Laws.accepted_rebroadcast_resolves_exact_payload(payload):
  {==}

def Laws.failed_save_keeps_duplicate_protection(payload):
  {==}

def Laws.published_replay_never_rebroadcasts(payload, consent):
  {==}
