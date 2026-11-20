import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.unresolved_ack_refused(payload):
  {==}

def Laws.published_ack_removes(payload):
  {==}

def Laws.absent_ack_idempotent():
  {==}

def Laws.missing_cannot_be_published():
  {==}

def Laws.publication_preserves_payload(payload):
  {==}

def Laws.publication_idempotent(payload):
  {==}

def Laws.restart_preserves_every_state(s):
  match s:
    case M.Absent{}:
      {==}
    case M.Signed{old}:
      {==}
    case M.Published{old}:
      {==}

def Laws.write_retains_signed_block(s, payload):
  match s:
    case M.Absent{}:
      {==}
    case M.Signed{old}:
      {==}
    case M.Published{old}:
      {==}

def Laws.unknown_survives_ack_and_restart(payload):
  {==}

def Laws.confirmed_result_can_be_acknowledged(payload):
  {==}
