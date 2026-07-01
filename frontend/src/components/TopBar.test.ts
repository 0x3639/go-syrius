import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
// Stub the router so TopBar's useRouter()/useRoute() resolve without a real
// router being installed (otherwise Vue logs an "injection Symbol(router) not
// found" warn). useRoute drives the on-Pillars-page title suffix.
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }), useRoute: () => ({ name: 'dashboard', query: {} }) }))
import TopBar from './TopBar.vue'
import { useWalletStore } from '../stores/wallet'
import { useAutoReceiveStore } from '../stores/autoReceive'

const stubs = { AccountSlotPicker: true }

describe('TopBar', () => {
  beforeEach(() => setActivePinia(createPinia()))

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
    const lock = vi.spyOn(wallet, 'lock').mockImplementation(() => {})
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    await w.find('button[aria-label="Lock wallet"]').trigger('click')
    expect(lock).toHaveBeenCalled()
  })

  it('toggles auto-receive with the active account index', async () => {
    const wallet = useWalletStore(); wallet.locked = false; wallet.activeIndex = 3
    const autoReceive = useAutoReceiveStore()
    const toggle = vi.spyOn(autoReceive, 'toggle').mockResolvedValue(undefined)
    const w = mount(TopBar, { props: { title: 'Dashboard' }, global: { stubs } })
    await w.find('button[aria-label="Auto-receive off"]').trigger('click')
    expect(toggle).toHaveBeenCalledWith(3)
  })

  it('disables the lock button while locked', () => {
    const w = mount(TopBar, { props: { title: 'Dashboard', locked: true }, global: { stubs } })
    const btn = w.find('button[aria-label="Lock wallet"]')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('disabled')).toBeDefined()
  })
})
