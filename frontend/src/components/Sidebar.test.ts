import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useNodeStore } from '../stores/node'
import { vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

const GetBuildInfo = vi.hoisted(() => vi.fn().mockResolvedValue({ version: 'v9.9.9', commit: 'abc1234' }))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetBuildInfo }))

function mountSidebar() {
  return mount(Sidebar, { global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows Checking sync after reconnect until a current sample arrives', async () => {
    const node = useNodeStore()
    node.applyStatus({ connected: true, mode: 'embedded' })
    node.sync = { state: 'stalled' } as any
    const w = mountSidebar()
    expect(w.text()).toContain('Sync stalled')
    node.applyStatus({ connected: false })
    node.applyStatus({ connected: true })
    await w.vm.$nextTick()
    expect(w.text()).toContain('Checking sync…')
    expect(w.text()).not.toContain('Sync stalled')
  })

  it('distinguishes disconnected, syncing and stalled states', async () => {
    const node = useNodeStore()
    const w = mountSidebar()
    expect(w.text()).toContain('Disconnected')
    expect(w.text()).not.toContain('Syncing')
    node.connected = true; node.syncing = true
    await w.vm.$nextTick()
    expect(w.text()).toContain('Syncing')
    node.sync = { state: 'stalled', currentHeight: 1, targetHeight: 2, percent: 50, etaSeconds: 0, peers: 0 }
    await w.vm.$nextTick()
    expect(w.text()).toContain('Sync stalled')
  })

  it('renders the core nav destinations', () => {
    const w = mountSidebar()
    const text = w.text()
    for (const label of ['Dashboard', 'Transfer', 'Receive', 'Tokens', 'Plasma', 'Staking', 'Pillars', 'Sentinels', 'Accelerator', 'Rewards', 'WalletConnect', 'Settings']) {
      expect(text).toContain(label)
    }
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
