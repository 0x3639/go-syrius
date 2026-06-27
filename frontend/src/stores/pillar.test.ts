import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePillarStore, PILLAR_PLASMA_REQUIRED } from './pillar'

// Don't touch the (unmocked) backend; refreshRegistration is stubbed per test.
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetMyPillar: vi.fn(), GetPillarDepositedQsr: vi.fn(), GetPillarQsrCost: vi.fn(),
  GetPlasmaInfo: vi.fn(), GetPillarReward: vi.fn(),
  GetPillarList: vi.fn(), GetDelegation: vi.fn(),
}))

beforeEach(() => setActivePinia(createPinia()))

describe('pillar store registration pending/poll', () => {
  it('beginPending(plasma) clears once plasma reaches the requirement', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => {
      s.plasma = { currentPlasma: Number(PILLAR_PLASMA_REQUIRED), maxPlasma: 0, qsrFused: '0' } as never
    })
    s.beginPending('plasma')
    expect(s.pendingStep).toBe('plasma')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(deposit) clears once deposited reaches the cost', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    s.qsrCost = '15000000000000'
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => { s.depositedQsr = '15000000000000' })
    s.beginPending('deposit')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('beginPending(register) clears once a pillar is owned', async () => {
    vi.useFakeTimers()
    const s = usePillarStore()
    vi.spyOn(s, 'refreshRegistration').mockImplementation(async () => { s.myPillar = { name: 'Pillar-A' } as never })
    s.beginPending('register')
    await vi.advanceTimersByTimeAsync(3000)
    expect(s.pendingStep).toBe(null)
    vi.useRealTimers()
  })

  it('stopPolling clears the pending state', () => {
    const s = usePillarStore()
    s.beginPending('deposit')
    s.stopPolling()
    expect(s.pendingStep).toBe(null)
    expect(s.pollCount).toBe(0)
  })
})
