import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
const PrepareSend = vi.hoisted(() => vi.fn())
const ConfirmPublish = vi.hoisted(() => vi.fn())
const CancelPending = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/TxService', () => ({ PrepareSend, ConfirmPublish, CancelPending }))
import { useTxStore } from './tx'
beforeEach(() => { setActivePinia(createPinia()); PrepareSend.mockReset(); ConfirmPublish.mockReset() })
describe('tx store (confirm-what-you-sign)', () => {
  it('prepare seats the built-block preview', async () => {
    PrepareSend.mockResolvedValue({ toAddress: 'z1', amount: '150000000', zts: 'zts1znn', needsPoW: true, difficulty: 1, hash: 'h' })
    const s = useTxStore(); await s.prepare('z1', 'zts1znn', '150000000')
    expect(s.status).toBe('awaiting'); expect(s.preview?.amount).toBe('150000000')
  })
  it('confirm publishes the held block', async () => {
    ConfirmPublish.mockResolvedValue('hash123')
    const s = useTxStore(); await s.confirm()
    expect(ConfirmPublish).toHaveBeenCalled(); expect(s.status).toBe('done'); expect(s.hash).toBe('hash123')
  })
  it('prepare error sets error state', async () => {
    PrepareSend.mockRejectedValue(new Error('bad addr'))
    const s = useTxStore(); await s.prepare('x', 'z', '1'); expect(s.status).toBe('error'); expect(s.error).toBe('bad addr')
  })

  it('a stale cancel does not wipe a NEWER awaiting transaction', async () => {
    const s = useTxStore()
    s.awaitConfirm({ summary: 'first' } as any)
    // Hold CancelPending open so a new prepare can land mid-round-trip.
    let release!: () => void
    CancelPending.mockReturnValueOnce(new Promise<void>((r) => { release = r }))
    const cancelling = s.cancel()
    s.awaitConfirm({ summary: 'second' } as any) // user starts a new action
    release()
    await cancelling
    // Same status ('awaiting') but a different transaction — must survive.
    expect(s.status).toBe('awaiting')
    expect((s.preview as any)?.summary).toBe('second')
  })

  it('discard leaves awaiting synchronously and releases the backend hold', () => {
    const s = useTxStore()
    s.awaitConfirm({ summary: 'gov' } as any)
    s.discard()
    expect(s.status).toBe('idle') // no frame with a live Confirm button
    expect(CancelPending).toHaveBeenCalled()
  })

  it('discard is a no-op outside awaiting', () => {
    const s = useTxStore()
    s.status = 'publishing'
    CancelPending.mockClear()
    s.discard()
    expect(s.status).toBe('publishing')
    expect(CancelPending).not.toHaveBeenCalled()
  })
})
