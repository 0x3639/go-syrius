import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const CancelPending = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/TxService', () => ({ CancelPending }))

import router from './index'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
import { useUiStore } from '../stores/ui'

describe('router guard', () => {
  // Start each test from '/import' — a public route distinct from every route a
  // test pushes ('/dashboard', '/unlock', '/network/plasma'). Landing the
  // singleton router on the same route a test pushes would make push() a
  // duplicate no-op that skips the guard, so the neutral start is load-bearing.
  beforeEach(async () => {
    setActivePinia(createPinia())
    CancelPending.mockClear()
    await router.replace('/import').catch(() => {})
  })

  it('redirects to /unlock when locked and visiting a gated route', async () => {
    useWalletStore().locked = true
    await router.push('/dashboard').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/unlock')
  })

  it('redirects to /dashboard when unlocked and visiting a public route', async () => {
    useWalletStore().locked = false
    await router.push('/unlock').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('opening an operator tool from Everyday switches to Advanced and says so', async () => {
    useWalletStore().locked = false
    const ui = useUiStore(); ui.mode = 'everyday'
    await router.push('/network/pillars').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/network/pillars')
    expect(ui.mode).toBe('advanced')
    expect(ui.modeNotice).toContain('Pillars')
    localStorage.removeItem('syrius.mode')
  })

  it('shared tools open in Everyday without changing mode', async () => {
    useWalletStore().locked = false
    const ui = useUiStore(); ui.mode = 'everyday'
    for (const p of ['/network/plasma', '/earn', '/tools', '/activity', '/walletconnect']) {
      await router.push(p).catch(() => {})
      expect(router.currentRoute.value.path).toBe(p)
    }
    expect(ui.mode).toBe('everyday')
  })

  it('Everyday-only screens fall back to the overview in Advanced', async () => {
    useWalletStore().locked = false
    useUiStore().mode = 'advanced'
    await router.push('/earn').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('allows a gated route when unlocked', async () => {
    useWalletStore().locked = false
    await router.push('/network/plasma').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/network/plasma')
  })

  it('navigation discards a retryable error hold instead of orphaning it', async () => {
    useWalletStore().locked = false
    const tx = useTxStore()
    tx.status = 'error'
    tx.error = 'not connected'
    tx.preview = { holdId: 55 } as any

    await router.push('/dashboard').catch(() => {})

    expect(tx.status).toBe('idle')
    expect(CancelPending).toHaveBeenCalledWith(55)
  })
})
