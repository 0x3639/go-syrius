import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import TokenAmount from './TokenAmount.vue'

it('exposes exact text to assistive technology and a rounded visual amount', () => {
  const w = mount(TokenAmount, { props: { amount: '123456789', decimals: 8, symbol: 'ZNN' } })
  expect(w.get('[aria-hidden="true"]').text()).toBe('1.23 ZNN')
  expect(w.get('.sr-only').text()).toBe('1.23456789 ZNN')
  expect(w.attributes('title')).toBe('1.23456789 ZNN')
})
