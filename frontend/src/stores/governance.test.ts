import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetActions, GetVotablePillars, GetActivePillarCount } = vi.hoisted(() => ({
  GetActions: vi.fn(),
  GetVotablePillars: vi.fn(),
  GetActivePillarCount: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetActions,
  GetAction: vi.fn(),
  GetVotablePillars,
  GetActivePillarCount,
}))

import { useGovernanceStore } from './governance'

describe('governance store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('loadActions sets actions + count + page', async () => {
    GetActions.mockResolvedValue({ count: 42, list: [{ id: 'a1' }, { id: 'a2' }] })
    const s = useGovernanceStore()
    await s.loadActions(1)
    expect(GetActions).toHaveBeenCalledWith(1, 20)
    expect(s.actions).toEqual([{ id: 'a1' }, { id: 'a2' }])
    expect(s.actionCount).toBe(42)
    expect(s.actionPage).toBe(1)
  })

  it('loadActions surfaces error', async () => {
    GetActions.mockRejectedValue(new Error('boom'))
    const s = useGovernanceStore()
    await s.loadActions()
    expect(s.error).toBe('boom')
  })

  it('loadVotablePillars swallows errors to empty', async () => {
    GetVotablePillars.mockRejectedValue(new Error('locked'))
    const s = useGovernanceStore()
    await s.loadVotablePillars()
    expect(s.votablePillars).toEqual([])
  })

  it('loadActivePillarCount swallows errors to 0', async () => {
    GetActivePillarCount.mockRejectedValue(new Error('locked'))
    const s = useGovernanceStore()
    await s.loadActivePillarCount()
    expect(s.numActivePillars).toBe(0)
  })
})
