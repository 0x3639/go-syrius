import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const GetNodeConfig = vi.hoisted(() => vi.fn().mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://old', localUrl: 'ws://127.0.0.1:35998' }))
const GetEmbeddedInfo = vi.hoisted(() => vi.fn().mockResolvedValue({ running: false, dataDir: '', sizeBytes: 0 }))
const SetNodeMode = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const SetNodeURL = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([]),
  GetNodeConfig,
  GetEmbeddedInfo,
  SetNodeMode,
  SetNodeURL,
  DeleteEmbeddedData: vi.fn().mockResolvedValue(undefined),
}))

const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ chainId: 73404 }))
const SetChainID = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const SetShowGovernance = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetChainID, SetShowGovernance }))

const RevealMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie delta'))
const ChangePassword = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const RenameWallet = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const ListWallets = vi.hoisted(() => vi.fn().mockResolvedValue([{ id: 'abc.dat', name: 'Main', baseAddress: 'z1' }]))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets,
  RevealMnemonic,
  ChangePassword,
  RenameWallet,
  Lock: vi.fn(),
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

vi.mock('nom-ui', () => ({
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
}))

import Settings from './Settings.vue'
import { useNodeStore } from '../stores/node'

const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
  SetNodeMode.mockClear()
  SetNodeURL.mockClear()
  ChangePassword.mockClear()
  RevealMnemonic.mockClear()
  RenameWallet.mockClear()
  GetSettings.mockClear()
  SetChainID.mockClear()
  SetShowGovernance.mockClear()
  GetSettings.mockResolvedValue({ chainId: 73404 })
})

describe('Settings.vue', () => {
  it('applies an edited remote URL then changed mode in order', async () => {
    const w = mount(Settings)
    await flush() // onMounted getConfig + refreshEmbedded

    // Edit the remote URL (marks remoteDirty)
    await w.find('input[aria-label="wss endpoint url"]').setValue('wss://new')
    // Change the mode to local (marks modeDirty)
    await w.find('input[type="radio"][value="local"]').setValue()

    await w.find('button[aria-label="Apply node"]').trigger('click')
    await flush()

    expect(SetNodeURL).toHaveBeenCalledWith('remote', 'wss://new')
    expect(SetNodeMode).toHaveBeenCalledWith('local')
    // URL applied before mode
    expect(SetNodeURL.mock.invocationCallOrder[0]).toBeLessThan(SetNodeMode.mock.invocationCallOrder[0])
  })

  it('reveals the mnemonic with a password then Hide clears it', async () => {
    const w = mount(Settings)
    await flush()

    await w.find('input[aria-label="reveal password"]').setValue('pw')
    await w.findAll('button').find((b) => b.text() === 'Reveal')!.trigger('click')
    await flush()

    expect(RevealMnemonic).toHaveBeenCalledWith('pw')
    expect(w.text()).toContain('alpha bravo charlie delta')

    await w.findAll('button').find((b) => b.text() === 'Hide')!.trigger('click')
    await flush()
    expect(w.text()).not.toContain('alpha bravo charlie delta')
  })

  it('change-password with matching new/confirm calls wallet.changePassword(old, new)', async () => {
    const w = mount(Settings)
    await flush()

    await w.find('input[aria-label="current password"]').setValue('old')
    await w.find('input[aria-label="new password"]').setValue('newpw')
    await w.find('input[aria-label="confirm new password"]').setValue('newpw')
    await w.findAll('button').find((b) => b.text() === 'Change')!.trigger('click')
    await flush()

    expect(ChangePassword).toHaveBeenCalledWith('abc.dat', 'old', 'newpw')
    expect(w.text()).toContain('Password changed')
  })

  it('seeds the wallet name and Rename calls wallet.rename(activeId, newName)', async () => {
    const w = mount(Settings)
    await flush() // onMounted: loadWallets seeds active + name

    const field = w.find('input[aria-label="wallet name"]')
    expect((field.element as HTMLInputElement).value).toBe('Main')

    await field.setValue('Renamed')
    await w.findAll('button').find((b) => b.text() === 'Rename')!.trigger('click')
    await flush()

    expect(RenameWallet).toHaveBeenCalledWith('abc.dat', 'Renamed')
    expect(w.text()).toContain('Wallet name updated')
  })

  it('loads the chain id from GetSettings and Apply read-modify-writes it', async () => {
    const w = mount(Settings)
    await flush() // onMounted: getConfig + GetSettings

    const field = w.find('input[aria-label="chain id"]')
    expect((field.element as HTMLInputElement).value).toBe('73404')

    await field.setValue('1')
    await w.findAll('button').find((b) => b.text() === 'Apply network')!.trigger('click')
    await flush()

    // read-modify-write: GetSettings object merged with the entered chain id
    expect(SetChainID).toHaveBeenCalledWith(1)
    expect(w.text()).toContain('Network configuration applied')
  })

  it('toggling Show Governance persists via the targeted setter', async () => {
    const w = mount(Settings)
    await flush() // onMounted: ui.init() loads showGovernance (absent → false)

    const cb = w.find('input[aria-label="show governance"]')
    expect((cb.element as HTMLInputElement).checked).toBe(false)

    await cb.setValue(true)
    await flush()

    expect(SetShowGovernance).toHaveBeenCalledWith(true)
  })

  it('renders a mismatch warning when the node chain differs from the configured chain', async () => {
    const node = useNodeStore()
    node.connected = true
    node.chainId = 1 // connected to mainnet, configured is testnet (73404)

    const w = mount(Settings)
    await flush()

    expect(w.text()).toContain("differs from the connected node's chain 1")
  })
})
