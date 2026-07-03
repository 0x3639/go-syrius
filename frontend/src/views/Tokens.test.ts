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
  SearchTokens: vi.fn().mockResolvedValue([]),
}))

// token store: stubbed so we can assert search and seed myTokens/results
const refresh = vi.hoisted(() => vi.fn())
const search = vi.hoisted(() => vi.fn())
const clearSearch = vi.hoisted(() => vi.fn())
const tokenState = vi.hoisted(() => ({
  myTokens: [] as any[],
  searchResults: [] as any[],
}))
vi.mock('../stores/token', () => ({
  useTokenStore: () => ({
    get myTokens() { return tokenState.myTokens },
    get searchResults() { return tokenState.searchResults },
    refresh,
    search,
    clearSearch,
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
  // Like the real nom-ui Button, do NOT $emit('click') — the parent's @click
  // falls through to the native <button>. Emitting AND falling through would
  // double-fire handlers (which broke the mint/update toggle behavior).
  Button: { props: ['disabled'], template: '<button :disabled="disabled"><slot/></button>' },
  TokenIcon: { props: ['symbol'], template: '<span>{{ symbol }}</span>' },
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
  search.mockClear()
  clearSearch.mockClear()
  awaitConfirm.mockClear()
  tokenState.myTokens = []
  tokenState.searchResults = []
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

  it('search calls token.search(query)', async () => {
    const w = mount(Tokens)
    await flush()
    await w.find('input[aria-label="token search"]').setValue('zts1abc')
    const searchBtn = w.findAll('button').find((b) => b.text() === 'Search')!
    await searchBtn.trigger('click')
    await flush()
    expect(search).toHaveBeenCalledWith('zts1abc')
  })

  it('search results replace the table; non-owned tokens get Burn but not Mint/Update', async () => {
    tokenState.myTokens = [{
      name: 'Mine', symbol: 'MINE', domain: '', tokenStandard: 'zts1mine', owner: 'z1qactive',
      totalSupply: '100', maxSupply: '200', decimals: 8, isMintable: true, isBurnable: true, isUtility: false,
    }]
    tokenState.searchResults = [{
      name: 'Alpine', symbol: 'ALPN', domain: '', tokenStandard: 'zts1alpine', owner: 'z1qsomeoneelse',
      totalSupply: '5', maxSupply: '9', decimals: 8, isMintable: true, isBurnable: true, isUtility: false,
    }]
    const w = mount(Tokens)
    await flush()
    expect(w.text()).toContain('MINE')

    await w.find('input[aria-label="token search"]').setValue('alp')
    const searchBtn = w.findAll('button').find((b) => b.text() === 'Search')!
    await searchBtn.trigger('click')
    await flush()

    // table now shows the search results, not the owned list
    expect(w.text()).toContain('ALPN')
    expect(w.text()).not.toContain('MINE')
    // not the owner → no Mint/Update; burnable → Burn present
    expect(w.find('button[aria-label="mint ALPN"]').exists()).toBe(false)
    expect(w.find('button[aria-label="update ALPN"]').exists()).toBe(false)
    expect(w.find('button[aria-label="burn ALPN"]').exists()).toBe(true)

    // Clear returns to the owned list
    await w.find('button[aria-label="clear search"]').trigger('click')
    expect(clearSearch).toHaveBeenCalled()
    expect(w.text()).toContain('MINE')
  })

  it('mint after startMint calls PrepareMint(zts, amount, receiver) then awaitConfirm', async () => {
    tokenState.myTokens = [{
      name: 'My Token', symbol: 'MYT', domain: '', tokenStandard: 'zts1mine', owner: 'z1qactive',
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

  it('row Burn opens the inline form and calls PrepareBurn with the row token', async () => {
    tokenState.myTokens = [{
      name: 'My Token', symbol: 'MYT', domain: '', tokenStandard: 'zts1mine', owner: 'z1qactive',
      totalSupply: '100', maxSupply: '200', decimals: 8, isMintable: true, isBurnable: true, isUtility: false,
    }]
    const w = mount(Tokens)
    await flush()

    await w.find('button[aria-label="burn MYT"]').trigger('click')
    await w.find('input[aria-label="row burn amount"]').setValue('7')
    const confirmBurn = w.findAll('button').find((b) => b.text() === 'Confirm burn')!
    await confirmBurn.trigger('click')
    await flush()
    expect(PrepareBurn).toHaveBeenCalledWith('zts1mine', '7')
    expect(awaitConfirm).toHaveBeenCalledWith({ summary: 'burn' })

    // opening Mint closes the burn form (mutual exclusion covers all three)
    await w.find('button[aria-label="mint MYT"]').trigger('click')
    expect(w.find('input[aria-label="row burn amount"]').exists()).toBe(false)
  })

  it('mint and update forms are mutually exclusive, toggle closed, and close via X', async () => {
    tokenState.myTokens = [{
      name: 'My Token', symbol: 'MYT', domain: '', tokenStandard: 'zts1mine', owner: 'z1qactive',
      totalSupply: '100', maxSupply: '200', decimals: 8, isMintable: true, isBurnable: true, isUtility: false,
    }]
    const w = mount(Tokens)
    await flush()

    // open mint, then update — mint must close
    await w.find('button[aria-label="mint MYT"]').trigger('click')
    expect(w.find('input[aria-label="mint amount"]').exists()).toBe(true)
    await w.find('button[aria-label="update MYT"]').trigger('click')
    expect(w.find('input[aria-label="mint amount"]').exists()).toBe(false)
    expect(w.find('input[aria-label="update owner"]').exists()).toBe(true)

    // clicking Update again collapses the row
    await w.find('button[aria-label="update MYT"]').trigger('click')
    expect(w.find('input[aria-label="update owner"]').exists()).toBe(false)

    // the X button closes an open row
    await w.find('button[aria-label="mint MYT"]').trigger('click')
    expect(w.find('input[aria-label="mint amount"]').exists()).toBe(true)
    await w.find('button[aria-label="close MYT actions"]').trigger('click')
    expect(w.find('input[aria-label="mint amount"]').exists()).toBe(false)
  })
})
