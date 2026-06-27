import { mount } from '@vue/test-utils'
import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import StatusStrip from './StatusStrip.vue'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'

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

describe('StatusStrip pillar name', () => {
  it('shows the OWNED pillar name (preferred over delegation)', async () => {
    const w = mount(StatusStrip)
    const pillar = usePillarStore()
    pillar.myPillar = { name: 'test' } as never
    pillar.delegation = { name: 'other' } as never
    await w.vm.$nextTick()
    expect(w.text()).toContain('Pillar: test')
  })

  it('falls back to the delegated pillar when none is owned', async () => {
    const w = mount(StatusStrip)
    const pillar = usePillarStore()
    pillar.myPillar = { name: '' } as never
    pillar.delegation = { name: 'other' } as never
    await w.vm.$nextTick()
    expect(w.text()).toContain('Pillar: other')
  })

  it('shows None when neither owned nor delegated', async () => {
    const w = mount(StatusStrip)
    await w.vm.$nextTick()
    expect(w.text()).toContain('Pillar: None')
  })
})
