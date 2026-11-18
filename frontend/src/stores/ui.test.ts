import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

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

})
