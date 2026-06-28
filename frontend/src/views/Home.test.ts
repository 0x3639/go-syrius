import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

// --- bindings: stub everything Home (and the children we keep) reach for.
// refresh() on mount pulls balances/txs/unreceived through these, so we seed the
// test data via the mocked returns (setting the store directly would be clobbered
// by refresh()). Data is inlined into the factories — vi.mock is hoisted above
// any top-level consts. ---
vi.mock('../../wailsjs/go/app/ConfigService', () => ({
  GetSettings: vi.fn().mockResolvedValue({ autoReceive: false }),
  SetSettings: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([
    { zts: 'zts1znn', symbol: 'ZNN', decimals: 8, amount: '5045401869374' }, // 50454.018… -> 50,454
    { zts: 'zts1qsr', symbol: 'QSR', decimals: 8, amount: '150000000' }, // 1.5
  ]),
  GetTransactions: vi.fn().mockResolvedValue({ records: [], hasMore: false }),
  GetUnreceived: vi.fn().mockResolvedValue([
    { fromHash: 'h1', fromAddress: 'a', token: 't', amount: '1' },
    { fromHash: 'h2', fromAddress: 'b', token: 't', amount: '2' },
  ]),
  StartAutoReceive: vi.fn().mockResolvedValue(undefined),
  StopAutoReceive: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/go/app/NomService', () => ({
  GetPlasmaInfo: vi.fn().mockResolvedValue(null),
  GetDelegation: vi.fn().mockResolvedValue(null),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

// --- router: keep the test component-local. ---
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }), useRoute: () => ({ query: {} }) }))

// --- nom-ui: trivial stubs. Tabs renders all content (so TokensPanel shows);
// Input/TokenIcon support TokensPanel; Dialog* support the child modals. ---
vi.mock('nom-ui', () => ({
  Button: { template: '<button @click="$emit(\'click\')"><slot/></button>' },
  Tabs: { props: ['modelValue'], template: '<div><slot/></div>' },
  TabsList: { template: '<div><slot/></div>' },
  TabsTrigger: { props: ['value'], template: '<button><slot/></button>' },
  TabsContent: { props: ['value'], template: '<div><slot/></div>' },
  Input: {
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template:
      '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
  TokenIcon: { props: ['symbol'], template: '<span>{{ symbol }}</span>' },
  Dialog: { props: ['open'], template: '<div v-if="open"><slot/></div>' },
  DialogContent: { template: '<div><slot/></div>' },
  DialogHeader: { template: '<div><slot/></div>' },
  DialogTitle: { template: '<div><slot/></div>' },
  useToast: () => ({ show: vi.fn() }),
}))

import Home from './Home.vue'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

const STUBS = { TopBar: true, StatusStrip: true, TxHistory: true, SendModal: true, ReceiveModal: true }
const flush = () => new Promise((r) => setTimeout(r))

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockClear()
  // Reset settings to the default (Governance off) so per-test overrides isolate.
  vi.mocked(Cfg.GetSettings).mockResolvedValue({ autoReceive: false } as never)
})

describe('Home.vue', () => {
  it('renders ZNN/QSR balances via formatAmount', async () => {
    const w = mount(Home, { global: { stubs: STUBS } })
    await flush()
    await w.vm.$nextTick()
    expect(w.text()).toContain('50,454')
    expect(w.text()).not.toContain('50,454.01')
    expect(w.text()).toContain('1.5')
  })

  it('shows the Tokens panel in the tabs', async () => {
    const w = mount(Home, { global: { stubs: STUBS } })
    await flush()
    await w.vm.$nextTick()
    // TokensPanel renders a row per token (symbol + zts).
    expect(w.text()).toContain('ZNN')
    expect(w.text()).toContain('zts1znn')
  })

  it('reflects the unreceived count on the Receive ActionCard badge', async () => {
    const w = mount(Home, { global: { stubs: STUBS } })
    await flush()
    await w.vm.$nextTick()
    expect(w.find('[aria-label="2 pending"]').exists()).toBe(true)
  })

  it('hides the Governance tab by default', async () => {
    const w = mount(Home, { global: { stubs: STUBS } })
    await flush()
    await w.vm.$nextTick()
    expect(w.text()).toContain('Accelerator') // base tabs render
    expect(w.text()).not.toContain('Governance')
  })

  it('shows the Governance tab when showGovernance is enabled in settings', async () => {
    vi.mocked(Cfg.GetSettings).mockResolvedValue({ showGovernance: true } as never)
    const w = mount(Home, { global: { stubs: { ...STUBS, GovernancePanel: true } } })
    await flush()
    await w.vm.$nextTick()
    expect(w.text()).toContain('Governance')
  })
})
