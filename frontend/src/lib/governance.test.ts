import { describe, it, expect } from 'vitest'
import {
  actionStatusLabel,
  actionTypeLabel,
  isOpen,
  isActionApproved,
  isActionRejected,
} from './governance'

describe('governance vote math', () => {
  it('labels statuses and types', () => {
    expect(actionStatusLabel(0)).toBe('Voting')
    expect(actionStatusLabel(3)).toBe('NoDecision')
    expect(actionStatusLabel(9)).toBe('#9')
    expect(actionTypeLabel(1)).toBe('Spork')
    expect(actionTypeLabel(2)).toBe('Normal')
  })
  it('isOpen = Voting && !expired', () => {
    expect(isOpen({ status: 0, expired: false })).toBe(true)
    expect(isOpen({ status: 0, expired: true })).toBe(false)
    expect(isOpen({ status: 1, expired: false })).toBe(false)
  })
  it('approval needs quorum on yes+no AND directional yes-share (abstain excluded)', () => {
    const thr = { activePillarThreshold: 50, directionalThreshold: 50 }
    // 100 pillars, round-0 Type2: quorum needs (yes+no)*100 > 5000 → >50 directional votes
    // 40 yes / 10 no / 30 abstain → directional 50 → 5000 !> 5000 → no quorum
    expect(isActionApproved({ yes: 40, no: 10, total: 80 }, thr, 100)).toBe(false)
    // 60 yes / 10 no → directional 70 → 7000 > 5000 quorum; yes 6000 > 70*50=3500 → approved
    expect(isActionApproved({ yes: 60, no: 10, total: 70 }, thr, 100)).toBe(true)
    // abstain does not help quorum: 40 yes / 5 no / 100 abstain → directional 45 → no quorum
    expect(isActionApproved({ yes: 40, no: 5, total: 145 }, thr, 100)).toBe(false)
  })
  it('rejection mirrors approval on the no-share', () => {
    const thr = { activePillarThreshold: 50, directionalThreshold: 50 }
    expect(isActionRejected({ yes: 10, no: 60, total: 70 }, thr, 100)).toBe(true)
    expect(isActionRejected({ yes: 60, no: 10, total: 70 }, thr, 100)).toBe(false)
  })
})
