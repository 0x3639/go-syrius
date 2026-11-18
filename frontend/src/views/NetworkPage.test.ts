import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, enableAutoUnmount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

// NetworkPage resolves its panel via useRoute() (inject-based), so a $route
// global.mock won't satisfy it — mock the composable directly, matching the
// repo's established vue-router test pattern. The state is mutable so each
// test can pick its panel.
const routeState = vi.hoisted(() => ({ meta: { panel: 'plasma' } as Record<string, string>, query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => routeState }))

import NetworkPage from './NetworkPage.vue'
import { useNodeStore } from '../stores/node'

enableAutoUnmount(afterEach)

const stubs = {
  PlasmaPanel: { data: () => ({ draft: '' }), template: '<div class="plasma-stub"><input v-model="draft" /></div>' },
}

describe('NetworkPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    useNodeStore().connected = true
    routeState.meta.panel = 'plasma'
  })

  it('renders the panel named by route meta', () => {
    const w = mount(NetworkPage, { attachTo: document.body, global: { stubs } })
    expect(w.find('.plasma-stub').exists()).toBe(true)
  })

  it('preserves the mounted draft through disconnect and reconnect', async () => {
    const node = useNodeStore()
    const w = mount(NetworkPage, { attachTo: document.body, global: { stubs } })
    await w.get('input').setValue('12 QSR draft')
    node.connected = false
    await w.vm.$nextTick()
    expect(w.find('.plasma-stub').isVisible()).toBe(false)
    expect(w.text()).toContain('Reconnect')
    node.connected = true
    await w.vm.$nextTick()
    expect(w.find('.plasma-stub').isVisible()).toBe(true)
    expect((w.get('input').element as HTMLInputElement).value).toBe('12 QSR draft')
  })
})
