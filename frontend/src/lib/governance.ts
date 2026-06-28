// Governance action vote math — mirrors go-zenon checkActionVoteBreakdown.
// Per the CURRENT round only; abstain is EXCLUDED from the directional total
// (unlike the accelerator, which counts abstain toward quorum). Thresholds are
// read off the action (the node computes the current round's values).
export const ACTION_STATUS = ['Voting', 'Approved', 'Rejected', 'NoDecision'] as const

export function actionStatusLabel(n: number): string {
  return ACTION_STATUS[n] ?? `#${n}`
}

export function actionTypeLabel(t: number): string {
  return t === 1 ? 'Spork' : t === 2 ? 'Normal' : `#${t}`
}

export function isOpen(a: { status: number; expired: boolean }): boolean {
  return a.status === 0 && !a.expired
}

type Votes = { yes: number; no: number; total: number }
type Thresholds = { activePillarThreshold: number; directionalThreshold: number }

function quorumMet(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return directional * 100 > numPillars * a.activePillarThreshold
}

export function isActionApproved(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return quorumMet(v, a, numPillars) && v.yes * 100 > directional * a.directionalThreshold
}

export function isActionRejected(v: Votes, a: Thresholds, numPillars: number): boolean {
  const directional = v.yes + v.no
  return quorumMet(v, a, numPillars) && v.no * 100 > directional * a.directionalThreshold
}
