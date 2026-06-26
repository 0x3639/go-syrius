import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock TxService: PrepareSend is the assertion target — SendModal must convert
// the decimal amount to BASE UNITS (toBase) before calling prepare.
const PrepareSend = vi.hoisted(() => vi.fn().mockResolvedValue({}))
vi.mock('../../wailsjs/go/app/TxService', () => ({
  PrepareSend,
  ConfirmPublish: vi.fn(),
  CancelPending: vi.fn(),
}))

// Stub nom-ui Dialog/Content/Header/Title (pass-through slots) + useToast.
vi.mock('nom-ui', () => ({
  Dialog: { props: ['open'], emits: ['update:open'], template: '<div><slot /></div>' },
  DialogContent: { template: '<div><slot /></div>' },
  DialogHeader: { template: '<div><slot /></div>' },
  DialogTitle: { template: '<div><slot /></div>' },
  useToast: () => ({ show: vi.fn() }),
}))

// Stub SendForm to a button that emits the intent on click.
vi.mock('./SendForm.vue', () => ({
  default: {
    emits: ['send'],
    template:
      '<button @click="$emit(\'send\', { recipient: \'z1abc\', zts: \'zts1znn\', amountDecimal: \'1.5\' })">send</button>',
  },
}))
// Children that read tx state — keep them inert.
vi.mock('./TxModal.vue', () => ({ default: { template: '<div />' } }))
vi.mock('./TxResult.vue', () => ({ default: { template: '<div />' } }))

import SendModal from './SendModal.vue'
import { useBalancesStore, type TokenBalance } from '../stores/balances'
import { useTxStore } from '../stores/tx'

const znn: TokenBalance = { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '0' }

beforeEach(() => {
  setActivePinia(createPinia())
  PrepareSend.mockClear()
})

describe('SendModal', () => {
  it('converts the decimal amount to base units (toBase) before PrepareSend', async () => {
    useBalancesStore().items = [znn]
    const w = mount(SendModal, { props: { open: true } })

    await w.find('button').trigger('click')

    // 1.5 ZNN at 8 decimals -> 150000000 base units.
    expect(PrepareSend).toHaveBeenCalledWith(
      expect.objectContaining({ toAddress: 'z1abc', zts: 'zts1znn', amount: '150000000' }),
    )
  })

  it('hides the form (and its Send button) once a tx is in flight', async () => {
    useBalancesStore().items = [znn]
    const w = mount(SendModal, { props: { open: true } })
    expect(w.find('button').exists()).toBe(true) // form shown while idle
    useTxStore().status = 'awaiting'
    await w.vm.$nextTick()
    expect(w.find('button').exists()).toBe(false) // form gone in flight (TxModal/Result stubbed)
  })
})
