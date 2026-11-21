import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetStakeList, GetUncollectedReward } = vi.hoisted(() => ({ GetStakeList: vi.fn(), GetUncollectedReward: vi.fn() }))
vi.mock('../../wailsjs/go/app/NomService', () => ({ GetStakeList, GetUncollectedReward }))

import { READ_TIMEOUT_MS } from '../lib/withTimeout'
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

  // A node slower than the tick interval: every tick used to supersede the
  // request before it, so no response was ever accepted.
  it('periodic reloads let the request in flight land instead of superseding it', async () => {
    const s = useStakeStore()
    let land!: (v: unknown) => void
    GetStakeList.mockReturnValueOnce(new Promise((r) => { land = r }))
    const first = s.refreshIfIdle()
    await s.refreshIfIdle() // next tick, response still outstanding
    await s.refreshIfIdle()
    expect(GetStakeList).toHaveBeenCalledTimes(1)
    land({ totalAmount: '9', entries: [] })
    await first
    expect(s.stakeInfo?.totalAmount).toBe('9')
    GetStakeList.mockResolvedValueOnce({ totalAmount: '9', entries: [] })
    await s.refreshIfIdle() // idle again: the next tick reloads
    expect(GetStakeList).toHaveBeenCalledTimes(2)
  })

  it('a periodic reload is not blocked by a request that can no longer land', async () => {
    const s = useStakeStore()
    GetStakeList.mockReturnValueOnce(new Promise(() => {})) // never answers
    s.refreshIfIdle()
    s.invalidate() // account or connection changed
    GetStakeList.mockResolvedValueOnce({ totalAmount: '9', entries: [] })
    await s.refreshIfIdle()
    expect(GetStakeList).toHaveBeenCalledTimes(2)
    expect(s.stakeInfo?.totalAmount).toBe('9')
  })

  // The WebSocket transport has no per-call deadline: a read the node never
  // answers would otherwise block periodic reloads and hold the rows forever.
  it('gives up on an unanswered read, reports unknown and lets the next reload run', async () => {
    vi.useFakeTimers()
    try {
      const s = useStakeStore()
      s.stale = true
      GetStakeList.mockReturnValueOnce(new Promise(() => {})) // never answers
      const hung = s.refreshIfIdle()
      await s.refreshIfIdle()
      expect(GetStakeList).toHaveBeenCalledTimes(1) // gate is held while it is outstanding
      await vi.advanceTimersByTimeAsync(READ_TIMEOUT_MS)
      await hung
      expect(s.status).toBe('error') // shown as "couldn't check", not as old data
      expect(s.stale).toBe(false)
      GetStakeList.mockResolvedValueOnce({ totalAmount: '9', entries: [] })
      await s.refreshIfIdle()
      expect(GetStakeList).toHaveBeenCalledTimes(2)
      expect(s.stakeInfo?.totalAmount).toBe('9')
      expect(s.status).toBe('ready')
    } finally { vi.useRealTimers() }
  })
})
