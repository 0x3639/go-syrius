// stores/node.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
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
})
