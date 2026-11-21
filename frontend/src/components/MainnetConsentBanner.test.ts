import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

const push = vi.hoisted(() => vi.fn())
const route = vi.hoisted(() => ({ name: 'transfer', fullPath: '/transfer?to=z1abc' }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }), useRoute: () => route }))

import MainnetConsentBanner from './MainnetConsentBanner.vue'
import { useConsentStore } from '../stores/consent'
import { useNodeStore } from '../stores/node'

function blocked() {
  const node = useNodeStore(); node.connected = true; node.chainId = 1
  const consent = useConsentStore(); consent.setAllowed(false)
  return consent
}

describe('MainnetConsentBanner', () => {
  beforeEach(() => { setActivePinia(createPinia()); push.mockClear(); route.name = 'transfer' })

  it('explains the block and leads to the setting, remembering where the user was', async () => {
    const consent = blocked()
    const w = mount(MainnetConsentBanner)
    const banner = w.get('[data-testid="mainnet-consent-banner"]')
    expect(banner.text()).toContain('Mainnet transactions are off.')
    expect(banner.text()).toContain('real funds')
    await banner.get('button').trigger('click')
    expect(consent.returnTo).toBe('/transfer?to=z1abc')
    expect(push).toHaveBeenCalledWith({ path: '/settings', query: { section: 'mainnet' } })
  })

  it.each([
    ['on testnet', () => { blocked(); useNodeStore().chainId = 73404 }],
    ['when mainnet transactions are allowed', () => { blocked().setAllowed(true) }],
    ['before the setting is known', () => { const n = useNodeStore(); n.connected = true; n.chainId = 1 }],
    ['while disconnected', () => { blocked(); useNodeStore().connected = false }],
    ['on the Settings screen itself', () => { blocked(); route.name = 'settings' }],
  ])('stays hidden %s', (_label, arrange) => {
    arrange()
    expect(mount(MainnetConsentBanner).find('[data-testid="mainnet-consent-banner"]').exists()).toBe(false)
  })
})
