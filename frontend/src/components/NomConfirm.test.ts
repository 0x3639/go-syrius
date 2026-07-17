import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Stub nom-ui Dialog as a passthrough that forwards v-model:open. It renders its
// slot only when open, and emits update:open so closing can be simulated. The
// other Dialog parts are inert passthroughs.
vi.mock('nom-ui', () => ({
  Dialog: {
    name: 'Dialog',
    props: ['open'],
    emits: ['update:open'],
    template: '<div data-test="dialog" :data-open="open"><slot v-if="open" /></div>',
  },
  DialogContent: { template: '<div><slot /></div>' },
  DialogHeader: { template: '<div><slot /></div>' },
  DialogTitle: { template: '<div><slot /></div>' },
  Button: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
}))

// Stub the body components so we only assert which one renders, not their guts.
vi.mock('./TxModal.vue', () => ({
  default: { template: '<div data-test="tx-modal" />' },
}))
vi.mock('./TxResult.vue', () => ({
  default: { template: '<div data-test="tx-result" />' },
}))

import NomConfirm from './NomConfirm.vue'
import { useTxStore } from '../stores/tx'

const PREVIEW = {
  toAddress: 'z1abc',
  amount: '100000000',
  zts: 'zts1znn',
  symbol: 'ZNN',
  needsPoW: false,
  difficulty: 0,
  hash: 'h',
  usedPlasma: 0,
} as any

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('NomConfirm (global panel confirm)', () => {
  it('opens and renders TxModal when status is awaiting', () => {
    const tx = useTxStore()
    tx.awaitConfirm(PREVIEW)

    const w = mount(NomConfirm)

    expect(w.find('[data-test="dialog"]').attributes('data-open')).toBe('true')
    expect(w.find('[data-test="tx-modal"]').exists()).toBe(true)
    expect(w.find('[data-test="tx-result"]').exists()).toBe(false)
  })

  it('renders TxResult when status is done', () => {
    const tx = useTxStore()
    tx.status = 'done'
    tx.hash = 'h'

    const w = mount(NomConfirm)

    expect(w.find('[data-test="dialog"]').attributes('data-open')).toBe('true')
    expect(w.find('[data-test="tx-result"]').exists()).toBe(true)
    expect(w.find('[data-test="tx-modal"]').exists()).toBe(false)
  })

  it('stays closed when idle', () => {
    const tx = useTxStore()
    tx.reset()

    const w = mount(NomConfirm)

    expect(w.find('[data-test="dialog"]').attributes('data-open')).toBe('false')
    expect(w.find('[data-test="tx-modal"]').exists()).toBe(false)
  })

  it('closing while awaiting calls tx.discard', async () => {
    const tx = useTxStore()
    tx.awaitConfirm(PREVIEW)
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const reset = vi.spyOn(tx, 'reset')

    const w = mount(NomConfirm)
    w.findComponent({ name: 'Dialog' }).vm.$emit('update:open', false)
    await w.vm.$nextTick()

    expect(discard).toHaveBeenCalled()
    expect(reset).not.toHaveBeenCalled()
  })

  it('closing while done calls tx.reset', async () => {
    const tx = useTxStore()
    tx.status = 'done'
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const reset = vi.spyOn(tx, 'reset')

    const w = mount(NomConfirm)
    w.findComponent({ name: 'Dialog' }).vm.$emit('update:open', false)
    await w.vm.$nextTick()

    expect(reset).toHaveBeenCalled()
    expect(discard).not.toHaveBeenCalled()
  })

  it('stays open on error and renders the failure', async () => {
    const tx = useTxStore()
    tx.status = 'error'
    tx.error = 'no pending block'

    const w = mount(NomConfirm)
    expect(w.find('[data-test="dialog"]').attributes('data-open')).toBe('true')
    expect(w.text()).toContain('no pending block')

    // A retryable confirm error may still own its backend hold.
    const discard = vi.spyOn(tx, 'discard').mockResolvedValue(undefined)
    const reset = vi.spyOn(tx, 'reset')
    w.findComponent({ name: 'Dialog' }).vm.$emit('update:open', false)
    await w.vm.$nextTick()
    expect(discard).toHaveBeenCalled()
    expect(reset).not.toHaveBeenCalled()
  })
})
