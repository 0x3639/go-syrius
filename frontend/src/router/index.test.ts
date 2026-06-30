import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import router from './index'
import { useWalletStore } from '../stores/wallet'

describe('router guard', () => {
  // Start each test from '/import' — a public route distinct from every route a
  // test pushes ('/dashboard', '/unlock', '/network/plasma'). Landing the
  // singleton router on the same route a test pushes would make push() a
  // duplicate no-op that skips the guard, so the neutral start is load-bearing.
  beforeEach(async () => { setActivePinia(createPinia()); await router.replace('/import').catch(() => {}) })

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

  it('allows a gated route when unlocked', async () => {
    useWalletStore().locked = false
    await router.push('/network/plasma').catch(() => {})
    expect(router.currentRoute.value.path).toBe('/network/plasma')
  })
})
