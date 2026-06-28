import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAcceleratorStore } from './accelerator'

const GetVotableForMyPillars = vi.hoisted(() => vi.fn())
const GetActivePillarCount = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetVotableForMyPillars, GetActivePillarCount,
  GetProjects: vi.fn(), GetProject: vi.fn(), GetVotablePillars: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('accelerator store votable', () => {
  it('refreshVotable populates state and needsVoteCount counts needs-vote items', async () => {
    GetVotableForMyPillars.mockResolvedValue([
      { id: '0xa', needsMyVote: true },
      { id: '0xb', needsMyVote: false },
      { id: '0xc', needsMyVote: true },
    ])
    GetActivePillarCount.mockResolvedValue(42)
    const s = useAcceleratorStore()
    await s.refreshVotable()
    expect(s.votable.length).toBe(3)
    expect(s.numActivePillars).toBe(42)
    expect(s.needsVoteCount).toBe(2)
  })

  it('refreshVotable swallows errors (locked/disconnected → empty)', async () => {
    GetVotableForMyPillars.mockRejectedValue(new Error('locked'))
    GetActivePillarCount.mockResolvedValue(0)
    const s = useAcceleratorStore()
    await s.refreshVotable()
    expect(s.votable).toEqual([])
    expect(s.needsVoteCount).toBe(0)
  })
})
