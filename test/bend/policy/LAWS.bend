import Base
import ./model.bend as M

# Any failed sender predicate vetoes effect, regardless of the others.
law effect_requires_sender:
  for +destination: Bool
  for +token: Bool
  for +present: Bool
  for +amount: Bool
  for +data: Bool
  {M.effect(False{}, destination, token, present, amount, data) == False{} : Bool}

# Any failed destination predicate vetoes effect, regardless of the others.
law effect_requires_destination:
  for +sender: Bool
  for +token: Bool
  for +present: Bool
  for +amount: Bool
  for +data: Bool
  {M.effect(sender, False{}, token, present, amount, data) == False{} : Bool}

# Any failed token predicate vetoes effect, regardless of the others.
law effect_requires_token:
  for +sender: Bool
  for +destination: Bool
  for +present: Bool
  for +amount: Bool
  for +data: Bool
  {M.effect(sender, destination, False{}, present, amount, data) == False{} : Bool}

# Any failed present predicate vetoes effect, regardless of the others.
law effect_requires_present:
  for +sender: Bool
  for +destination: Bool
  for +token: Bool
  for +amount: Bool
  for +data: Bool
  {M.effect(sender, destination, token, False{}, amount, data) == False{} : Bool}

# Any failed amount predicate vetoes effect, regardless of the others.
law effect_requires_amount:
  for +sender: Bool
  for +destination: Bool
  for +token: Bool
  for +present: Bool
  for +data: Bool
  {M.effect(sender, destination, token, present, False{}, data) == False{} : Bool}

# Any failed data predicate vetoes effect, regardless of the others.
law effect_requires_data:
  for +sender: Bool
  for +destination: Bool
  for +token: Bool
  for +present: Bool
  for +amount: Bool
  {M.effect(sender, destination, token, present, amount, False{}) == False{} : Bool}

# A valid request is accepted; forbids an always-refuse model.
law effect_accepts_valid:
  {M.effect(True{}, True{}, True{}, True{}, True{}, True{}) == True{} : Bool}

# Any failed identity predicate vetoes publish, regardless of the others.
law publish_requires_identity:
  for +session: Bool
  for +effect: Bool
  for +chain: Bool
  for +consent: Bool
  for +prepared: Bool
  for +durable: Bool
  {M.publish(False{}, session, effect, chain, consent, prepared, durable) == False{} : Bool}

# Any failed session predicate vetoes publish, regardless of the others.
law publish_requires_session:
  for +identity: Bool
  for +effect: Bool
  for +chain: Bool
  for +consent: Bool
  for +prepared: Bool
  for +durable: Bool
  {M.publish(identity, False{}, effect, chain, consent, prepared, durable) == False{} : Bool}

# Any failed effect predicate vetoes publish, regardless of the others.
law publish_requires_effect:
  for +identity: Bool
  for +session: Bool
  for +chain: Bool
  for +consent: Bool
  for +prepared: Bool
  for +durable: Bool
  {M.publish(identity, session, False{}, chain, consent, prepared, durable) == False{} : Bool}

# Any failed chain predicate vetoes publish, regardless of the others.
law publish_requires_chain:
  for +identity: Bool
  for +session: Bool
  for +effect: Bool
  for +consent: Bool
  for +prepared: Bool
  for +durable: Bool
  {M.publish(identity, session, effect, False{}, consent, prepared, durable) == False{} : Bool}

# Any failed consent predicate vetoes publish, regardless of the others.
law publish_requires_consent:
  for +identity: Bool
  for +session: Bool
  for +effect: Bool
  for +chain: Bool
  for +prepared: Bool
  for +durable: Bool
  {M.publish(identity, session, effect, chain, False{}, prepared, durable) == False{} : Bool}

# Any failed prepared predicate vetoes publish, regardless of the others.
law publish_requires_prepared:
  for +identity: Bool
  for +session: Bool
  for +effect: Bool
  for +chain: Bool
  for +consent: Bool
  for +durable: Bool
  {M.publish(identity, session, effect, chain, consent, False{}, durable) == False{} : Bool}

# Any failed durable predicate vetoes publish, regardless of the others.
law publish_requires_durable:
  for +identity: Bool
  for +session: Bool
  for +effect: Bool
  for +chain: Bool
  for +consent: Bool
  for +prepared: Bool
  {M.publish(identity, session, effect, chain, consent, prepared, False{}) == False{} : Bool}

