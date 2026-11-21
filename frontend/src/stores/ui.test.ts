import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const GetSettings = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetUIMode: vi.fn().mockResolvedValue(undefined), SetPinnedActions: vi.fn().mockResolvedValue(undefined) }))

import { useUiStore } from './ui'

describe('ui store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
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


  it('starts in Advanced until a preference is known, then restores the cached mode', () => {
    const s = useUiStore()
    expect(s.mode).toBe('advanced')
    localStorage.setItem('syrius.mode', 'everyday')
    try { s.initMode(); expect(s.mode).toBe('everyday') } finally { localStorage.removeItem('syrius.mode') }
  })

  it('ignores an unknown cached mode', () => {
    localStorage.setItem('syrius.mode', 'expert')
    try { const s = useUiStore(); s.initMode(); expect(s.mode).toBe('advanced') } finally { localStorage.removeItem('syrius.mode') }
  })

  it('setMode switches at once, caches it and survives a failed save', () => {
    const s = useUiStore()
    s.setMode('everyday', 'note')
    expect(s.mode).toBe('everyday')
    expect(s.modeNotice).toBe('note')
    expect(localStorage.getItem('syrius.mode')).toBe('everyday')
    localStorage.removeItem('syrius.mode')
  })

  it('togglePin adds, removes and stops at the limit', () => {
    const s = useUiStore()
    s.pins = []
    for (const id of ['fuse', 'collect', 'receive', 'vote', 'send', 'stake'] as const) s.togglePin(id)
    expect(s.pins).toHaveLength(6)
    s.togglePin('node')
    expect(s.pins).not.toContain('node')
    s.togglePin('fuse')
    expect(s.pins).not.toContain('fuse')
  })

  it('loadPrefs applies saved mode and pins', async () => {
    GetSettings.mockResolvedValue({ uiMode: 'everyday', pinnedActions: ['vote', 'bogus'] })
    const s = useUiStore()
    await s.loadPrefs()
    expect(s.mode).toBe('everyday')
    expect(s.pins).toEqual(['vote'])
    localStorage.removeItem('syrius.mode')
  })

  it('a settings load that started earlier does not undo a newer change', async () => {
    let resolve!: (v: unknown) => void
    GetSettings.mockReturnValue(new Promise((r) => { resolve = r }))
    const s = useUiStore()
    const pending = s.loadPrefs()
    s.setMode('everyday')
    s.togglePin('vote')
    resolve({ uiMode: 'advanced', pinnedActions: ['fuse'] })
    await pending
    expect(s.mode).toBe('everyday')
    expect(s.pins).toContain('vote')
    expect(localStorage.getItem('syrius.mode')).toBe('everyday')
    localStorage.removeItem('syrius.mode')
  })
})
