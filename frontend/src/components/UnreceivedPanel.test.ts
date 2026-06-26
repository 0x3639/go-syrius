import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import UnreceivedPanel from './UnreceivedPanel.vue'
import { useUnreceivedStore, type Unreceived } from '../stores/unreceived'

// Stub nom-ui Button to a plain <button> mirroring disabled + click, so we
// exercise our row composition and store binding, not nom-ui internals.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['disabled'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
}))

const item: Unreceived = {
  fromHash: '0xhash1',
  fromAddress: 'z1qabcdefghijklmnopqrstuvwxyz0123456789ab',
  token: 'ZNN',
  amount: '150000000',
  decimals: 8,
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('UnreceivedPanel', () => {
  it('renders a row with the from address, amount and a Receive button', async () => {
    const w = mount(UnreceivedPanel)
    const store = useUnreceivedStore()
    // stub load so onMounted does not hit the (mocked-absent) backend
    vi.spyOn(store, 'load').mockResolvedValue()
    store.items = [item]
    await w.vm.$nextTick()

    expect(w.text()).toContain('Unreceived (1)')
    expect(w.text()).toContain('1.5 ZNN')
    const btns = w.findAll('button')
    const receive = btns.find((b) => b.text() === 'Receive')
    expect(receive).toBeTruthy()
  })

  it('renders a non-8-decimal token amount using the row decimals', async () => {
    const w = mount(UnreceivedPanel)
    const store = useUnreceivedStore()
    vi.spyOn(store, 'load').mockResolvedValue()
    // 1500000 base units at 6 decimals == 1.5, NOT 0.015 (the wrong 8-dec form).
    store.items = [{ ...item, token: 'CUSTOM', amount: '1500000', decimals: 6 }]
    await w.vm.$nextTick()

    expect(w.text()).toContain('1.5 CUSTOM')
    expect(w.text()).not.toContain('0.015')
  })

  it('calls store.receive(fromHash) when the Receive button is clicked', async () => {
    const w = mount(UnreceivedPanel)
    const store = useUnreceivedStore()
    vi.spyOn(store, 'load').mockResolvedValue()
    const receiveSpy = vi.spyOn(store, 'receive').mockResolvedValue()
    store.items = [item]
    await w.vm.$nextTick()

    const receive = w.findAll('button').find((b) => b.text() === 'Receive')!
    await receive.trigger('click')
    expect(receiveSpy).toHaveBeenCalledWith('0xhash1')
  })

  it('shows a pulsing Generating Plasma status while the row is busy (no Receive button)', async () => {
    const w = mount(UnreceivedPanel)
    const store = useUnreceivedStore()
    vi.spyOn(store, 'load').mockResolvedValue()
    store.items = [item]
    store.busy = { '0xhash1': true }
    await w.vm.$nextTick()

    // plasma None (default) → "Generating Plasma…"; the Receive button is replaced.
    expect(w.text()).toContain('Generating Plasma')
    expect(w.findAll('button').find((b) => b.text() === 'Receive')).toBeFalsy()
    expect(w.find('.animate-pulse').exists()).toBe(true)
  })
})
