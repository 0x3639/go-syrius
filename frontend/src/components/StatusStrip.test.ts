import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import StatusStrip from './StatusStrip.vue'
import { usePlasmaStore } from '../stores/plasma'

beforeEach(() => setActivePinia(createPinia()))

function mountWithPlasma(currentPlasma: number) {
  const w = mount(StatusStrip)
  const plasma = usePlasmaStore()
  // Cast: only currentPlasma is read by plasmaLevel().
  plasma.info = { currentPlasma } as unknown as typeof plasma.info
  return w
}

describe('StatusStrip plasmaLevel', () => {
  it.each([
    [84000, 'High'],
    [21000, 'Medium'],
    [1, 'Low'],
    [0, 'None'],
  ])('maps currentPlasma %i to %s', async (p, level) => {
    const w = mountWithPlasma(p)
    await w.vm.$nextTick()
    expect(w.text()).toContain(level)
  })
})
