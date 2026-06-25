import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import PlasmaPanel from './PlasmaPanel.vue'
import { usePlasmaStore } from '../../stores/plasma'
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
  it('Fuse calls PrepareFuse(beneficiary, baseUnits) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup()
    const w = render()
    await w.vm.$nextTick()

    const addr = 'z1qtestbeneficiaryaddress000000000000000000'
    await w.get('[aria-label="beneficiary"]').setValue(addr)
    await w.get('[aria-label="qsr amount"]').setValue('50')

    // The Fuse button is the only non-outline button labelled "Fuse Plasma".
    const fuseBtn = w
      .findAll('button')
      .find((b) => b.text() === 'Fuse Plasma')!
    await fuseBtn.trigger('click')
    await w.vm.$nextTick()

    // 50 QSR @ 8 decimals -> 5000000000 base units.
    expect(Nom.PrepareFuse).toHaveBeenCalledWith(addr, '5000000000')
    expect(awaitConfirm).toHaveBeenCalledWith(fusePreview)
  })
})
