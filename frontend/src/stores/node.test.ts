// stores/node.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
}))
import { useNodeStore } from './node'
beforeEach(() => setActivePinia(createPinia()))
describe('node store', () => {
  it('connects and loads balances', async () => {
    const s = useNodeStore()
    await s.connect(); expect(s.connected).toBe(true)
    await s.loadBalances(); expect(s.balances[0].symbol).toBe('ZNN')
  })
})
