import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

// NetworkPage resolves its panel via useRoute() (inject-based), so a $route
// global.mock won't satisfy it — mock the composable directly, matching the
// repo's established vue-router test pattern.
vi.mock('vue-router', () => ({ useRoute: () => ({ meta: { panel: 'plasma' }, query: {} }) }))

import NetworkPage from './NetworkPage.vue'

describe('NetworkPage', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('renders the panel named by route meta', () => {
    const w = mount(NetworkPage, {
      global: {
        stubs: { PlasmaPanel: { template: '<div class="plasma-stub"/>' } },
      },
    })
    expect(w.find('.plasma-stub').exists()).toBe(true)
  })
})
