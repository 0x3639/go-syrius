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
  it.each(['receive', 'receiveAll'] as const)('drops an obsolete %s failure after invalidation', async (action) => {
    const s = useUnreceivedStore()
    await s.load()
    let reject!: (e: Error) => void
    Receive.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail }))
    const receiving = action === 'receive' ? s.receive('h1') : s.receiveAll()
    s.invalidate()
    reject(new Error('old connection error'))
    await receiving
    expect(s.error).toBe('')
    expect(s.busyAll).toBe(false)
    expect(s.busy).toEqual({})
  })

  it('stops the undispatched receive batch after invalidation', async () => {
    const s = useUnreceivedStore()
    s.items = [{ fromHash: 'h1' }, { fromHash: 'h2' }] as any
    let finish!: () => void
    Receive.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }))
    const receiving = s.receiveAll()
    s.invalidate()
    finish()
    await receiving
    expect(Receive).toHaveBeenCalledTimes(1)
    expect(Receive).toHaveBeenCalledWith('h1')
  })
  it('clears pending rows and rejects an old connection response after invalidation', async () => {
    const s = useUnreceivedStore()
    await s.load()
    let finish!: (r: unknown) => void
    GetUnreceived.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const loading = s.load()
    s.invalidate()
    expect(s.items).toEqual([])
    finish([{ fromHash: 'old-node' }])
    await loading
    expect(s.items).toEqual([])
    await s.load()
    expect(s.items[0].fromHash).toBe('h1')
  })
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
