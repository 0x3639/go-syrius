import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

// NetworkPage resolves its panel via useRoute() (inject-based), so a $route
// global.mock won't satisfy it — mock the composable directly, matching the
// repo's established vue-router test pattern. The state is mutable so each
// test can pick its panel.
const routeState = vi.hoisted(() => ({ meta: { panel: 'plasma' } as Record<string, string>, query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => routeState }))

import NetworkPage from './NetworkPage.vue'
import { useNodeStore } from '../stores/node'

const stubs = {
  PlasmaPanel: { template: '<div class="plasma-stub"/>' },
}

describe('NetworkPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useNodeStore().connected = true
    routeState.meta.panel = 'plasma'
  })

  it('renders the panel named by route meta', () => {
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.plasma-stub').exists()).toBe(true)
  })

  it('hides stale network data and offers reconnection while offline', () => {
    useNodeStore().connected = false
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.plasma-stub').exists()).toBe(false)
    expect(w.text()).toContain('Disconnected')
    expect(w.text()).toContain('Reconnect')
  })

})
