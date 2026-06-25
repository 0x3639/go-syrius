import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import SentinelsPanel from './SentinelsPanel.vue'
import { useSentinelStore } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'

// Stub nom-ui Button to a plain element mirroring disabled/click, so we
// exercise the panel's bindings, not nom-ui internals.
vi.mock('nom-ui', () => ({
  Button: {
    props: ['disabled'],
    template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot /></button>',
  },
  Input: {
    props: ['modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))

// Mock the NomService preparers — each returns a distinct preview so we can
// assert the right preview is forwarded to tx.awaitConfirm.
const depositPreview = { kind: 'deposit' }
const registerPreview = { kind: 'register' }
vi.mock('../../../wailsjs/go/app/NomService', () => ({
  PrepareDepositQsr: vi.fn(() => Promise.resolve(depositPreview)),
  PrepareRegisterSentinel: vi.fn(() => Promise.resolve(registerPreview)),
  PrepareCollectSentinelReward: vi.fn(() => Promise.resolve({ kind: 'collect' })),
  PrepareRevokeSentinel: vi.fn(() => Promise.resolve({ kind: 'revoke' })),
  PrepareWithdrawQsr: vi.fn(() => Promise.resolve({ kind: 'withdraw' })),
}))

import * as Nom from '../../../wailsjs/go/app/NomService'

// 50,000 QSR in base units (1e8) — the full required deposit, matching the
// Svelte original's QSR_REQUIRED. With depositedQsr '0', shortfall == this.
const QSR_REQUIRED = '5000000000000'

function setup(depositedQsr = '0', sentinelOverride: unknown = null) {
  setActivePinia(createPinia())
  const sentinel = useSentinelStore()
  const tx = useTxStore()
  // Don't hit the (mocked-absent) backend on mount.
  vi.spyOn(sentinel, 'refresh').mockResolvedValue()
  const awaitConfirm = vi.spyOn(tx, 'awaitConfirm').mockImplementation(() => {})
  sentinel.depositedQsr = depositedQsr
  sentinel.sentinel = sentinelOverride as never
  return { sentinel, tx, awaitConfirm }
}

let wrapper: VueWrapper | null = null
function render() {
  wrapper = mount(SentinelsPanel)
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
})

describe('SentinelsPanel', () => {
  it('clicking Deposit calls PrepareDepositQsr(<base units>) then tx.awaitConfirm(preview)', async () => {
    const { awaitConfirm } = setup('0')
    const w = render()
    await w.vm.$nextTick()

    await w.get('[aria-label="deposit qsr"]').trigger('click')
    await w.vm.$nextTick()

    // Faithful port: the panel passes the shortfall in BASE units, not decimal.
    expect(Nom.PrepareDepositQsr).toHaveBeenCalledWith(QSR_REQUIRED)
    expect(awaitConfirm).toHaveBeenCalledWith(depositPreview)
  })

  it('clicking Register (full deposit) calls PrepareRegisterSentinel then tx.awaitConfirm(preview)', async () => {
    // Deposited == required -> the register button is shown.
    const { awaitConfirm } = setup(QSR_REQUIRED)
    const w = render()
    await w.vm.$nextTick()

    await w.get('[aria-label="register sentinel"]').trigger('click')
    await w.vm.$nextTick()

    expect(Nom.PrepareRegisterSentinel).toHaveBeenCalled()
    expect(awaitConfirm).toHaveBeenCalledWith(registerPreview)
  })
})
