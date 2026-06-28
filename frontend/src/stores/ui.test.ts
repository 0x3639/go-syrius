import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetSettings, SetSettings } = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SetSettings: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetSettings }))

import { useUiStore } from './ui'

describe('ui store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    GetSettings.mockReset()
    SetSettings.mockReset()
  })

  it('defaults showGovernance to false', () => {
    expect(useUiStore().showGovernance).toBe(false)
  })

  it('init loads showGovernance from settings', async () => {
    GetSettings.mockResolvedValue({ showGovernance: true })
    const s = useUiStore()
    await s.init()
    expect(s.showGovernance).toBe(true)
  })

  it('init swallows errors and keeps the default (offline/locked)', async () => {
    GetSettings.mockRejectedValue(new Error('locked'))
    const s = useUiStore()
    await s.init()
    expect(s.showGovernance).toBe(false)
  })

  it('setShowGovernance flips state and persists the merged settings', async () => {
    GetSettings.mockResolvedValue({ showGovernance: false })
    const s = useUiStore()
    await s.setShowGovernance(true)
    expect(s.showGovernance).toBe(true)
    expect(SetSettings).toHaveBeenCalledWith({ showGovernance: true })
  })
})
