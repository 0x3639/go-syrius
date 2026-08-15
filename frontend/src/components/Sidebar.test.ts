import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'
import { vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

const GetBuildInfo = vi.hoisted(() => vi.fn().mockResolvedValue({ version: 'v9.9.9', commit: 'abc1234' }))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetBuildInfo }))

function mountSidebar() {
  return mount(Sidebar, { global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the core nav destinations', () => {
    const w = mountSidebar()
    const text = w.text()
    for (const label of ['Dashboard', 'Transfer', 'Receive', 'Tokens', 'Plasma', 'Staking', 'Pillars', 'Sentinels', 'Accelerator', 'Rewards', 'WalletConnect', 'Settings']) {
      expect(text).toContain(label)
    }
  })

  it('hides Governance unless the feature flag, opt-in, and testnet all hold', async () => {
    const w = mountSidebar()
    expect(w.text()).not.toContain('Governance')
    const ui = useUiStore(); const node = useNodeStore()
    ui.showGovernance = true; node.chainId = 2
    await w.vm.$nextTick()
    // kill switch off → still hidden even when opted in on testnet
    expect(w.text()).not.toContain('Governance')
    ui.governanceFeatureEnabled = true
    await w.vm.$nextTick()
    expect(w.text()).toContain('Governance')
  })

  it('shows the app version below the sync pill', async () => {
    const w = mountSidebar()
    await flushPromises()
    expect(w.text()).toContain('v9.9.9')
    // Version only in the sidebar — the commit lives in Settings.
    expect(w.text()).not.toContain('abc1234')
  })

  it('shows the node-sync height when connected', async () => {
    const node = useNodeStore()
    node.connected = true; node.syncing = false; node.height = 3_420_000
    const w = mountSidebar()
    await w.vm.$nextTick()
    expect(w.text()).toContain('3,420,000')
  })
})
