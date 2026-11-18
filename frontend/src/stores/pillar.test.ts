import { useNodeStore } from './node'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { usePillarStore, PILLAR_PLASMA_REQUIRED } from './pillar'

// Don't touch the (unmocked) backend; refreshRegistration is stubbed per test.
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetMyPillar: vi.fn(), GetPillarDepositedQsr: vi.fn(), GetPillarQsrCost: vi.fn(),
  GetPlasmaInfo: vi.fn(), GetPillarReward: vi.fn(),
  GetPillarList: vi.fn(), GetDelegation: vi.fn(),
}))

beforeEach(() => { setActivePinia(createPinia()); useNodeStore().connected = true })

describe('pillar store plasmaCleared', () => {
  it('clears on sufficient current plasma', () => {
    const s = usePillarStore()
    s.plasma = { currentPlasma: Number(PILLAR_PLASMA_REQUIRED), maxPlasma: 0, qsrFused: '0' } as never
    expect(s.plasmaCleared).toBe(true)
  })
  it('clears on sufficient fused capacity even when current plasma is momentarily low', () => {
    const s = usePillarStore()
    s.plasma = { currentPlasma: 0, maxPlasma: Number(PILLAR_PLASMA_REQUIRED), qsrFused: '50000000000' } as never
    expect(s.plasmaCleared).toBe(true)
  })
  it('does not clear when neither current nor max plasma is sufficient', () => {
    const s = usePillarStore()
    s.plasma = { currentPlasma: 100, maxPlasma: 100, qsrFused: '0' } as never
    expect(s.plasmaCleared).toBe(false)
  })
})

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

it('pauses settlement polling offline and resumes without losing the pending step', async () => {
  vi.useFakeTimers()
  const s = usePillarStore()
  const refresh = vi.spyOn(s, 'refreshRegistration').mockResolvedValue()
  try {
    s.beginPending('deposit')
    useNodeStore().connected = false
    await vi.advanceTimersByTimeAsync(21000)
    expect(refresh).not.toHaveBeenCalled()
    expect(s.pollCount).toBe(0)
    expect(s.pendingStep).toBe('deposit')
    useNodeStore().connected = true
    await vi.advanceTimersByTimeAsync(3000)
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(s.pollCount).toBe(1)
  } finally { s.stopPolling(); vi.useRealTimers() }
})
