import Base
import ./model.bend as M

law empty_slot_accepts:
  for +n: Nat
  for +busy: Bool
  {M.hold(M.State{n, False{}, busy}) == M.Reply{True{}, M.State{1n+n, True{}, busy}} : M.Reply}

law occupied_slot_never_replaced:
  for +n: Nat
  for +busy: Bool
  {M.hold(M.State{n, True{}, busy}) == M.Reply{False{}, M.State{n, True{}, busy}} : M.Reply}

law stale_cancel_preserves_slot:
  for +s: M.State
  {M.cancel(s, False{}, False{}) == M.Reply{True{}, s} : M.Reply}

law publishing_cancel_refused:
  for +n: Nat
  for +held: Bool
  {M.cancel(M.State{n, held, True{}}, False{}, True{}) == M.Reply{False{}, M.State{n, held, True{}}} : M.Reply}

law publishing_cancel_all_refused:
  for +n: Nat
  for +held: Bool
  for +matches: Bool
  {M.cancel(M.State{n, held, True{}}, True{}, matches) == M.Reply{False{}, M.State{n, held, True{}}} : M.Reply}

law matching_cancel_clears:
  for +n: Nat
  for +held: Bool
  {M.cancel(M.State{n, held, False{}}, False{}, True{}) == M.Reply{True{}, M.State{n, False{}, False{}}} : M.Reply}

law cancel_all_clears:
  for +n: Nat
  for +held: Bool
  for +matches: Bool
  {M.cancel(M.State{n, held, False{}}, True{}, matches) == M.Reply{True{}, M.State{n, False{}, False{}}} : M.Reply}

law stale_cleanup_preserves_slot:
  for +n: Nat
  for +held: Bool
  for +busy: Bool
  {M.clear(M.State{n, held, busy}, False{}) == M.State{n, held, busy} : M.State}

law matching_cleanup_keeps_counter_and_latch:
  for +n: Nat
  for +held: Bool
  for +busy: Bool
  {M.clear(M.State{n, held, busy}, True{}) == M.State{n, False{}, busy} : M.State}

law cancel_then_hold_uses_fresh_identity:
  for +n: Nat
  {M.hold(M.state(M.cancel(M.State{n, True{}, False{}}, False{}, True{}))) == M.Reply{True{}, M.State{1n+n, True{}, False{}}} : M.Reply}
