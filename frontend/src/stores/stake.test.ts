import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetStakeList, GetUncollectedReward } = vi.hoisted(() => ({ GetStakeList: vi.fn(), GetUncollectedReward: vi.fn() }))
vi.mock('../../wailsjs/go/app/NomService', () => ({ GetStakeList, GetUncollectedReward }))

import { useStakeStore } from './stake'

describe('stake store resync', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    GetStakeList.mockReset(); GetUncollectedReward.mockReset()
    GetUncollectedReward.mockResolvedValue({ znn: '0', qsr: '0' })
  })

  it('marks the snapshot stale at once and clears it when fresh data lands', async () => {
    const s = useStakeStore()
    GetStakeList.mockResolvedValueOnce({ totalAmount: '500', entries: [] })
    await s.refresh()
    expect(s.stale).toBe(false)

    let land!: (v: unknown) => void
    GetStakeList.mockReturnValueOnce(new Promise((r) => { land = r }))
    const pending = s.resync()
    expect(s.stale).toBe(true)
    expect(s.stakeInfo?.totalAmount).toBe('500') // old data still held, but flagged
    land({ totalAmount: '0', entries: [] })
    await pending
    expect(s.stale).toBe(false)
    expect(s.stakeInfo?.totalAmount).toBe('0')
  })

  it('stays stale until the newest of overlapping reloads lands', async () => {
    const s = useStakeStore()
    let first!: (v: unknown) => void
    let second!: (v: unknown) => void
    GetStakeList.mockReturnValueOnce(new Promise((r) => { first = r }))
    GetStakeList.mockReturnValueOnce(new Promise((r) => { second = r }))
    const a = s.resync()
    const b = s.resync()
    first({ totalAmount: '1', entries: [] })
    await a
    expect(s.stale).toBe(true) // superseded response must not clear the mark
    second({ totalAmount: '2', entries: [] })
    await b
    expect(s.stale).toBe(false)
    expect(s.stakeInfo?.totalAmount).toBe('2')
  })

  it('clears the mark when the reload fails', async () => {
    const s = useStakeStore()
    GetStakeList.mockRejectedValueOnce(new Error('offline'))
    await s.resync()
    expect(s.stale).toBe(false)
    expect(s.stakeInfo).toBeNull()
  })
})
