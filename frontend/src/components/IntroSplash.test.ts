import { mount } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'

// Capture the registered Lottie event handlers so tests can fire them, and the
// destroy spy so we can assert cleanup. lottie-web is the default export.
const handlers: Record<string, () => void> = {}
const destroy = vi.fn()
const loadAnimation = vi.fn((_opts: Record<string, unknown>) => ({
  addEventListener: (name: string, cb: () => void) => {
    handlers[name] = cb
  },
  destroy,
}))
vi.mock('lottie-web', () => ({ default: { loadAnimation } }))
// The JSON is large; stub it so the test stays fast and decoupled from the asset.
vi.mock('../assets/zn-logo.json', () => ({ default: { v: '5.1.13', op: 180 } }))

import IntroSplash from './IntroSplash.vue'

beforeEach(() => {
  for (const k of Object.keys(handlers)) delete handlers[k]
  loadAnimation.mockClear()
  destroy.mockClear()
})

// loadAnimation runs in onMounted via a dynamic import; flush microtasks.
async function mountSplash() {
  const w = mount(IntroSplash, { attachTo: document.body })
  await new Promise((r) => setTimeout(r, 0))
  await w.vm.$nextTick()
  return w
}

// A skip fades immediately and emits `done` after FADE_MS (600ms).
const afterFade = () => new Promise((r) => setTimeout(r, 700))
// A natural complete holds FREEZE_MS (1000ms) then fades FADE_MS (600ms).
const afterFreezeAndFade = () => new Promise((r) => setTimeout(r, 1700))

describe('IntroSplash', () => {
  it('loads the Lottie animation once on mount, play-once', async () => {
    await mountSplash()
    expect(loadAnimation).toHaveBeenCalledTimes(1)
    const arg = loadAnimation.mock.calls[0][0]
    expect(arg.loop).toBe(false)
    expect(arg.autoplay).toBe(true)
  })

  it('holds the last frame before emitting done (freeze)', async () => {
    const w = await mountSplash()
    handlers['complete']?.()
    await afterFade() // 700ms < freeze(1000) — still holding the last frame
    expect(w.emitted('done')).toBeFalsy()
    await afterFreezeAndFade() // now well past freeze + fade
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done after the natural complete (freeze + fade)', async () => {
    const w = await mountSplash()
    handlers['complete']?.()
    await afterFreezeAndFade()
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done on overlay click (skip), no freeze hold', async () => {
    const w = await mountSplash()
    await w.trigger('click')
    await afterFade()
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done on Esc keydown (skip)', async () => {
    const w = await mountSplash()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await afterFade()
    expect(w.emitted('done')).toBeTruthy()
  })

  it('emits done only once even if complete and skip both fire', async () => {
    const w = await mountSplash()
    handlers['complete']?.()
    await w.trigger('click')
    await afterFreezeAndFade()
    expect(w.emitted('done')).toHaveLength(1)
  })
})
