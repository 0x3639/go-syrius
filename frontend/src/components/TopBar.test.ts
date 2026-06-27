import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ autoReceive: false }))
const SetSettings = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetSettings }))
const StartAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const StopAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ StartAutoReceive, StopAutoReceive }))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
import TopBar from './TopBar.vue'
import { useWalletStore } from '../stores/wallet'

const flush = () => new Promise((r) => setTimeout(r))
const opts = { global: { stubs: { AccountSlotPicker: true } } }

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
  StartAutoReceive.mockClear()
  StopAutoReceive.mockClear()
})

describe('TopBar', () => {
  it('locks the wallet', async () => {
    const w = mount(TopBar, opts)
    const wallet = useWalletStore()
    const lock = vi.spyOn(wallet, 'lock').mockImplementation(() => {})
    await w.find('button[aria-label="Lock wallet"]').trigger('click')
    expect(lock).toHaveBeenCalled()
  })

  it('navigates to the address book and settings', async () => {
    const w = mount(TopBar, opts)
    await w.find('button[aria-label="Address book"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/address-book')
    await w.find('button[aria-label="Settings"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/settings')
  })

  it('toggles auto-receive', async () => {
    const w = mount(TopBar, opts)
    await w.find('button[aria-label="Auto-receive off"]').trigger('click')
    await flush()
    expect(StartAutoReceive).toHaveBeenCalled()
  })

  it('renders a locked variant with every icon shown but disabled', () => {
    const w = mount(TopBar, { props: { locked: true }, ...opts })
    expect(w.text()).toContain('Locked')
    // All right-side icons are present but inert while locked.
    for (const label of ['Plasma', 'Auto-receive', 'Lock wallet', 'Address book', 'Settings']) {
      const btn = w.find(`button[aria-label="${label}"]`)
      expect(btn.exists(), label).toBe(true)
      expect(btn.attributes('disabled'), label).toBeDefined()
    }
  })

  it('does not navigate when a locked icon is clicked', async () => {
    const w = mount(TopBar, { props: { locked: true }, ...opts })
    await w.find('button[aria-label="Settings"]').trigger('click')
    await w.find('button[aria-label="Address book"]').trigger('click')
    expect(push).not.toHaveBeenCalled()
  })
})
