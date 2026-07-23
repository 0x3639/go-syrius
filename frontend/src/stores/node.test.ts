// stores/node.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
const NodeStatus = vi.hoisted(() => vi.fn().mockResolvedValue({ mode: 'remote', connected: true, syncing: false, height: 42, peers: 3, chainId: 3 }))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
  NodeStatus,
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

  it('a mode switch clears stale sync state (embedded mid-sync → remote)', async () => {
    const s = useNodeStore()
    s.initEvents(vi.fn())
    await new Promise((r) => setTimeout(r))
    handlers['node:status']?.({ connected: true, chainId: 3, mode: 'embedded' })
    handlers['node:sync']?.({ state: 'syncing' })
    expect(s.syncing).toBe(true)
    // Only the embedded node runs the sync poller — switching modes must not
    // leave "Syncing…" stuck for the rest of the session.
    handlers['node:status']?.({ connected: true, chainId: 3, mode: 'remote' })
    expect(s.syncing).toBe(false)
    expect(s.sync).toBeNull()
  })

  it('the pull never clobbers syncing (owned by node:sync)', async () => {
    const s = useNodeStore()
    s.initEvents(vi.fn())
    await new Promise((r) => setTimeout(r))
    handlers['node:status']?.({ connected: true, chainId: 3, mode: 'embedded' })
    handlers['node:sync']?.({ state: 'syncing' })
    expect(s.syncing).toBe(true)
    // A pull consistent with the current mode must not touch syncing (the
    // snapshot DTO carries no meaningful syncing value).
    NodeStatus.mockResolvedValueOnce({ mode: 'embedded', connected: true, height: 43, peers: 3, chainId: 3 })
    await s.refreshStatus()
    expect(s.syncing).toBe(true)
  })

  it('drops straggler node:sync events outside embedded mode', async () => {
    const s = useNodeStore()
    s.initEvents(vi.fn())
    await new Promise((r) => setTimeout(r))
    expect(s.mode).toBe('remote')
    // A dying embedded poller can emit one last sync after the mode switch —
    // it must not re-stick "Syncing…".
    handlers['node:sync']?.({ state: 'syncing' })
    expect(s.syncing).toBe(false)
  })

  it('pauses the tick refresh during embedded bulk sync', async () => {
    const s = useNodeStore()
    const cb = vi.fn()
    s.initEvents(cb)
    await new Promise((r) => setTimeout(r))
    handlers['node:status']?.({ connected: true, chainId: 1, mode: 'embedded' })
    handlers['node:sync']?.({ state: 'syncing' })
    // Ticks arrive continuously during bulk sync; refreshing seven RPC groups
    // per tick would compete with block insertion for the node's CPU and DB.
    handlers['momentum:tick']?.()
    expect(cb).not.toHaveBeenCalled()
    // Sync done → the tick refresh resumes.
    handlers['node:sync']?.({ state: 'synced' })
    handlers['momentum:tick']?.()
    expect(cb).toHaveBeenCalledTimes(1)
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
