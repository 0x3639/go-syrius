import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  Connect: vi.fn().mockResolvedValue(undefined),
  GetBalances: vi
    .fn()
    .mockResolvedValue([{ zts: 'z', symbol: 'ZNN', decimals: 8, amount: '150000000' }]),
}))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  Button: { template: '<button><slot/></button>' },
  Amount: { props: ['value', 'decimals'], template: '<span class="amount">{{ value }}</span>' },
}))

import Home from './Home.vue'

beforeEach(() => setActivePinia(createPinia()))

describe('Home.vue', () => {
  it('connects and renders a balance', async () => {
    const w = mount(Home)
    await new Promise((r) => setTimeout(r))
    expect(w.find('.amount').exists()).toBe(true)
  })
})
