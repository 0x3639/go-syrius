import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import ActionCard from './ActionCard.vue'

describe('ActionCard', () => {
  it('shows a badge when pending and emits click', async () => {
    const w = mount(ActionCard, { props: { label: 'Receive', direction: 'receive', badge: 3 } })
    expect(w.text()).toContain('3')
    expect(w.find('[aria-label="3 pending"]').exists()).toBe(true)
    await w.find('button').trigger('click')
    expect(w.emitted('click')).toBeTruthy()
  })

  it('hides the badge at zero', () => {
    const w = mount(ActionCard, { props: { label: 'Send', direction: 'send', badge: 0 } })
    expect(w.find('[aria-label$="pending"]').exists()).toBe(false)
  })
})
