import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'

function mountSidebar() {
  return mount(Sidebar, { global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the core nav destinations', () => {
    const w = mountSidebar()
    const text = w.text()
    for (const label of ['Dashboard', 'Transfer', 'Receive', 'Tokens', 'Plasma', 'Staking', 'Pillars', 'Sentinels', 'Accelerator', 'Rewards', 'Settings']) {
      expect(text).toContain(label)
    }
  })

  it('hides Governance unless opted in on testnet', async () => {
    const w = mountSidebar()
    expect(w.text()).not.toContain('Governance')
    const ui = useUiStore(); const node = useNodeStore()
    ui.showGovernance = true; node.chainId = 2
    await w.vm.$nextTick()
    expect(w.text()).toContain('Governance')
  })

  it('shows the node-sync height when connected', async () => {
    const node = useNodeStore()
    node.connected = true; node.syncing = false; node.height = 3_420_000
    const w = mountSidebar()
    await w.vm.$nextTick()
    expect(w.text()).toContain('3,420,000')
  })
})
