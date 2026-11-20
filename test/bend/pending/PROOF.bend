import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.empty_slot_accepts(n, busy):
  {==}

def Laws.occupied_slot_never_replaced(n, busy):
  {==}

def Laws.stale_cancel_preserves_slot(s):
  {==}

def Laws.publishing_cancel_refused(n, held):
  {==}

def Laws.publishing_cancel_all_refused(n, held, matches):
  {==}

def Laws.matching_cancel_clears(n, held):
  {==}

def Laws.cancel_all_clears(n, held, matches):
  {==}

def Laws.stale_cleanup_preserves_slot(n, held, busy):
  {==}

def Laws.matching_cleanup_keeps_counter_and_latch(n, held, busy):
  {==}

def Laws.cancel_then_hold_uses_fresh_identity(n):
  {==}
