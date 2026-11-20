import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def Laws.effect_requires_sender(destination, token, present, amount, data):
  {==}

def Laws.effect_requires_destination(sender, token, present, amount, data):
  match sender:
    case False{}:
      {==}
    case True{}:
      {==}

def Laws.effect_requires_token(sender, destination, present, amount, data):
  match sender destination:
    case False{} _:
      {==}
    case True{} False{}:
      {==}
    case True{} True{}:
      {==}

def Laws.effect_requires_present(sender, destination, token, amount, data):
  match sender destination token:
    case False{} _ _:
      {==}
    case True{} False{} _:
      {==}
    case True{} True{} False{}:
      {==}
    case True{} True{} True{}:
      {==}

def Laws.effect_requires_amount(sender, destination, token, present, data):
  match sender destination token present:
    case False{} _ _ _:
      {==}
    case True{} False{} _ _:
      {==}
    case True{} True{} False{} _:
      {==}
    case True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{}:
      {==}

def Laws.effect_requires_data(sender, destination, token, present, amount):
  match sender destination token present amount:
    case False{} _ _ _ _:
      {==}
    case True{} False{} _ _ _:
      {==}
    case True{} True{} False{} _ _:
      {==}
    case True{} True{} True{} False{} _:
      {==}
    case True{} True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{} True{}:
      {==}

def Laws.effect_accepts_valid():
  {==}

def Laws.publish_requires_identity(session, effect, chain, consent, prepared, durable):
  {==}

def Laws.publish_requires_session(identity, effect, chain, consent, prepared, durable):
  match identity:
    case False{}:
      {==}
    case True{}:
      {==}

def Laws.publish_requires_effect(identity, session, chain, consent, prepared, durable):
  match identity session:
    case False{} _:
      {==}
    case True{} False{}:
      {==}
    case True{} True{}:
      {==}

def Laws.publish_requires_chain(identity, session, effect, consent, prepared, durable):
  match identity session effect:
    case False{} _ _:
      {==}
    case True{} False{} _:
      {==}
    case True{} True{} False{}:
      {==}
    case True{} True{} True{}:
      {==}

def Laws.publish_requires_consent(identity, session, effect, chain, prepared, durable):
  match identity session effect chain:
    case False{} _ _ _:
      {==}
    case True{} False{} _ _:
      {==}
    case True{} True{} False{} _:
      {==}
    case True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{}:
      {==}

def Laws.publish_requires_prepared(identity, session, effect, chain, consent, durable):
  match identity session effect chain consent:
    case False{} _ _ _ _:
      {==}
    case True{} False{} _ _ _:
      {==}
    case True{} True{} False{} _ _:
      {==}
    case True{} True{} True{} False{} _:
      {==}
    case True{} True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{} True{}:
      {==}

def Laws.publish_requires_durable(identity, session, effect, chain, consent, prepared):
  match identity session effect chain consent prepared:
    case False{} _ _ _ _ _:
      {==}
    case True{} False{} _ _ _ _:
      {==}
    case True{} True{} False{} _ _ _:
      {==}
    case True{} True{} True{} False{} _ _:
      {==}
    case True{} True{} True{} True{} False{} _:
      {==}
    case True{} True{} True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{} True{} True{}:
      {==}

def Laws.publish_accepts_valid():
  {==}

def Laws.reconnect_requires_current(embedded, running, offline, not_stopping, not_wedged):
  {==}

def Laws.reconnect_requires_embedded(current, running, offline, not_stopping, not_wedged):
  match current:
    case False{}:
      {==}
    case True{}:
      {==}

def Laws.reconnect_requires_running(current, embedded, offline, not_stopping, not_wedged):
  match current embedded:
    case False{} _:
      {==}
    case True{} False{}:
      {==}
    case True{} True{}:
      {==}

def Laws.reconnect_requires_offline(current, embedded, running, not_stopping, not_wedged):
  match current embedded running:
    case False{} _ _:
      {==}
    case True{} False{} _:
      {==}
    case True{} True{} False{}:
      {==}
    case True{} True{} True{}:
      {==}

def Laws.reconnect_requires_not_stopping(current, embedded, running, offline, not_wedged):
  match current embedded running offline:
    case False{} _ _ _:
      {==}
    case True{} False{} _ _:
      {==}
    case True{} True{} False{} _:
      {==}
    case True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{}:
      {==}

def Laws.reconnect_requires_not_wedged(current, embedded, running, offline, not_stopping):
  match current embedded running offline not_stopping:
    case False{} _ _ _ _:
      {==}
    case True{} False{} _ _ _:
      {==}
    case True{} True{} False{} _ _:
      {==}
    case True{} True{} True{} False{} _:
      {==}
    case True{} True{} True{} True{} False{}:
      {==}
    case True{} True{} True{} True{} True{}:
      {==}

def Laws.reconnect_accepts_valid():
  {==}

def Laws.mainnet_needs_consent():
  {==}

def Laws.mainnet_opt_in_works():
  {==}

def Laws.testnet_needs_no_opt_in(consent):
  {==}

def Laws.wrap_needs_positive(zero, znn):
  {==}

def Laws.wrap_accepts_positive(zero, znn):
  {==}

def Laws.redeem_never_carries_funds(positive, znn):
  {==}

def Laws.redeem_needs_znn(positive):
  {==}

def Laws.canonical_redeem_works(positive):
  {==}

def Laws.privileged_method_refused(positive, zero, znn):
  {==}

def Laws.negative_decimals_refused(value):
  {==}

def Laws.zero_decimals_allowed():
  {==}

def Laws.eighteen_decimals_allowed():
  {==}

def Laws.excess_decimals_refused(extra):
  {==}

def Laws.autolock_disabled(elapsed):
  {==}
