import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Transfer from './Transfer.vue'
import { useTxStore } from '../stores/tx'

describe('Transfer page', () => {
  beforeEach(() => setActivePinia(createPinia()))
  it('renders the send form while idle', () => {
    const w = mount(Transfer, { global: { stubs: { SendForm: true, TxModal: true, TxResult: true } } })
    expect(w.findComponent({ name: 'SendForm' }).exists() || w.find('send-form-stub').exists()).toBe(true)
  })

  it('awaits cleanup of a retained error hold before preparing a replacement send', async () => {
    const tx = useTxStore()
    tx.status = 'error'
    tx.preview = { holdId: 71 } as any
    let finishDiscard!: () => void
    const discard = vi.spyOn(tx, 'discard').mockReturnValue(new Promise<void>((resolve) => { finishDiscard = resolve }))
    const prepare = vi.spyOn(tx, 'prepare').mockResolvedValue(undefined)
    const w = mount(Transfer, {
      global: {
        stubs: {
          SendForm: {
            name: 'SendForm',
            emits: ['send'],
            template: '<button data-test="send" @click="$emit(\'send\', { recipient: \'z1qto\', zts: \'zts1znn\', amountDecimal: \'1\' })">send</button>',
          },
          TxModal: true,
          TxResult: true,
        },
      },
    })

    await w.find('[data-test="send"]').trigger('click')
    expect(discard).toHaveBeenCalledOnce()
    expect(prepare).not.toHaveBeenCalled()

    finishDiscard()
    await Promise.resolve()
    await Promise.resolve()
    expect(prepare).toHaveBeenCalledWith('z1qto', 'zts1znn', '100000000')
  })
})
