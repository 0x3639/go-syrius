import { mount } from '@vue/test-utils'
import { describe, it, expect } from 'vitest'
import StepHeader from './StepHeader.vue'

describe('StepHeader', () => {
  it('marks earlier steps done, the current step current, later steps todo', () => {
    const w = mount(StepHeader, { props: { current: 2 } })
    const states = w.findAll('[data-state]').map((n) => n.attributes('data-state'))
    expect(states).toEqual(['done', 'current', 'todo'])
  })

  it('labels the three sentinel launch stages', () => {
    const w = mount(StepHeader, { props: { current: 1 } })
    expect(w.text()).toContain('Deposit 50,000 QSR')
    expect(w.text()).toContain('Deposit 5,000 ZNN')
    expect(w.text()).toContain('Sentinel active')
  })
})
