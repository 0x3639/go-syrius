import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSentinelStore } from './sentinel'

// Don't touch the (unmocked) backend; refresh is stubbed per test.
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetSentinel: vi.fn(), GetDepositedQsr: vi.fn(), GetSentinelReward: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('sentinel store pending/poll', () => {
  it('beginPending(deposit) clears once deposited reaches 50,000 QSR', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    vi.spyOn(s, 'refresh').mockImplementation(async () => { s.depositedQsr = '5000000000000' })
    s.beginPending('deposit')
    expect(s.pendingStep).toBe('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('keeps polling (pendingStep stays) until the chain reflects the step', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    let credited = false
    vi.spyOn(s, 'refresh').mockImplementation(async () => { if (credited) s.depositedQsr = '5000000000000' })
    s.beginPending('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe('deposit')
    expect(s.pollCount).toBe(1)
    credited = true
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(register) clears once the sentinel is active', async () => {
    vi.useFakeTimers()
    const s = useSentinelStore()
    vi.spyOn(s, 'refresh').mockImplementation(async () => { s.sentinel = { owner: 'z1own', active: true } as never })
    s.beginPending('register')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('stopPolling clears the pending state', () => {
    const s = useSentinelStore()
    s.beginPending('deposit')
    s.stopPolling()
    expect(s.pendingStep).toBe(null)
    expect(s.pollCount).toBe(0)
  })
})
