import { describe, it, expect } from 'vitest'
import { isPassing, quorumNeeded, statusLabel } from './accelerator'

describe('accelerator vote math', () => {
  it('isPassing requires strict majority AND >33% turnout', () => {
    // 4 yes, 1 no, 5 total, 10 pillars → 5*100=500 > 10*33=330 ✓, 4>1 ✓
    expect(isPassing(4, 1, 5, 10)).toBe(true)
    // tie fails majority
    expect(isPassing(2, 2, 5, 10)).toBe(false)
    // below quorum: 3 total of 10 pillars → 300 <= 330
    expect(isPassing(3, 0, 3, 10)).toBe(false)
    // exactly 33% fails (strict >): 33 total, 100 pillars → 3300 <= 3300
    expect(isPassing(33, 0, 33, 100)).toBe(false)
    // just over: 34 total, 100 pillars → 3400 > 3300
    expect(isPassing(34, 0, 34, 100)).toBe(true)
  })
  it('quorumNeeded is the smallest turnout that passes (floor(33%)+1)', () => {
    expect(quorumNeeded(100)).toBe(34) // 33 votes fails; 34 passes
    expect(quorumNeeded(10)).toBe(4) // floor(3.3)+1
    expect(quorumNeeded(0)).toBe(0)
    // quorumNeeded is exactly the smallest total isPassing accepts
    expect(isPassing(quorumNeeded(100), 0, quorumNeeded(100), 100)).toBe(true)
    expect(isPassing(quorumNeeded(100) - 1, 0, quorumNeeded(100) - 1, 100)).toBe(false)
  })
  it('statusLabel maps known statuses', () => {
    expect(statusLabel(0)).toBe('Voting')
    expect(statusLabel(1)).toBe('Active')
    expect(statusLabel(3)).toBe('Closed')
    expect(statusLabel(4)).toBe('Completed')
    expect(statusLabel(9)).toBe('#9')
  })
})
