import { useNodeStore } from '../../stores/node'
import { mount } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { props: ['variant'], template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDonate: vi.fn(() => Promise.resolve({ kind: 'donate' })),
}))

import AcceleratorDonate from './AcceleratorDonate.vue'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useTxStore } from '../../stores/tx'
import { useBalancesStore } from '../../stores/balances'

describe('AcceleratorDonate', () => {
  function setup() {
    setActivePinia(createPinia()); useNodeStore().connected = true
    const tx = useTxStore()
    const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
    const balances = useBalancesStore()
    balances.items = [
      { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '1000000000' },
      { zts: 'zts1qsr', symbol: 'QSR', decimals: 8, amount: '1000000000' },
    ]
    balances.status = 'ready'
    return { awaitConfirm, balances, w: mount(AcceleratorDonate) }
  }
  const donateBtn = (w: ReturnType<typeof mount>) => w.find('button[aria-label="donate"]')

  it('converts a whole-token amount to base units and defaults to ZNN', async () => {
    const { w, awaitConfirm } = setup()
    await w.find('input[aria-label="donate amount"]').setValue('1.5')
    await donateBtn(w).trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareDonate).toHaveBeenLastCalledWith('150000000', 'ZNN')
    expect(awaitConfirm).toHaveBeenCalledWith({ kind: 'donate' })
  })

  it('donates QSR when it is picked', async () => {
    const { w } = setup()
    await w.find('button[aria-label="donate QSR"]').trigger('click')
    await w.find('input[aria-label="donate amount"]').setValue('5')
    await donateBtn(w).trigger('click')
    await new Promise((r) => setTimeout(r))
    expect(Nom.PrepareDonate).toHaveBeenLastCalledWith('500000000', 'QSR')
  })

  it('blocks zero, malformed and over-precise amounts', async () => {
    const { w } = setup()
    vi.mocked(Nom.PrepareDonate).mockClear()
    for (const bad of ['', '0', '-1', 'abc', '1.123456789']) {
      await w.find('input[aria-label="donate amount"]').setValue(bad)
      expect(donateBtn(w).attributes('disabled')).toBeDefined()
      await donateBtn(w).trigger('click')
    }
    expect(Nom.PrepareDonate).not.toHaveBeenCalled()
  })

  it('stays disabled until the balance is available', async () => {
    const { w, balances } = setup()
    balances.status = 'loading'
    await w.find('input[aria-label="donate amount"]').setValue('1')
    expect(donateBtn(w).attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('balance is not available yet')
    balances.status = 'ready'
    await w.vm.$nextTick()
    expect(donateBtn(w).attributes('disabled')).toBeUndefined()
  })

  it('blocks an amount above the held balance', async () => {
    const { w, balances } = setup()
    balances.items = [{ zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '100000000' }]
    await w.vm.$nextTick()
    await w.find('input[aria-label="donate amount"]').setValue('2')
    expect(donateBtn(w).attributes('disabled')).toBeDefined()
    expect(w.text()).toContain('more than this account holds')
    await w.find('input[aria-label="donate amount"]').setValue('1')
    expect(donateBtn(w).attributes('disabled')).toBeUndefined()
  })
})
