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
import { useNodeStore } from '../stores/node'
import { useBalancesStore } from '../stores/balances'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useTxsStore } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useWalletStore } from '../stores/wallet'

// Stub every bootstrap action so the integration runs end-to-end (we do NOT mock
// AppShell's bootstrap away — that's the regression this suite must catch) while
// no real Wails binding fires. Returns the live store instances for assertions.
function stubStores() {
  const node = useNodeStore()
  const balances = useBalancesStore()
  const plasma = usePlasmaStore()
  const pillar = usePillarStore()
  const accelerator = useAcceleratorStore()
  const txs = useTxsStore()
  const unreceived = useUnreceivedStore()
  const ui = useUiStore()
  const autoReceive = useAutoReceiveStore()
  const wallet = useWalletStore()

  vi.spyOn(node, 'initEvents').mockImplementation(() => {})
  vi.spyOn(balances, 'load').mockResolvedValue(undefined as any)
  vi.spyOn(plasma, 'refresh').mockResolvedValue(undefined as any)
  vi.spyOn(pillar, 'refreshDelegation').mockResolvedValue(undefined as any)
  vi.spyOn(pillar, 'refreshMyPillar').mockResolvedValue(undefined as any)
  vi.spyOn(accelerator, 'refreshVotable').mockResolvedValue(undefined as any)
  vi.spyOn(txs, 'load').mockResolvedValue(undefined as any)
  vi.spyOn(txs, 'resetPage').mockImplementation(() => {})
  vi.spyOn(unreceived, 'load').mockResolvedValue(undefined as any)
  vi.spyOn(ui, 'init').mockResolvedValue(undefined as any)
  vi.spyOn(autoReceive, 'init').mockResolvedValue(undefined as any)
  vi.spyOn(autoReceive, 'followAccount').mockResolvedValue(undefined as any)

  return { node, balances, txs, autoReceive, wallet }
}

function mountShell() {
  return mount(AppShell, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        RouterView: { template: '<div class="rv-stub">page</div>' },
        AccountSlotPicker: true,
      },
    },
  })
}

describe('AppShell', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('renders the sidebar, a topbar title from route meta, and a router-view outlet', () => {
    stubStores()
    const w = mountShell()
    expect(w.find('aside').exists()).toBe(true)
    expect(w.find('header').text()).toContain('Dashboard')
    expect(w.find('.rv-stub').exists()).toBe(true)
  })

  it('runs the global bootstrap on mount: wires node events and fires an initial balances refresh', () => {
    const { node, balances } = stubStores()
    mountShell()
    expect(node.initEvents).toHaveBeenCalledTimes(1)
    expect(balances.load).toHaveBeenCalledTimes(1)
  })

  it('refreshes data and re-points auto-receive when the active account changes', async () => {
    const { balances, txs, autoReceive, wallet } = stubStores()
    mountShell()
    // initial mount load
    expect(balances.load).toHaveBeenCalledTimes(1)

    wallet.activeIndex = 2
    await Promise.resolve() // let the watcher flush
    await Promise.resolve()

    expect(txs.resetPage).toHaveBeenCalled()
    expect(balances.load).toHaveBeenCalledTimes(2)
    expect(autoReceive.followAccount).toHaveBeenCalledWith(2)
  })
})
