import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const PrepareIssueToken = vi.hoisted(() => vi.fn().mockResolvedValue({ summary: 'issue' }))
const PrepareMint = vi.hoisted(() => vi.fn().mockResolvedValue({ summary: 'mint' }))
const PrepareBurn = vi.hoisted(() => vi.fn().mockResolvedValue({ summary: 'burn' }))
const PrepareUpdateToken = vi.hoisted(() => vi.fn().mockResolvedValue({ summary: 'update' }))
vi.mock('../../wailsjs/go/app/NomService', () => ({
  PrepareIssueToken,
  PrepareMint,
  PrepareBurn,
  PrepareUpdateToken,
  GetMyTokens: vi.fn().mockResolvedValue([]),
  GetTokenByZts: vi.fn().mockResolvedValue(null),
}))

// token store: stubbed so we can assert lookup and seed myTokens
const refresh = vi.hoisted(() => vi.fn())
const lookup = vi.hoisted(() => vi.fn())
const tokenState = vi.hoisted(() => ({
  myTokens: [] as any[],
  lookedUp: null as any,
}))
vi.mock('../stores/token', () => ({
  useTokenStore: () => ({
    get myTokens() { return tokenState.myTokens },
    get lookedUp() { return tokenState.lookedUp },
    refresh,
    lookup,
  }),
}))

// tx store: spy awaitConfirm
const awaitConfirm = vi.hoisted(() => vi.fn())
vi.mock('../stores/tx', () => ({
  useTxStore: () => ({ status: 'idle', preview: null, error: '', awaitConfirm }),
}))

vi.mock('../stores/wallet', () => ({
  useWalletStore: () => ({ activeAddress: () => 'z1qactive' }),
}))

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

vi.mock('../components/NomConfirm.vue', () => ({ default: { template: '<div data-test="nom-confirm" />' } }))

vi.mock('nom-ui', () => ({
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  Button: { props: ['disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Table: { template: '<table><slot/></table>' },
  TableHeader: { template: '<thead><slot/></thead>' },
  TableBody: { template: '<tbody><slot/></tbody>' },
  TableRow: { template: '<tr><slot/></tr>' },
  TableHead: { template: '<th><slot/></th>' },
  TableCell: { props: ['colspan'], template: '<td :colspan="colspan"><slot/></td>' },
  TableEmpty: { props: ['colspan'], template: '<tr><td :colspan="colspan"><slot/></td></tr>' },
}))

import Tokens from './Tokens.vue'

const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
  PrepareIssueToken.mockClear()
  PrepareMint.mockClear()
  PrepareBurn.mockClear()
  PrepareUpdateToken.mockClear()
  refresh.mockClear()
  lookup.mockClear()
  awaitConfirm.mockClear()
  tokenState.myTokens = []
  tokenState.lookedUp = null
})

describe('Tokens.vue', () => {
  it('refreshes tokens on mount', async () => {
    mount(Tokens)
    await flush()
    expect(refresh).toHaveBeenCalled()
  })

  it('issue calls PrepareIssueToken then awaitConfirm', async () => {
    const w = mount(Tokens)
    await flush()
    await w.find('input[aria-label="issue name"]').setValue('My Token')
    await w.find('input[aria-label="issue symbol"]').setValue('MYT')
    await w.find('input[aria-label="issue total"]').setValue('100')
    await w.find('input[aria-label="issue max"]').setValue('200')
    await w.find('button[aria-label="issue token"]').trigger('click')
    await flush()

    expect(PrepareIssueToken).toHaveBeenCalledWith('My Token', 'MYT', '', '100', '200', 8, true, true, false)
    expect(awaitConfirm).toHaveBeenCalledWith({ summary: 'issue' })
  })

  it('lookup calls token.lookup(zts)', async () => {
    const w = mount(Tokens)
    await flush()
    await w.find('input[aria-label="lookup zts"]').setValue('zts1abc')
    const lookupBtn = w.findAll('button').find((b) => b.text() === 'Look up')!
    await lookupBtn.trigger('click')
    await flush()
    expect(lookup).toHaveBeenCalledWith('zts1abc')
  })

  it('mint after startMint calls PrepareMint(zts, amount, receiver) then awaitConfirm', async () => {
    tokenState.myTokens = [{
      name: 'My Token', symbol: 'MYT', domain: '', tokenStandard: 'zts1mine', owner: 'z1qowner',
      totalSupply: '100', maxSupply: '200', decimals: 8, isMintable: true, isBurnable: true, isUtility: false,
    }]
    const w = mount(Tokens)
    await flush()

    // open the inline mint form (receiver defaults to activeAddress)
    await w.find('button[aria-label="mint MYT"]').trigger('click')
    await w.find('input[aria-label="mint amount"]').setValue('42')
    // receiver already prefilled to z1qactive
    const confirmMint = w.findAll('button').find((b) => b.text() === 'Confirm mint')!
    await confirmMint.trigger('click')
    await flush()

    expect(PrepareMint).toHaveBeenCalledWith('zts1mine', '42', 'z1qactive')
    expect(awaitConfirm).toHaveBeenCalledWith({ summary: 'mint' })
  })
})
