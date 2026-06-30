import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

// Mock the price store so AppShell's onMounted price.start() runs NO real fetch
// and sets NO 60s interval. Keeps output pristine (no jsdom fetch warning, no
// leaked timer bleeding into other specs).
vi.mock('../stores/price', () => ({
  usePriceStore: () => ({ start: vi.fn(), stop: vi.fn() }),
}))

// Stub vue-router: useRoute() supplies meta.title (an inject-based composable
// global.mocks does NOT satisfy); useRouter() keeps TopBar/Sidebar mountable.
vi.mock('vue-router', () => ({
  useRoute: () => ({ meta: { title: 'Dashboard' }, path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))

import AppShell from './AppShell.vue'

describe('AppShell', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the sidebar, a topbar title from route meta, and a router-view outlet', () => {
    const w = mount(AppShell, {
      global: {
        stubs: {
          RouterLink: RouterLinkStub,
          RouterView: { template: '<div class="rv-stub">page</div>' },
          AccountSlotPicker: true,
        },
      },
    })
    expect(w.find('aside').exists()).toBe(true)
    expect(w.find('header').text()).toContain('Dashboard')
    expect(w.find('.rv-stub').exists()).toBe(true)
  })
})
