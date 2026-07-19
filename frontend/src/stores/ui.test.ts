import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetSettings, SetShowGovernance, IsGovernanceFeatureEnabled } = vi.hoisted(() => ({
  GetSettings: vi.fn(),
  SetShowGovernance: vi.fn(),
  IsGovernanceFeatureEnabled: vi.fn(),
}))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetShowGovernance, IsGovernanceFeatureEnabled }))

import { useUiStore } from './ui'
import { useNodeStore } from './node'

describe('ui store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    GetSettings.mockReset()
    SetShowGovernance.mockReset()
    IsGovernanceFeatureEnabled.mockReset()
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

  // The store owns the preference only; the DOM class is applied by App.vue's
  // theme watch (nom-ui setTheme) — the single applier.
  it('initTheme restores a persisted light theme (sync, pre-mount safe)', () => {
    localStorage.setItem('syrius.theme', 'light')
    try {
      const s = useUiStore()
      s.initTheme()
      expect(s.theme).toBe('light')
    } finally {
      localStorage.removeItem('syrius.theme')
    }
  })

  it('toggleTheme flips and persists the preference', () => {
    const s = useUiStore()
    expect(s.theme).toBe('dark')
    s.toggleTheme()
    expect(s.theme).toBe('light')
    expect(localStorage.getItem('syrius.theme')).toBe('light')
    localStorage.removeItem('syrius.theme')
  })

  it('setShowGovernance flips state and persists the merged settings', async () => {
    GetSettings.mockResolvedValue({ showGovernance: false })
    const s = useUiStore()
    await s.setShowGovernance(true)
    expect(s.showGovernance).toBe(true)
    expect(SetShowGovernance).toHaveBeenCalledWith(true)
  })

  // TEMPORARY kill switch: governance is fully disabled pending an SDK update.
  it('governanceAllowed is false while the feature flag is off, even opted-in on testnet', () => {
    const s = useUiStore()
    s.showGovernance = true
    useNodeStore().chainId = 2
    expect(s.governanceAllowed).toBe(false)
  })

  it('governanceAllowed requires flag + opt-in + testnet', () => {
    const s = useUiStore()
    s.governanceFeatureEnabled = true
    s.showGovernance = true
    useNodeStore().chainId = 2
    expect(s.governanceAllowed).toBe(true)
  })

  it('init loads the kill-switch flag from the binding (fail-closed)', async () => {
    GetSettings.mockResolvedValue({})
    IsGovernanceFeatureEnabled.mockResolvedValue(true)
    const s = useUiStore()
    await s.init()
    expect(s.governanceFeatureEnabled).toBe(true)
  })

  it('init keeps the flag false when the binding fails (fail-closed)', async () => {
    GetSettings.mockResolvedValue({})
    IsGovernanceFeatureEnabled.mockRejectedValue(new Error('locked'))
    const s = useUiStore()
    await s.init()
    expect(s.governanceFeatureEnabled).toBe(false)
  })
})
