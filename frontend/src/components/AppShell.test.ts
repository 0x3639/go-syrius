import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, RouterLinkStub, enableAutoUnmount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

// Each mountShell() registers window activity listeners; without teardown they
// leak across tests and multiply NoteActivity pings. Auto-unmount every mount
// after its test so window handlers do not accumulate.
enableAutoUnmount(afterEach)

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

const NoteActivity = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const GetBalances = vi.hoisted(() => vi.fn())
vi.mock('../../wailsjs/go/app/NodeService', () => ({ GetBalances }))
vi.mock('../../wailsjs/go/app/WalletService', () => ({ NoteActivity }))

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
import { useTxStore } from '../stores/tx'
import { useWalletConnectStore } from '../stores/walletconnect'

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
  wallet.locked = false
  node.connected = true
  const tx = useTxStore()
  const walletConnect = useWalletConnectStore()

  vi.spyOn(node, 'initEvents').mockImplementation(() => {})
  vi.spyOn(tx, 'initEvents').mockImplementation(() => {})
  vi.spyOn(wallet, 'initLockEvent').mockImplementation(() => {})
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
  vi.spyOn(autoReceive, 'start').mockResolvedValue(undefined)
  vi.spyOn(autoReceive, 'followAccount').mockResolvedValue(undefined as any)
  vi.spyOn(walletConnect, 'projectId').mockReturnValue('test-project-id')
  vi.spyOn(walletConnect, 'ensureClient').mockResolvedValue({} as any)
  vi.spyOn(walletConnect, 'updateAccount').mockResolvedValue(undefined as any)

  return { node, balances, txs, plasma, unreceived, autoReceive, wallet, walletConnect }
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

  it('does not follow the reset account index when the wallet locks', () => {
    const { wallet, autoReceive, walletConnect } = stubStores()
    wallet.activeIndex = 2
    mountShell()
    wallet._applyLocked()
    expect(autoReceive.followAccount).not.toHaveBeenCalled()
    expect(walletConnect.updateAccount).not.toHaveBeenCalled()
  })

  it('loads the replacement connection without waiting for an obsolete RPC', async () => {
    const { node, balances } = stubStores()
    vi.mocked(balances.load).mockRestore()
    let finish!: (r: unknown) => void
    GetBalances.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    GetBalances.mockResolvedValueOnce([{ zts: 'new', amount: '2' }])
    mountShell()
    node.applyStatus({ connected: false })
    node.applyStatus({ connected: true })
    await flushPromises()
    expect(balances.items[0]?.zts).toBe('new')
    finish([{ zts: 'old', amount: '1' }])
    await flushPromises()
    expect(balances.items[0]?.zts).toBe('new')
  })

  it.each([
    { enabled: true, locked: false, starts: 1 },
    { enabled: false, locked: false, starts: 0 },
    { enabled: true, locked: true, starts: 0 },
  ])('restores auto-receive on reconnect only when enabled and unlocked: %j', ({ enabled, locked, starts }) => {
    const { node, wallet, autoReceive } = stubStores()
    autoReceive.enabled = enabled
    autoReceive.account = wallet.activeIndex
    mountShell()
    node.applyStatus({ connected: false })
    expect(autoReceive.start).not.toHaveBeenCalled()
    wallet.locked = locked
    node.applyStatus({ connected: true })
    expect(autoReceive.start).toHaveBeenCalledTimes(starts)
    if (starts) expect(autoReceive.start).toHaveBeenCalledWith(wallet.activeIndex)
  })

  it('invalidates all displayed wallet snapshots on a same-tick reconnect', () => {
    const { node, balances, plasma, unreceived } = stubStores()
    node.connected = true
    mountShell()
    balances.items = [{ amount: '1' } as any]
    plasma.info = { currentPlasma: 100 } as any
    unreceived.items = [{ fromHash: 'old' } as any]
    // Successful endpoint replacement may deliver false/true before Vue renders.
    node.applyStatus({ connected: false })
    node.applyStatus({ connected: true })
    expect(balances.items).toEqual([])
    expect(plasma.info).toBeNull()
    expect(unreceived.items).toEqual([])
  })

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
    const { balances, txs, autoReceive, wallet, walletConnect } = stubStores()
    wallet.accounts = [{ index: 2, address: 'z1qnew', label: '' }]
    mountShell()
    // initial mount load
    expect(balances.load).toHaveBeenCalledTimes(1)

    wallet.activeIndex = 2
    await Promise.resolve() // let the watcher flush
    await Promise.resolve()

    expect(txs.resetPage).toHaveBeenCalled()
    expect(balances.load).toHaveBeenCalledTimes(2)
    expect(autoReceive.followAccount).toHaveBeenCalledWith(2)
    expect(walletConnect.updateAccount).toHaveBeenCalledWith('z1qnew')
  })

  it('reconciles restored WalletConnect sessions with the active account after unlock', async () => {
    const { wallet, walletConnect } = stubStores()
    wallet.locked = false
    wallet.accounts = [{ index: 0, address: 'z1qactive', label: '' }]

    mountShell()
    await Promise.resolve()
    await Promise.resolve()

    expect(walletConnect.ensureClient).toHaveBeenCalledOnce()
    expect(walletConnect.updateAccount).toHaveBeenCalledWith('z1qactive')
  })

  it('wires the backend-lock listener on mount', async () => {
    const { wallet } = stubStores()
    mountShell()
    expect(wallet.initLockEvent).toHaveBeenCalledTimes(1)
    // Navigation is owned by App.vue's wallet.locked watcher, so the listener is
    // registered with no callback argument.
    expect((wallet.initLockEvent as any).mock.calls[0]).toEqual([])
  })

  it('pings NoteActivity on user input, throttled, and unregisters on unmount', async () => {
    stubStores()
    NoteActivity.mockClear()
    const w = mountShell()
    window.dispatchEvent(new Event('pointerdown'))
    window.dispatchEvent(new Event('keydown')) // within the 15s throttle window
    expect(NoteActivity).toHaveBeenCalledTimes(1)
    w.unmount()
    NoteActivity.mockClear()
    window.dispatchEvent(new Event('pointerdown'))
    expect(NoteActivity).not.toHaveBeenCalled() // listeners removed on unmount
  })
})
