import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useUiStore } from '../stores/ui'
import { useNodeStore } from '../stores/node'
import { useTxStore } from '../stores/tx'

// NetworkPage resolves its panel via useRoute() (inject-based), so a $route
// global.mock won't satisfy it — mock the composable directly, matching the
// repo's established vue-router test pattern. The state is mutable so each
// test can pick its panel.
const routeState = vi.hoisted(() => ({ meta: { panel: 'plasma' } as Record<string, string>, query: {} as Record<string, string> }))
vi.mock('vue-router', () => ({ useRoute: () => routeState }))

import NetworkPage from './NetworkPage.vue'

const stubs = {
  PlasmaPanel: { template: '<div class="plasma-stub"/>' },
  GovernancePanel: { template: '<div class="gov-stub"/>' },
}

describe('NetworkPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    routeState.meta.panel = 'plasma'
  })

  it('renders the panel named by route meta', () => {
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.plasma-stub').exists()).toBe(true)
  })

  // The frontend half of the testnet-only Governance rule: the PANEL itself is
  // gated reactively (not just the Sidebar link), so a node flipping to mainnet
  // while the route is open removes the interactive UI immediately.
  it('blocks Governance on mainnet (chainId 1) even when opted in', async () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
    const node = useNodeStore(); node.chainId = 1
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.gov-stub').exists()).toBe(false)
    expect(w.text()).toContain('testnet-only')

    // …and reappears reactively when the node is a testnet again.
    node.chainId = 2
    await w.vm.$nextTick()
    expect(w.find('.gov-stub').exists()).toBe(true)
  })

  it('blocks Governance when the Settings opt-in is off', () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.showGovernance = false
    const node = useNodeStore(); node.chainId = 2
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.gov-stub').exists()).toBe(false)
  })

  it('fails closed while chainId is unknown (0, pre-connect)', () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
    const node = useNodeStore(); node.chainId = 0
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.gov-stub').exists()).toBe(false)
  })

  it('discards a prepared (awaiting) tx when the gate slams shut', async () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
    const node = useNodeStore(); node.chainId = 2
    const tx = useTxStore()
    tx.status = 'awaiting' // a built governance block is held, dialog open
    // discard() leaves 'awaiting' synchronously, then releases the held block.
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const w = mount(NetworkPage, { global: { stubs } })
    expect(w.find('.gov-stub').exists()).toBe(true)

    node.chainId = 1 // node reconnects to mainnet mid-flow
    await w.vm.$nextTick()
    expect(w.find('.gov-stub').exists()).toBe(false)
    expect(discard).toHaveBeenCalled()
  })

  it('discards a Prepare that resolves AFTER the gate has closed', async () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
    const node = useNodeStore(); node.chainId = 2
    const tx = useTxStore()
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const w = mount(NetworkPage, { global: { stubs } })

    node.chainId = 1 // gate slams while a Prepare RPC is still in flight
    await w.vm.$nextTick()
    expect(discard).not.toHaveBeenCalled() // nothing to discard yet

    tx.status = 'awaiting' // the late Prepare resolves
    await w.vm.$nextTick()
    expect(discard).toHaveBeenCalled() // …and is discarded before any dialog frame
  })

  it('does not disturb a tx already publishing when the gate closes', async () => {
    routeState.meta.panel = 'governance'
    const ui = useUiStore(); ui.governanceFeatureEnabled = true; ui.showGovernance = true
    const node = useNodeStore(); node.chainId = 2
    const tx = useTxStore()
    tx.status = 'publishing' // ConfirmPublish already in flight
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const reset = vi.spyOn(tx, 'reset')
    const w = mount(NetworkPage, { global: { stubs } })

    node.chainId = 1
    await w.vm.$nextTick()
    expect(discard).not.toHaveBeenCalled()
    expect(reset).not.toHaveBeenCalled()
  })
})
