import Base
import ./model.bend as M

law unresolved_ack_refused:
  for +payload: Nat
  {M.step(M.Signed{payload}, M.Ack{}) == M.Reply{False{}, M.Signed{payload}} : M.Reply}

law published_ack_removes:
  for +payload: Nat
  {M.step(M.Published{payload}, M.Ack{}) == M.Reply{True{}, M.Absent{}} : M.Reply}

law absent_ack_idempotent:
  {M.step(M.Absent{}, M.Ack{}) == M.Reply{True{}, M.Absent{}} : M.Reply}

law missing_cannot_be_published:
  {M.step(M.Absent{}, M.Publish{}) == M.Reply{False{}, M.Absent{}} : M.Reply}

law publication_preserves_payload:
  for +payload: Nat
  {M.step(M.Signed{payload}, M.Publish{}) == M.Reply{True{}, M.Published{payload}} : M.Reply}

law publication_idempotent:
  for +payload: Nat
  {M.step(M.Published{payload}, M.Publish{}) == M.Reply{True{}, M.Published{payload}} : M.Reply}

law restart_preserves_every_state:
  for +s: M.State
  {M.step(s, M.Restart{}) == M.Reply{True{}, s} : M.Reply}

law write_retains_signed_block:
  for +s: M.State
  for +payload: Nat
  {M.step(s, M.Write{payload}) == M.Reply{True{}, M.Signed{payload}} : M.Reply}

law unknown_survives_ack_and_restart:
  for +payload: Nat
  {M.run([M.Ack{}, M.Restart{}, M.Ack{}], M.Signed{payload}) == M.Signed{payload} : M.State}

law confirmed_result_can_be_acknowledged:
  for +payload: Nat
  {M.run([M.Publish{}, M.Restart{}, M.Ack{}], M.Signed{payload}) == M.Absent{} : M.State}