# A valid request is accepted; forbids an always-refuse model.
law publish_accepts_valid:
  {M.publish(True{}, True{}, True{}, True{}, True{}, True{}, True{}) == True{} : Bool}

# Any failed current predicate vetoes reconnect, regardless of the others.
law reconnect_requires_current:
  for +embedded: Bool
  for +running: Bool
  for +offline: Bool
  for +not_stopping: Bool
  for +not_wedged: Bool
  {M.reconnect(False{}, embedded, running, offline, not_stopping, not_wedged) == False{} : Bool}

# Any failed embedded predicate vetoes reconnect, regardless of the others.
law reconnect_requires_embedded:
  for +current: Bool
  for +running: Bool
  for +offline: Bool
  for +not_stopping: Bool
  for +not_wedged: Bool
  {M.reconnect(current, False{}, running, offline, not_stopping, not_wedged) == False{} : Bool}

# Any failed running predicate vetoes reconnect, regardless of the others.
law reconnect_requires_running:
  for +current: Bool
  for +embedded: Bool
  for +offline: Bool
  for +not_stopping: Bool
  for +not_wedged: Bool
  {M.reconnect(current, embedded, False{}, offline, not_stopping, not_wedged) == False{} : Bool}

# Any failed offline predicate vetoes reconnect, regardless of the others.
law reconnect_requires_offline:
  for +current: Bool
  for +embedded: Bool
  for +running: Bool
  for +not_stopping: Bool
  for +not_wedged: Bool
  {M.reconnect(current, embedded, running, False{}, not_stopping, not_wedged) == False{} : Bool}

# Any failed not_stopping predicate vetoes reconnect, regardless of the others.
law reconnect_requires_not_stopping:
  for +current: Bool
  for +embedded: Bool
  for +running: Bool
  for +offline: Bool
  for +not_wedged: Bool
  {M.reconnect(current, embedded, running, offline, False{}, not_wedged) == False{} : Bool}

# Any failed not_wedged predicate vetoes reconnect, regardless of the others.
law reconnect_requires_not_wedged:
  for +current: Bool
  for +embedded: Bool
  for +running: Bool
  for +offline: Bool
  for +not_stopping: Bool
  {M.reconnect(current, embedded, running, offline, not_stopping, False{}) == False{} : Bool}

# A valid request is accepted; forbids an always-refuse model.
law reconnect_accepts_valid:
  {M.reconnect(True{}, True{}, True{}, True{}, True{}, True{}) == True{} : Bool}

law mainnet_needs_consent:
  {M.mainnet(True{}, False{}) == False{} : Bool}

law mainnet_opt_in_works:
  {M.mainnet(True{}, True{}) == True{} : Bool}

law testnet_needs_no_opt_in:
  for +consent: Bool
  {M.mainnet(False{}, consent) == True{} : Bool}

law wrap_needs_positive:
  for +zero: Bool
  for +znn: Bool
  {M.funding(M.Wrap{}, False{}, zero, znn) == False{} : Bool}

law wrap_accepts_positive:
  for +zero: Bool
  for +znn: Bool
  {M.funding(M.Wrap{}, True{}, zero, znn) == True{} : Bool}

law redeem_never_carries_funds:
  for +positive: Bool
  for +znn: Bool
  {M.funding(M.Redeem{}, positive, False{}, znn) == False{} : Bool}

law redeem_needs_znn:
  for +positive: Bool
  {M.funding(M.Redeem{}, positive, True{}, False{}) == False{} : Bool}

law canonical_redeem_works:
  for +positive: Bool
  {M.funding(M.Redeem{}, positive, True{}, True{}) == True{} : Bool}

law privileged_method_refused:
  for +positive: Bool
  for +zero: Bool
  for +znn: Bool
  {M.funding(M.Other{}, positive, zero, znn) == False{} : Bool}

law negative_decimals_refused:
  for +value: Nat
  {M.decimals(True{}, value) == False{} : Bool}

law zero_decimals_allowed:
  {M.decimals(False{}, 0n) == True{} : Bool}

law eighteen_decimals_allowed:
  {M.decimals(False{}, 18n) == True{} : Bool}

law excess_decimals_refused:
  for +extra: Nat
  {M.decimals(False{}, 19n+extra) == False{} : Bool}

law autolock_disabled:
  for +elapsed: Nat
  {M.expired(0n, elapsed) == False{} : Bool}
