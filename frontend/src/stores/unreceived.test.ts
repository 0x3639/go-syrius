// stores/unreceived.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetUnreceived, Receive } = vi.hoisted(() => ({
  GetUnreceived: vi.fn().mockResolvedValue([
    { fromHash: 'h1', fromAddress: 'z1aaa', token: 'ZNN', amount: '100000000' },
  ]),
  Receive: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetUnreceived }))
vi.mock('../../wailsjs/go/app/TxService', () => ({ Receive }))

import { useUnreceivedStore } from './unreceived'

beforeEach(() => {
  setActivePinia(createPinia())
  GetUnreceived.mockClear()
  Receive.mockClear()
  Receive.mockResolvedValue(undefined)
})

describe('unreceived store', () => {
  it('load sets items from GetUnreceived', async () => {
    const s = useUnreceivedStore()
    await s.load()
    expect(s.items[0].fromHash).toBe('h1')
  })

  it('receive calls TxService.Receive(hash) and clears busy', async () => {
    const s = useUnreceivedStore()
    await s.receive('h1')
    expect(Receive).toHaveBeenCalledWith('h1')
    expect(s.busy['h1']).toBeUndefined()
    expect(s.error).toBe('')
  })

  it('receive surfaces a rejected Receive to error', async () => {
    Receive.mockRejectedValueOnce(new Error('boom'))
    const s = useUnreceivedStore()
    await s.receive('h2')
    expect(s.error).toBe('boom')
    expect(s.busy['h2']).toBeUndefined()
  })
})
