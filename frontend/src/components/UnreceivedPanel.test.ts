import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import UnreceivedPanel from './UnreceivedPanel.vue'
import { useUnreceivedStore, type Unreceived } from '../stores/unreceived'
import { useNodeStore } from '../stores/node'

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

    expect(w.text()).toContain('Waiting to be received · 1')
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

  // Three answers that must never be merged: we could not check, we have not
  // heard back yet, and we checked and found nothing.
  describe('lookup states', () => {
    function mounted(set: (s: ReturnType<typeof useUnreceivedStore>) => void) {
      const store = useUnreceivedStore()
      vi.spyOn(store, 'load').mockResolvedValue()
      useNodeStore().connected = true
      set(store)
      return { w: mount(UnreceivedPanel), store }
    }

    it('says it could not check, and never "nothing is waiting", when the lookup failed', () => {
      const { w } = mounted((s) => { s.lookup = 'failed'; s.lastCheckedAt = new Date(2026, 10, 21, 14, 2, 7).getTime() })
      const alert = w.get('[data-testid="incoming-failed"]')
      expect(alert.attributes('role')).toBe('alert')
      expect(alert.text()).toContain("Couldn't check for incoming payments")
      expect(alert.text()).toContain('not the same as nothing waiting')
      expect(alert.text()).toContain('Last successful check at 14:02:07')
      expect(w.find('[data-testid="incoming-empty"]').exists()).toBe(false)
      expect(w.text()).not.toContain('Nothing is waiting.')
      expect(w.text()).not.toContain('· 0')
    })

    it('says so when there has been no successful check yet', () => {
      const { w } = mounted((s) => { s.lookup = 'failed' })
      expect(w.text()).toContain('No successful check yet')
    })

    it('Try again reruns the lookup', async () => {
      const { w, store } = mounted((s) => { s.lookup = 'failed' })
      ;(store.load as any).mockClear()
      await w.findAll('button').find((b) => b.text() === 'Try again')!.trigger('click')
      // (The stubbed Button both emits and forwards the native click, so the
      // handler can run twice here; what matters is that the lookup reruns.)
      expect(store.load).toHaveBeenCalled()
    })

    it('says nothing is waiting, with the check time, only after a successful lookup', () => {
      const { w } = mounted((s) => { s.lookup = 'ok'; s.lastCheckedAt = new Date(2026, 10, 21, 9, 30, 0).getTime() })
      const empty = w.get('[data-testid="incoming-empty"]')
      expect(empty.text()).toContain('Nothing is waiting.')
      expect(empty.text()).toContain('Checked at 09:30:00')
      expect(w.text()).toContain('· 0')
      expect(w.find('[data-testid="incoming-failed"]').exists()).toBe(false)
    })

    it('says it is still checking before the first answer', () => {
      const { w } = mounted(() => {})
      expect(w.get('[data-testid="incoming-pending"]').text()).toContain('Checking for incoming payments')
      expect(w.text()).not.toContain('Nothing is waiting.')
    })

    it('leaves a lost connection to the balance notice', () => {
      const { w } = mounted((s) => { s.lookup = 'failed' })
      useNodeStore().connected = false
      return w.vm.$nextTick().then(() => expect(w.find('[data-testid="incoming-failed"]').exists()).toBe(false))
    })
  })
})
