import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
// Stub the router so TopBar's useRouter()/useRoute() resolve without a real
// router being installed (otherwise Vue logs an "injection Symbol(router) not
// found" warn). useRoute drives the on-Pillars-page title suffix.
const push = vi.hoisted(() => vi.fn())
const route = vi.hoisted(() => ({ name: 'dashboard', query: {}, meta: {} as Record<string, unknown> }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }), useRoute: () => route }))
import TopBar from './TopBar.vue'
import { useWalletStore } from '../stores/wallet'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useUnreceivedStore } from '../stores/unreceived'
import { useNodeStore } from '../stores/node'
import { useUiStore } from '../stores/ui'
import { useWalletConnectStore } from '../stores/walletconnect'

const stubs = { AccountSlotPicker: true }

describe('TopBar', () => {
  beforeEach(() => { setActivePinia(createPinia()); push.mockClear(); route.meta = {}; localStorage.removeItem('syrius.mode') })

  it('renders the page title', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.text()).toContain('Dashboard')
  })

  it('shows a lock button when unlocked', () => {
    const wallet = useWalletStore(); wallet.locked = false
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.find('[aria-label="Lock wallet"]').exists()).toBe(true)
  })

  it('exposes a theme toggle', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    expect(w.find('[aria-label="Toggle theme"]').exists()).toBe(true)
  })

  it('locks the wallet when the lock button is clicked', async () => {
    const wallet = useWalletStore(); wallet.locked = false
    const lock = vi.spyOn(wallet, 'lock').mockResolvedValue(undefined)
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    await w.find('button[aria-label="Lock wallet"]').trigger('click')
    expect(lock).toHaveBeenCalled()
  })

  // The count is a link to the payments. It must never change auto-receive,
  // which is an explicit switch on the Receive screen.
  it('the incoming count opens the payments and never toggles auto-receive', async () => {
    const wallet = useWalletStore(); wallet.locked = false; wallet.activeIndex = 3
    const toggle = vi.spyOn(useAutoReceiveStore(), 'toggle').mockResolvedValue(undefined)
    useUnreceivedStore().items = [{ fromHash: 'h1' }, { fromHash: 'h2' }] as any
    const w = mount(TopBar, { props: { title: 'Home' }, global: { stubs } })
    const btn = w.find('button[aria-label="2 incoming payment(s) waiting"]')
    expect(btn.text()).toBe('2')
    await btn.trigger('click')
    expect(push).toHaveBeenCalledWith('/receive')
    expect(toggle).not.toHaveBeenCalled()
  })

  it('offers a labelled mode switch that reflects the current mode', async () => {
    const ui = useUiStore()
    const w = mount(TopBar, { props: { title: 'Overview' }, global: { stubs } })
    const group = w.find('[role="group"][aria-label="Wallet mode"]')
    expect(group.text()).toContain('Mode')
    const [everyday, advanced] = group.findAll('button')
    expect(advanced.attributes('aria-pressed')).toBe('true')
    await everyday.trigger('click')
    expect(ui.mode).toBe('everyday')
    expect(everyday.attributes('aria-pressed')).toBe('true')
    // A shared screen stays where it is: no navigation, nothing discarded.
    expect(push).not.toHaveBeenCalled()
  })

  it('leaves an operator-only screen when switching to Everyday', async () => {
    route.meta = { advanced: true }
    const w = mount(TopBar, { props: { title: 'Pillars' }, global: { stubs } })
    await w.find('[aria-label="Wallet mode"] button').trigger('click')
    expect(push).toHaveBeenCalledWith('/dashboard')
  })

  it('names the connected network and chain', async () => {
    const node = useNodeStore(); node.connected = true; node.chainId = 1
    const w = mount(TopBar, { props: { title: 'Home' }, global: { stubs } })
    expect(w.get('[data-testid="network-chip"]').text()).toContain('Mainnet')
    expect(w.get('[data-testid="network-chip"]').text()).toContain('chain 1')
    node.chainId = 73404
    await w.vm.$nextTick()
    expect(w.get('[data-testid="network-chip"]').text()).toContain('Testnet')
    node.connected = false
    await w.vm.$nextTick()
    expect(w.find('[data-testid="network-chip"]').exists()).toBe(false)
  })

  it('shows WalletConnect in muted state when disconnected', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    const button = w.find('button[aria-label="WalletConnect"]')
    expect(button.exists()).toBe(true)
    expect(button.classes()).toContain('text-muted-foreground')
  })

  it('shows WalletConnect in green when a dapp is connected', () => {
    const walletConnect = useWalletConnectStore()
    walletConnect.sessions = [{
      topic: 'topic-1',
      name: 'Zenon Bridge',
      url: 'https://bridge.0x3639.com',
      icon: '',
      accounts: ['zenon:1:z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz'],
    }]
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    const button = w.find('button[aria-label="WalletConnect connected"]')
    expect(button.exists()).toBe(true)
    expect(button.classes()).toContain('text-success')
  })

  // TopBar only renders inside AppShell (unlocked); the lock button is always live.
  it('renders an enabled lock button', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    const btn = w.find('button[aria-label="Lock wallet"]')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('disabled')).toBeUndefined()
  })
})
