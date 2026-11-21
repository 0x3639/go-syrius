import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const GetCollateral = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NomService', () => ({ GetCollateral }))

import { READ_TIMEOUT_MS } from '../lib/withTimeout'
import { useCollateralStore } from './collateral'
import { useNodeStore } from './node'

const snapshot = (pillarZnn: string) => ({ pillarZnn, sentinelZnn: '0', sentinelQsr: '0', pillarDepositedQsr: '0', sentinelDepositedQsr: '0' })

describe('collateral store', () => {
  beforeEach(() => { setActivePinia(createPinia()); GetCollateral.mockReset() })

  it('loads a snapshot and is available only while connected', async () => {
    const s = useCollateralStore()
    GetCollateral.mockResolvedValueOnce(snapshot('15'))
    await s.refresh()
    expect(s.info?.pillarZnn).toBe('15')
    expect(s.available).toBe(false)
    useNodeStore().connected = true
    expect(s.available).toBe(true)
  })

  it('a failed load leaves no snapshot rather than an old one', async () => {
    const s = useCollateralStore()
    GetCollateral.mockResolvedValueOnce(snapshot('15'))
    await s.refresh()
    GetCollateral.mockRejectedValueOnce(new Error('offline'))
    await s.refresh()
    expect(s.info).toBeNull()
    expect(s.status).toBe('error')
  })

  it('resync marks the snapshot stale until the newest reload lands', async () => {
    const s = useCollateralStore()
    let first!: (v: unknown) => void
    let second!: (v: unknown) => void
    GetCollateral.mockReturnValueOnce(new Promise((r) => { first = r }))
    GetCollateral.mockReturnValueOnce(new Promise((r) => { second = r }))
    const a = s.resync()
    const b = s.resync()
    expect(s.stale).toBe(true)
    first(snapshot('1'))
    await a
    expect(s.stale).toBe(true) // superseded response must not clear the mark
    second(snapshot('2'))
    await b
    expect(s.stale).toBe(false)
    expect(s.info?.pillarZnn).toBe('2')
  })

  it('invalidate drops the snapshot and the stale mark', async () => {
    const s = useCollateralStore()
    GetCollateral.mockResolvedValueOnce(snapshot('15'))
    await s.refresh()
    s.stale = true
    s.invalidate()
    expect(s.info).toBeNull()
    expect(s.stale).toBe(false)
  })

  // A node slower than the tick interval: every tick used to supersede the
  // request before it, so no response was ever accepted.
  it('periodic reloads let the request in flight land instead of superseding it', async () => {
    const s = useCollateralStore()
    let land!: (v: unknown) => void
    GetCollateral.mockReturnValueOnce(new Promise((r) => { land = r }))
    const first = s.refreshIfIdle()
    await s.refreshIfIdle() // next tick, response still outstanding
    await s.refreshIfIdle()
    expect(GetCollateral).toHaveBeenCalledTimes(1)
    land(snapshot('9'))
    await first
    expect(s.info?.pillarZnn).toBe('9')
    GetCollateral.mockResolvedValueOnce(snapshot('9'))
    await s.refreshIfIdle() // idle again: the next tick reloads
    expect(GetCollateral).toHaveBeenCalledTimes(2)
  })

  it('a periodic reload is not blocked by a request that can no longer land', async () => {
    const s = useCollateralStore()
    GetCollateral.mockReturnValueOnce(new Promise(() => {})) // never answers
    s.refreshIfIdle()
    s.invalidate() // account or connection changed
    GetCollateral.mockResolvedValueOnce(snapshot('9'))
    await s.refreshIfIdle()
    expect(GetCollateral).toHaveBeenCalledTimes(2)
    expect(s.info?.pillarZnn).toBe('9')
  })

  // The WebSocket transport has no per-call deadline: a read the node never
  // answers would otherwise block periodic reloads and hold the rows forever.
  it('gives up on an unanswered read, reports unknown and lets the next reload run', async () => {
    vi.useFakeTimers()
    try {
      const s = useCollateralStore()
      s.stale = true
      GetCollateral.mockReturnValueOnce(new Promise(() => {})) // never answers
      const hung = s.refreshIfIdle()
      await s.refreshIfIdle()
      expect(GetCollateral).toHaveBeenCalledTimes(1) // gate is held while it is outstanding
      await vi.advanceTimersByTimeAsync(READ_TIMEOUT_MS)
      await hung
      expect(s.status).toBe('error') // shown as "couldn't check", not as old data
      expect(s.stale).toBe(false)
      GetCollateral.mockResolvedValueOnce(snapshot('9'))
      await s.refreshIfIdle()
      expect(GetCollateral).toHaveBeenCalledTimes(2)
      expect(s.info?.pillarZnn).toBe('9')
      expect(s.status).toBe('ready')
    } finally { vi.useRealTimers() }
  })
})
