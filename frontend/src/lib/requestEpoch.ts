// Account/session request epoch.
//
// Account-scoped RPCs (balances, history, stakes, …) are launched on account
// switches and momentum ticks without cancellation, so a slow response for
// account A can land after account B was selected and overwrite B's state.
// The wallet store bumps this epoch on every unlock/lock/account switch; a
// store captures it before awaiting and discards the response if it changed
// while the request was in flight.
let epoch = 0

export function bumpRequestEpoch(): void {
  epoch++
}

export function currentRequestEpoch(): number {
  return epoch
}
