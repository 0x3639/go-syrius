import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
const { GetPlasmaInfo, GetFusionEntries } = vi.hoisted(() => ({
  GetPlasmaInfo: vi.fn(), GetFusionEntries: vi.fn().mockResolvedValue([]),
}))
vi.mock('../../wailsjs/go/app/NomService', () => ({ GetPlasmaInfo, GetFusionEntries }))
import { usePlasmaStore } from './plasma'
import { bumpRequestEpoch } from '../lib/requestEpoch'

beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks() })
describe('plasma snapshots', () => {
  it.each(['account', 'connection'])('rejects a pending snapshot after an %s change', async (context) => {
    const s = usePlasmaStore()
    s.info = { currentPlasma: 100 } as any
    let finish!: (r: unknown) => void
    GetPlasmaInfo.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const refreshing = s.refresh()
    if (context === 'account') bumpRequestEpoch()
    s.invalidate()
    expect(s.info).toBeNull()
    finish({ currentPlasma: 999 })
    await refreshing
    expect(s.info).toBeNull()
    expect(GetFusionEntries).not.toHaveBeenCalled()
    GetPlasmaInfo.mockResolvedValueOnce({ currentPlasma: 42 })
    await s.refresh()
    expect(s.info?.currentPlasma).toBe(42)
  })

  it('clears the previous level on a failed refresh', async () => {
    const s = usePlasmaStore()
    s.info = { currentPlasma: 100 } as any
    GetPlasmaInfo.mockRejectedValueOnce(new Error('offline'))
    await s.refresh()
    expect(s.info).toBeNull()
  })
})
