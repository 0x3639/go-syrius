// Accelerator-Z vote math — the single source of truth shared by the Vote view,
// the Projects "awaiting payout" filter, and the quorum bar. Mirrors go-zenon's
// checkAcceleratorVotes: strict majority (yes>no) AND turnout above 33% of the
// active pillar count.
export const AZ_STATUS = ['Voting', 'Active', 'Paid', 'Closed', 'Completed'] as const

export function statusLabel(n: number): string {
  return AZ_STATUS[n] ?? `#${n}`
}

export function quorumNeeded(numPillars: number): number {
  return Math.ceil(numPillars * 0.33)
}

export function isPassing(yes: number, no: number, total: number, numPillars: number): boolean {
  return yes > no && total * 100 > numPillars * 33
}
