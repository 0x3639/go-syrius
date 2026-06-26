import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const push = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push, currentRoute: { value: { name: 'home' } } }),
  RouterView: { template: '<div />' },
}))
vi.mock('nom-ui', () => ({
  useTheme: () => ({ setTheme: vi.fn() }),
  Toaster: { template: '<div />' },
}))
vi.mock('../wailsjs/go/app/NodeService', () => ({ Connect: vi.fn().mockResolvedValue(undefined) }))

import App from './App.vue'
import { useWalletStore } from './stores/wallet'

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
})

describe('App — lock leaves the protected UI', () => {
  it('redirects to /unlock when the wallet locks while on a gated route', async () => {
    const wallet = useWalletStore()
    wallet.locked = false // unlocked, on /home
    const w = mount(App)
    wallet.locked = true // lock (e.g. the Lock button)
    await w.vm.$nextTick()
    expect(push).toHaveBeenCalledWith('/unlock')
  })
})
