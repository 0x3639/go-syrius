import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import PlasmaPanel from './PlasmaPanel.vue'
import { usePlasmaStore } from '../../stores/plasma'
import { useBalancesStore } from '../../stores/balances'
import { useNodeStore } from '../../stores/node'
import { useTxStore } from '../../stores/tx'

// Stub nom-ui Input/Button to plain elements that honour v-model + click, so we
// exercise the panel's bindings (Nom + tx), not nom-ui internals.
vi.mock('nom-ui', () => ({
  Input: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Button: {
    props: ['disabled'],
    emits: ['click'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
}))

// Mock the NomService preparers — each returns a distinct preview so we can
// assert the right one is forwarded to tx.awaitConfirm.
const fusePreview = { kind: 'fuse' }
const cancelPreview = { kind: 'cancel' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareFuse: vi.fn(() => Promise.resolve(fusePreview)),
  PrepareCancelFuse: vi.fn(() => Promise.resolve(cancelPreview)),
  EstimatePlasma: vi.fn(() => Promise.resolve(0)),
  GetPlasmaInfo: vi.fn(() => Promise.resolve(null)),
  GetFusionEntries: vi.fn(() => Promise.resolve([])),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

function setup() {
  setActivePinia(createPinia())
  useNodeStore().connected = true
  useBalancesStore().status = 'ready'
  useBalancesStore().items = [{ zts: 'zts1qsr', symbol: 'QSR', amount: '10000000000', decimals: 8 }]
  const plasma = usePlasmaStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(plasma, 'refresh').mockResolvedValue()
  vi.spyOn(plasma, 'estimate').mockResolvedValue(0)
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  return { plasma, tx, awaitConfirm }
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(PlasmaPanel)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('PlasmaPanel', () => {
  it('floors Max to whole QSR and validates the fusion minimum and address case', async () => {
    setup()
    useBalancesStore().items[0].amount = '1250000000'
    const w = render()
    const address = 'z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7'.toUpperCase()
    await w.get('[aria-label="beneficiary"]').setValue(address)
    await w.get('[aria-label="max amount"]').trigger('click')
    expect((w.get('[aria-label="Amount (QSR)"]').element as HTMLInputElement).value).toBe('12')
    expect(w.text()).toContain('12.5 QSR available')
    const button = w.findAll('button').find(b => b.text() === 'Fuse Plasma')!
    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click')
    expect(Nom.PrepareFuse).toHaveBeenCalledTimes(1)
    expect(Nom.PrepareFuse).toHaveBeenCalledWith(address, '1200000000')
    for (const value of ['9', '12.5', '-10']) {
      await w.get('[aria-label="Amount (QSR)"]').setValue(value)
      expect(button.attributes('disabled')).toBeDefined()
    }
    await w.get('[aria-label="Amount (QSR)"]').setValue('10')
    await w.get('[aria-label="beneficiary"]').setValue('z' + address.slice(1))
    expect(button.attributes('disabled')).toBeDefined()
    useBalancesStore().items[0].amount = '999999999'
    await w.vm.$nextTick()
    expect(w.find('[aria-label="max amount"]').exists()).toBe(false)
  })

  it('refreshes on reconnect without discarding its draft', async () => {
    const { plasma } = setup()
    const w = render()
    await w.get('[aria-label="Amount (QSR)"]').setValue('12')
    useNodeStore().connected = false
    await w.vm.$nextTick()
    expect(plasma.refresh).toHaveBeenCalledTimes(1)
    useNodeStore().connected = true
    await w.vm.$nextTick()
    expect(plasma.refresh).toHaveBeenCalledTimes(2)
    expect((w.get('[aria-label="Amount (QSR)"]').element as HTMLInputElement).value).toBe('12')
  })
  it('Fuse calls PrepareFuse(beneficiary, baseUnits) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    const addr = 'z1' + 'q'.repeat(38)
    await w.get('[aria-label="beneficiary"]').setValue(addr)
    await w.get('[aria-label="Amount (QSR)"]').setValue('50')

    // The Fuse button is the only non-outline button labelled "Fuse Plasma".
    const fuseBtn = w
      .findAll('button')
      .find((b) => b.text() === 'Fuse Plasma')!
    await fuseBtn.trigger('click')
    await w.vm.$nextTick()

    // 50 QSR @ 8 decimals -> 5000000000 base units.
    expect(Nom.PrepareFuse).toHaveBeenCalledTimes(1)
    expect(Nom.PrepareFuse).toHaveBeenCalledWith(addr, '5000000000')
    expect(awaitConfirm).toHaveBeenCalledWith(fusePreview)
  })
})
