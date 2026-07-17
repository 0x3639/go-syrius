import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('nom-ui', () => ({
  Button: { template: '<button @click="$emit(\'click\')"><slot /></button>' },
  Input: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
}))
vi.mock('../../wailsjs/go/app/TxService', () => ({}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({}))
vi.mock('@walletconnect/sign-client', () => ({ SignClient: { init: vi.fn() } }))

import WalletConnect from './WalletConnect.vue'
import { useWalletConnectStore } from '../stores/walletconnect'

beforeEach(() => {
  setActivePinia(createPinia())
  vi.stubEnv('VITE_WALLETCONNECT_PROJECT_ID', 'test-project-id')
})

describe('WalletConnect view untrusted-metadata handling', () => {
  it('never renders peer-provided icon URLs as images', () => {
    const wc = useWalletConnectStore()
    wc.proposal = {
      id: 1,
      name: 'Bridge',
      description: '',
      url: 'https://bridge.example',
      icon: 'http://127.0.0.1:8080/probe.png',
      methods: ['znn_send'],
      events: [],
      raw: {},
      verifiedOrigin: '',
      validation: 'UNKNOWN',
      isScam: false,
    }
    wc.sessions = [{
      topic: 't1',
      name: 'Bridge',
      url: 'https://bridge.example',
      icon: 'http://10.0.0.1/lan-probe.png',
      accounts: ['zenon:1:z1q...'],
    }]

    const w = mount(WalletConnect)

    // Rendering untrusted metadata must not make the privileged WebView issue
    // attacker-chosen requests (loopback/LAN probing, IP disclosure).
    expect(w.findAll('img')).toHaveLength(0)
    expect(w.html()).not.toContain('probe.png')
  })

  it('shows the verified origin for a Verify-validated proposal and a warning otherwise', () => {
    const wc = useWalletConnectStore()
    wc.proposal = {
      id: 2,
      name: 'Bridge',
      description: '',
      url: 'https://bridge.example',
      icon: '',
      methods: ['znn_send'],
      events: [],
      raw: {},
      verifiedOrigin: 'https://bridge.0x3639.com',
      validation: 'VALID',
      isScam: false,
    }
    expect(mount(WalletConnect).text()).toContain('https://bridge.0x3639.com')

    wc.proposal = { ...wc.proposal, verifiedOrigin: '', validation: 'UNKNOWN' }
    expect(mount(WalletConnect).text().toLowerCase()).toContain('not verified')
  })

  it('marks a scam-flagged proposal prominently', () => {
    const wc = useWalletConnectStore()
    wc.proposal = {
      id: 3,
      name: 'Definitely The Real Bridge',
      description: '',
      url: 'https://scam.example',
      icon: '',
      methods: ['znn_send'],
      events: [],
      raw: {},
      verifiedOrigin: 'https://scam.example',
      validation: 'INVALID',
      isScam: true,
    }

    expect(mount(WalletConnect).text().toLowerCase()).toContain('known scam')
  })
})
