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

  it('discard leaves awaiting synchronously and releases the backend hold by identity', () => {
    const s = useTxStore()
    s.awaitConfirm({ summary: 'gov', holdId: 7 } as any)
    s.discard()
    expect(s.status).toBe('idle') // no frame with a live Confirm button
    // Identity-checked: the backend can only release THIS hold — a newer
    // Prepare that wins a race against the RPC is untouchable.
    expect(CancelPending).toHaveBeenCalledWith(7)
  })

  it('a real broadcast ALWAYS surfaces, even after a mid-publish reset', async () => {
    let resolve!: (h: string) => void
    ConfirmPublish.mockReturnValueOnce(new Promise<string>((r) => { resolve = r }))
    const s = useTxStore()
    const publishing = s.confirm()
    s.reset() // user closed the dialog / navigated during the publish
    resolve('hash-real')
    await publishing
    // Funds moved on-chain — hiding the outcome would invite a double-send.
    expect(s.status).toBe('done')
    expect(s.hash).toBe('hash-real')
  })

  it('a stale publish FAILURE is dropped when the state was reset mid-publish', async () => {
    let reject!: (e: Error) => void
    ConfirmPublish.mockReturnValueOnce(new Promise<string>((_, rj) => { reject = rj }))
    const s = useTxStore()
    const publishing = s.confirm()
    s.reset() // router.afterEach fires on navigation
    reject(new Error('boom'))
    await publishing
    // Nothing happened on-chain — no orphan error dialog on the new screen.
    expect(s.status).toBe('idle')
    expect(s.error).toBe('')
  })

  it('a stale prepare cannot resurrect the dialog after a reset, and releases its hold', async () => {
    let resolve!: (p: unknown) => void
    PrepareSend.mockReturnValueOnce(new Promise((r) => { resolve = r }))
    const s = useTxStore()
    const preparing = s.prepare('z1', 'zts1znn', '1')
    s.reset() // navigation while PrepareSend is in flight
    CancelPending.mockClear()
    resolve({ toAddress: 'z1', amount: '1', holdId: 9 })
    await preparing
    expect(s.status).toBe('idle') // no live Confirm popping on another screen
    // The dropped prepare's backend hold must not linger in the pending slot.
    expect(CancelPending).toHaveBeenCalledWith(9)
  })

  it('a late success does not clobber a NEWER awaiting transaction', async () => {
    let resolve!: (h: string) => void
    ConfirmPublish.mockReturnValueOnce(new Promise<string>((r) => { resolve = r }))
    const s = useTxStore()
    s.awaitConfirm({ summary: 'A', holdId: 1 } as any)
    const publishingA = s.confirm()
    s.reset()
    s.awaitConfirm({ summary: 'B', holdId: 2 } as any) // user prepared B, dialog live
    resolve('hash-A')
    await publishingA
    expect(s.status).toBe('awaiting') // B's dialog intact (A lands in history)
    expect((s.preview as any)?.holdId).toBe(2)
  })

  it('a late success does not wipe a NEWER prepare in flight', async () => {
    let resolveConfirm!: (h: string) => void
    ConfirmPublish.mockReturnValueOnce(new Promise<string>((r) => { resolveConfirm = r }))
    let resolvePrepare!: (p: unknown) => void
    PrepareSend.mockReturnValueOnce(new Promise((r) => { resolvePrepare = r }))
    const s = useTxStore()
    s.awaitConfirm({ summary: 'A', holdId: 1 } as any)
    const publishingA = s.confirm()
    const preparingB = s.prepare('z1', 'zts1znn', '1') // user already sending B
    resolveConfirm('hash-A')
    await publishingA
    expect(s.status).toBe('preparing') // B's flow not hijacked by A's outcome
    resolvePrepare({ toAddress: 'z1', amount: '1', holdId: 2 })
    await preparingB
    expect(s.status).toBe('awaiting') // B's dialog appears as the user expects
  })

  it('a stale failure cannot hijack a newer publishing transaction (holdId guard)', async () => {
    let reject!: (e: Error) => void
    ConfirmPublish.mockReturnValueOnce(new Promise<string>((_, rj) => { reject = rj }))
    const s = useTxStore()
    s.awaitConfirm({ summary: 'A', holdId: 1 } as any)
    const publishingA = s.confirm()
    s.awaitConfirm({ summary: 'B', holdId: 2 } as any)
    s.status = 'publishing' // B's own confirm is now in flight
    reject(new Error('A failed'))
    await publishingA
    expect(s.status).toBe('publishing') // same status, different tx — A dropped
    expect(s.error).toBe('')
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
