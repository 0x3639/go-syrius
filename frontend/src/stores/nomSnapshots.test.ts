import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetStakeList: vi.fn(), GetUncollectedReward: vi.fn(),
  GetSentinel: vi.fn(), GetDepositedQsr: vi.fn(), GetSentinelReward: vi.fn(),
  GetMyPillar: vi.fn(), GetPillarDepositedQsr: vi.fn(), GetPillarQsrCost: vi.fn(),
  GetPlasmaInfo: vi.fn(), GetPillarReward: vi.fn(), GetPillarList: vi.fn(), GetDelegation: vi.fn(),
}))
import * as Nom from '../../wailsjs/go/app/NomService'
import { useStakeStore } from './stake'
import { usePillarStore } from './pillar'
import { useSentinelStore } from './sentinel'
import { bumpRequestEpoch } from '../lib/requestEpoch'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.resetAllMocks()
  for (const read of Object.values(Nom)) vi.mocked(read).mockResolvedValue({} as never)
})
const cases = [
  { name: 'stake', store: useStakeStore, read: Nom.GetStakeList, reward: Nom.GetUncollectedReward, snapshot: 'stakeInfo', refresh: 'refresh' },
  { name: 'pillar', store: usePillarStore, read: Nom.GetMyPillar, reward: Nom.GetPillarReward, snapshot: 'myPillar', refresh: 'refreshRegistration' },
  { name: 'sentinel', store: useSentinelStore, read: Nom.GetSentinel, reward: Nom.GetSentinelReward, snapshot: 'sentinel', refresh: 'refresh' },
]

describe.each(cases)('$name snapshots', ({ store, read, reward, snapshot, refresh }) => {
  it('does not restore a reward sampled before a newer collection refresh', async () => {
    const s = store() as any
    let finish!: (r: any) => void
    vi.mocked(reward).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const old = s[refresh]()
    vi.mocked(reward).mockResolvedValueOnce({ znn: '0', qsr: '0' } as any)
    await s[refresh]()
    finish({ znn: '0', qsr: '100' })
    await old
    expect(s.reward.qsr).toBe('0')
  })

  it('clears the previous account snapshot even when its replacement lookup fails', async () => {
    const s = store() as any
    vi.mocked(read).mockResolvedValueOnce({ name: 'Old pillar', owner: 'old account', entries: [] } as any)
    await s[refresh]()
    expect(s[snapshot]).not.toBeNull()
    bumpRequestEpoch()
    vi.mocked(read).mockRejectedValueOnce(new Error('unavailable'))
    await s[refresh]()
    expect(s[snapshot]).toBeNull()
    expect(s.reward).toBeNull()
  })
})

it('orders pillar reward writes across registration and delegation refreshes', async () => {
  const s = usePillarStore()
  let finish!: (r: any) => void
  vi.mocked(Nom.GetPillarReward).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
  const old = s.refreshRegistration()
  vi.mocked(Nom.GetPillarReward).mockResolvedValueOnce({ znn: '0', qsr: '0' } as any)
  await s.refresh()
  finish({ znn: '100', qsr: '100' })
  await old
  expect(s.reward?.qsr).toBe('0')
})
