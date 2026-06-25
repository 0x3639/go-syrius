import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWalletStore } from '../stores/wallet'

// Stub the lazy-loaded views so navigation in the guard test doesn't pull the
// real nom-ui components into jsdom — we're testing the guard, not the screens.
vi.mock('../views/Unlock.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/Create.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/ImportMnemonic.vue', () => ({ default: { template: '<div/>' } }))
vi.mock('../views/Home.vue', () => ({ default: { template: '<div/>' } }))
import router, { PUBLIC_ROUTES } from './index'

beforeEach(async () => {
  setActivePinia(createPinia())
  // The router is a module-level singleton whose location persists across
  // tests. Force it back to a neutral starting point so a prior test's landing
  // route can't turn a test's push() into a duplicate no-op that skips the
  // guard. '/import' is public, so while locked the guard leaves it in place —
  // a start state that differs from both tests' landing routes ('/unlock',
  // '/home'), guaranteeing each test's push() actually re-runs the guard.
  useWalletStore().locked = true
  await router.push('/import')
})

describe('router lock guard', () => {
  it('redirects a locked wallet away from gated routes to unlock', async () => {
    useWalletStore().locked = true
    await router.push('/home')
    expect(router.currentRoute.value.name).toBe('unlock')
  })
  it('redirects an unlocked wallet away from public routes to home', async () => {
    useWalletStore().locked = false
    await router.push('/unlock')
    expect(router.currentRoute.value.name).toBe('home')
  })
  it('lists the public routes', () => {
    expect(PUBLIC_ROUTES).toEqual(['unlock', 'create', 'import'])
  })
})
