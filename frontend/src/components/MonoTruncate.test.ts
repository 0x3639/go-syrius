import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MonoTruncate from './MonoTruncate.vue'

// jsdom has no layout (clientWidth = 0) and no canvas, so MonoTruncate falls back
// to the full value — which is the correct degraded behavior. The width-aware
// middle-truncation itself is exercised in the running app.
describe('MonoTruncate', () => {
  it('renders the full value when width is unmeasured (no clipping in jsdom)', () => {
    const v = 'z1qxem0123456789abcdefghijklmnop1234amk0'
    const w = mount(MonoTruncate, { props: { value: v } })
    expect(w.text()).toBe(v)
    // The full value is always available on hover, even once truncated.
    expect(w.find('span').attributes('title')).toBe(v)
  })

  it('is null-safe and renders short values whole', () => {
    expect(mount(MonoTruncate, { props: { value: '' } }).text()).toBe('')
    expect(mount(MonoTruncate, { props: { value: 'abc' } }).text()).toBe('abc')
    expect(mount(MonoTruncate, {}).text()).toBe('')
  })
})
