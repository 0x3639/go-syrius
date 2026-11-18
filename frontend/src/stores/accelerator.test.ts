import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAcceleratorStore } from './accelerator'
import * as Nom from '../../wailsjs/go/app/NomService'

const GetVotableForMyPillars = vi.hoisted(() => vi.fn())
const GetActivePillarCount = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetVotableForMyPillars, GetActivePillarCount,
  GetProjects: vi.fn(), GetProject: vi.fn(), GetVotablePillars: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('accelerator store votable', () => {
  it.each(['resolve', 'reject'])('ignores a superseded project page %s', async (outcome) => {
    let finish!: (r: any) => void
    let fail!: (e: Error) => void
    vi.mocked(Nom.GetProjects).mockImplementationOnce(() => new Promise((resolve, reject) => { finish = resolve; fail = reject }))
    const s = useAcceleratorStore()
    const old = s.loadProjects(0)
    vi.mocked(Nom.GetProjects).mockResolvedValueOnce({ list: [{ id: 'new' }], count: 50 } as any)
    await s.loadProjects(1)
    if (outcome === 'resolve') finish({ list: [{ id: 'old' }], count: 1 })
    else fail(new Error('old page error'))
    await old
    expect(s.projects[0].id).toBe('new')
    expect(s.projectPage).toBe(1)
    expect(s.projectCount).toBe(50)
    expect(s.error).toBe('')
  })
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
