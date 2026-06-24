import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi.fn().mockResolvedValue([
    { zts: 'z', symbol: 'ZNN', decimals: 8, amount: '5045401869374' }, // 50454.018… -> 50,454
    { zts: 'q', symbol: 'QSR', decimals: 8, amount: '150000000' }, // 1.5
  ]),
}))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button><slot/></button>' },
}))

import Home from './Home.vue'

beforeEach(() => setActivePinia(createPinia()))

describe('Home.vue', () => {
  it('connects and renders balances with the display format rule', async () => {
    const w = mount(Home)
    await new Promise((r) => setTimeout(r))
    // 3+ integer digits -> no decimals + commas; under 100 -> 2dp.
    expect(w.text()).toContain('50,454')
    expect(w.text()).not.toContain('50,454.01')
    expect(w.text()).toContain('1.5')
  })
})
