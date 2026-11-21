import { describe, it, expect, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from './Sidebar.vue'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'
import { useUnreceivedStore } from '../stores/unreceived'
import { vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'

const GetBuildInfo = vi.hoisted(() => vi.fn().mockResolvedValue({ version: 'v9.9.9', commit: 'abc1234' }))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetBuildInfo }))
const handlers = vi.hoisted(() => ({} as Record<string, (data?: any) => void>))
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn((event: string, handler: (data?: any) => void) => { handlers[event] = handler }),
}))

function mountSidebar() {
  return mount(Sidebar, { global: { stubs: { RouterLink: RouterLinkStub } } })
}

describe('Sidebar', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('shows Checking sync after reconnect until a current sample arrives', async () => {
    const node = useNodeStore()
    node.initEvents(vi.fn())
    node.applyStatus({ connected: true, mode: 'embedded' })
    node.sync = { state: 'stalled' } as any
    const w = mountSidebar()
    expect(w.text()).toContain('Sync stalled')
    node.applyStatus({ connected: false })
    node.applyStatus({ connected: true })
    await w.vm.$nextTick()
    expect(w.text()).toContain('Checking sync…')
    expect(w.text()).not.toContain('Sync stalled')
    handlers['node:sync']({ state: 'synced' })
    await w.vm.$nextTick()
    expect(w.text()).toContain('Synced')
    expect(w.text()).not.toContain('Checking sync…')
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
    for (const label of ['Overview', 'Activity', 'Send', 'Receive', 'Tokens', 'Plasma', 'Staking', 'Pillars', 'Sentinels', 'Accelerator', 'Rewards', 'WalletConnect', 'Settings']) {
      expect(text).toContain(label)
    }
  })

  it('Everyday shows five destinations and keeps operator tools out', () => {
    useUiStore().mode = 'everyday'
    const text = mountSidebar().text()
    for (const label of ['Home', 'Activity', 'Earn', 'More tools', 'Settings', 'Send', 'Receive']) {
      expect(text).toContain(label)
    }
    for (const label of ['Pillars', 'Sentinels', 'Tokens', 'Accelerator', 'Staking', 'Network of Momentum']) {
      expect(text).not.toContain(label)
    }
  })

  it('Everyday still shows how many payments are waiting', () => {
    useUiStore().mode = 'everyday'
    useUnreceivedStore().items = [{ fromHash: 'h1' }, { fromHash: 'h2' }] as any
    expect(mountSidebar().find('[aria-label="2 waiting"]').exists()).toBe(true)
  })

  it.each(['everyday', 'advanced'] as const)('%s marks Receive when incoming payments could not be checked', (mode) => {
    useUiStore().mode = mode
    useNodeStore().connected = true
    useUnreceivedStore().lookup = 'failed'
    const w = mountSidebar()
    expect(w.get(`[aria-label="Couldn't check"]`).text()).toBe('?')
    expect(w.find('[aria-label="0 waiting"]').exists()).toBe(false)
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
