import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import MoreTools from './MoreTools.vue'

const push = vi.hoisted(() => vi.fn())
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

describe('MoreTools', () => {
  beforeEach(() => { setActivePinia(createPinia()); push.mockClear() })

  it('lists everyday tools and names the operator tools as Advanced only', () => {
    const text = mount(MoreTools).text()
    for (const s of ['Plasma', 'Address book', 'WalletConnect']) expect(text).toContain(s)
    expect(text).toContain('Advanced mode only')
    for (const s of ['Pillars', 'Sentinels', 'Token issuance', 'Accelerator-Z']) expect(text).toContain(s)
  })

  it('opens a tool by route', async () => {
    const w = mount(MoreTools)
    await w.get('[data-testid="tool-/network/plasma"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/network/plasma')
    await w.get('[data-testid="tool-/network/pillars"]').trigger('click')
    expect(push).toHaveBeenCalledWith('/network/pillars')
  })
})
