import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const ClipboardSetText = vi.hoisted(() => vi.fn().mockResolvedValue(true))
vi.mock('../../wailsjs/runtime/runtime', () => ({ ClipboardSetText }))
vi.mock('nom-ui', () => ({
  Button: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
}))
import TxResult from './TxResult.vue'
import { useTxStore } from '../stores/tx'

beforeEach(() => {
  setActivePinia(createPinia())
  ClipboardSetText.mockClear()
})

describe('TxResult', () => {
  it('copies the hash via the copy icon', async () => {
    useTxStore().hash = 'deadbeef'
    const w = mount(TxResult)
    await w.find('button[aria-label="copy hash"]').trigger('click')
    expect(ClipboardSetText).toHaveBeenCalledWith('deadbeef')
  })

  it('emits close from the Close button', async () => {
    const w = mount(TxResult)
    const closeBtn = w.findAll('button').find((b) => b.text() === 'Close')!
    await closeBtn.trigger('click')
    expect(w.emitted('close')).toBeTruthy()
  })
})
