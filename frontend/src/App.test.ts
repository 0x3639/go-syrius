import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push, currentRoute: { value: { name: 'dashboard' } } }),
  RouterView: { template: '<div />' },
}))
vi.mock('nom-ui', () => ({
  useTheme: () => ({ setTheme: vi.fn() }),
  Toaster: { template: '<div />' },
}))
vi.mock('../wailsjs/go/app/NodeService', () => ({ Connect: vi.fn().mockResolvedValue(undefined) }))
// Stub the splash so jsdom never pulls lottie-web in; assert on its presence.
vi.mock('./components/IntroSplash.vue', () => ({
  default: { name: 'IntroSplash', emits: ['done'], template: '<div data-test="intro" />' },
}))

import App from './App.vue'
import { useWalletStore } from './stores/wallet'

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
  localStorage.clear()
})

describe('App — lock leaves the protected UI', () => {
  it('redirects to /unlock when the wallet locks while on a gated route', async () => {
    const wallet = useWalletStore()
    wallet.locked = false // unlocked, on /dashboard
    const w = mount(App)
    wallet.locked = true // lock (e.g. the Lock button)
    await w.vm.$nextTick()
    expect(push).toHaveBeenCalledWith('/unlock')
  })
})

describe('App — intro splash', () => {
  it('shows the intro splash on launch', () => {
    useWalletStore().locked = true
    const w = mount(App)
    expect(w.find('[data-test="intro"]').exists()).toBe(true)
  })

  it('shows the intro splash on every launch, even if the old seen flag is set', () => {
    localStorage.setItem('zn:introSeen', '1') // legacy flag must no longer suppress it
    useWalletStore().locked = true
    const w = mount(App)
    expect(w.find('[data-test="intro"]').exists()).toBe(true)
  })

  it('removes the splash on done', async () => {
    useWalletStore().locked = true
    const w = mount(App)
    w.findComponent({ name: 'IntroSplash' }).vm.$emit('done')
    await w.vm.$nextTick()
    expect(w.find('[data-test="intro"]').exists()).toBe(false)
  })
})
