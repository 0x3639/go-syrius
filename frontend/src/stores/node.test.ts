// stores/node.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
  NodeStatus: vi.fn().mockResolvedValue({ mode: 'remote', connected: true, syncing: false, height: 42, peers: 3, chainId: 3 }),
}))
// Capture EventsOn handlers so tests can fire backend events.
const handlers = vi.hoisted(() => ({}) as Record<string, (data?: any) => void>)
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn((evt: string, cb: (data?: any) => void) => { handlers[evt] = cb }),
}))
import { useNodeStore } from './node'
beforeEach(() => setActivePinia(createPinia()))
describe('node store', () => {
  it('connects and loads balances', async () => {
    const s = useNodeStore()
    await s.connect(); expect(s.connected).toBe(true)
    await s.loadBalances(); expect(s.balances[0].symbol).toBe('ZNN')
  })

  it('initEvents re-points the momentum:tick callback on later calls', () => {
    const s = useNodeStore()
    const firstMount = vi.fn()
    const secondMount = vi.fn()
    s.initEvents(firstMount)
    // AppShell remounts after a lock/unlock cycle and calls initEvents again:
    // the listener registers once, but ticks must drive the LATEST callback.
    s.initEvents(secondMount)
    handlers['momentum:tick']?.()
    expect(firstMount).not.toHaveBeenCalled()
    expect(secondMount).toHaveBeenCalledTimes(1)
  })

  it('initEvents hydrates status by pull (connect-time push may predate the listener)', async () => {
    const s = useNodeStore()
    expect(s.chainId).toBe(0)
    s.initEvents(vi.fn())
    await new Promise((r) => setTimeout(r))
    expect(s.chainId).toBe(3)
    expect(s.connected).toBe(true)
    expect(s.height).toBe(42)
  })

  it('a push landing during the pull wins (stale snapshot discarded)', async () => {
    const s = useNodeStore()
    s.initEvents(vi.fn()) // pull (chainId 3) starts…
    // …but a fresher push (mainnet) lands before the RPC resolves:
    handlers['node:status']?.({ connected: true, chainId: 1, height: 50, mode: 'remote' })
    await new Promise((r) => setTimeout(r))
    expect(s.chainId).toBe(1) // stale pull must not re-open the governance gate
    expect(s.height).toBe(50)
  })

  it('the pull never clobbers syncing (owned by node:sync)', async () => {
    const s = useNodeStore()
    s.initEvents(vi.fn())
    await new Promise((r) => setTimeout(r))
    handlers['node:sync']?.({ state: 'syncing' })
    expect(s.syncing).toBe(true)
    await s.refreshStatus() // snapshot DTO carries no meaningful syncing value
    expect(s.syncing).toBe(true)
  })

  it('clearTick detaches the callback (no refreshes while locked)', () => {
    const s = useNodeStore()
    const cb = vi.fn()
    s.initEvents(cb)
    s.clearTick() // AppShell unmounts on lock
    handlers['momentum:tick']?.()
    expect(cb).not.toHaveBeenCalled()
  })
})
