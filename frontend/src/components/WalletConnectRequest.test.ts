import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

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
vi.mock('../../wailsjs/go/app/TxService', () => ({ CancelPending: vi.fn() }))
vi.mock('../../wailsjs/go/app/NodeService', () => ({}))
vi.mock('@walletconnect/sign-client', () => ({ SignClient: { init: vi.fn() } }))

import WalletConnectRequest from './WalletConnectRequest.vue'
import { useWalletConnectStore, type WalletConnectRequest as WcRequest } from '../stores/walletconnect'

function request(overrides: Partial<WcRequest> = {}): WcRequest {
  return {
    topic: 'topic',
    id: 1,
    dapp: 'Zenon Bridge',
    preview: {
      fromAddress: 'z1qsender',
      toAddress: 'z1qxemdeddedxbridgexxxxxxxxxxxxxxxs6f5v0',
      symbol: 'WEIRD',
      zts: 'zts1customtoken',
      amount: '100000000',
      decimals: 8,
      usedPlasma: 0,
      difficulty: 0,
      hash: '',
      needsPoW: false,
      summary: 'Bridge.WrapToken',
      holdId: 7,
    },
    status: 'awaiting',
    error: '',
    publishedResult: null,
    publishedHash: '',
    sessionEnded: false,
    verifiedOrigin: '',
    validation: 'UNKNOWN',
    isScam: false,
    ...overrides,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('WalletConnectRequest confirm-what-you-sign rendering', () => {
  it('always shows the exact base-unit amount and ZTS from the held block', () => {
    const wc = useWalletConnectStore()
    wc.request = request()

    const w = mount(WalletConnectRequest)

    // The human rendering depends on node-reported decimals; the base-unit
    // integer is the block's authoritative amount and must always be visible.
    expect(w.text()).toContain('100000000')
    expect(w.text()).toContain('zts1customtoken')
    expect(w.text()).toContain('base units')
  })

  it('shows the verified origin when Verify validated the dapp', () => {
    const wc = useWalletConnectStore()
    wc.request = request({ verifiedOrigin: 'https://bridge.0x3639.com', validation: 'VALID' })

    const w = mount(WalletConnectRequest)

    expect(w.text()).toContain('https://bridge.0x3639.com')
  })

  it('renders the unknown-outcome state with reconcile-only actions', () => {
    const wc = useWalletConnectStore()
    wc.request = request({ status: 'unknown', publishedHash: 'maybe-hash', error: 'walletconnect publication outcome unknown: timeout' })

    const w = mount(WalletConnectRequest)

    expect(w.text()).toContain('maybe-hash')
    expect(w.text().toLowerCase()).toContain('check outcome')
    expect(w.text().toLowerCase()).not.toContain('approve and publish')
    expect(w.text().toLowerCase()).not.toContain('reject')
  })

  it('warns when the dapp origin is not verified', () => {
    const wc = useWalletConnectStore()
    wc.request = request({ validation: 'UNKNOWN' })

    const w = mount(WalletConnectRequest)

    expect(w.text().toLowerCase()).toContain('not verified')
  })
})
