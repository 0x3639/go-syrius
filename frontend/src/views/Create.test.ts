import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createHash } from 'node:crypto'

const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const GenerateMnemonicWithEntropy = vi.hoisted(() => vi.fn().mockResolvedValue('alpha bravo charlie'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ id: 'abc.dat', name: 'New', baseAddress: 'z1' }))
const Unlock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([]),
  GenerateMnemonic,
  GenerateMnemonicWithEntropy,
  ImportMnemonic,
  Unlock,
  Lock: vi.fn(),
}))
const ClipboardSetText = vi.hoisted(() => vi.fn().mockResolvedValue(true))
const ClipboardGetText = vi.hoisted(() => vi.fn().mockResolvedValue(''))
vi.mock('../../wailsjs/runtime/runtime', () => ({ ClipboardSetText, ClipboardGetText }))
const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ autoReceive: true }))
const SetAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetAutoReceive }))
const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('nom-ui', () => ({
  Card: { template: '<div><slot/></div>' },
  CardContent: { template: '<div><slot/></div>' },
  // `emits: ['click']` keeps the stub faithful to the real nom-ui Button, which
  // has no click emit and receives the handler purely by attribute fallthrough:
  // without it Vue both emits AND falls the listener through to the native
  // <button>, firing every handler twice per click.
  Button: { props: ['disabled'], emits: ['click'], template: '<button :disabled="disabled" @click="$emit(\'click\')"><slot/></button>' },
  Input: {
    props: ['modelValue', 'type'],
    template: '<input :type="type" :aria-label="$attrs[\'aria-label\']" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
  },
}))
import Create from './Create.vue'

const EMPTY_SHA256_B64 = '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='

let randCtr = 0
// Deterministic but *varying* fill — pickDistinctIndexes rejection-samples until
// it has n distinct values, so a constant-valued stub would spin forever.
function counterRandom(arr: ArrayBufferView) {
  const bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength)
  for (let i = 0; i < bytes.length; i++) bytes[i] = randCtr++ & 0xff
  return arr
}
function stubWebCrypto() {
  randCtr = 0
  vi.stubGlobal('crypto', {
    getRandomValues: counterRandom,
    subtle: {
      digest: async (_alg: string, data: Uint8Array) => {
        const h = createHash('sha256')
          .update(Buffer.from(data.buffer, data.byteOffset, data.byteLength))
          .digest()
        return h.buffer.slice(h.byteOffset, h.byteOffset + h.byteLength)
      },
    },
  })
}

const flush = () => new Promise((r) => setTimeout(r))
const btn = (w: ReturnType<typeof mount>, label: string) =>
  w.findAll('button').find((b) => b.text() === label)

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  // afterEach's restoreAllMocks (needed for the performance.now spy) also strips
  // the implementations off the hoisted module mocks, so re-install them here.
  GenerateMnemonic.mockResolvedValue('alpha bravo charlie')
  GenerateMnemonicWithEntropy.mockResolvedValue('alpha bravo charlie')
  ImportMnemonic.mockResolvedValue({ id: 'abc.dat', name: 'New', baseAddress: 'z1' })
  Unlock.mockResolvedValue(undefined)
  ClipboardSetText.mockResolvedValue(true)
  ClipboardGetText.mockResolvedValue('')
  GetSettings.mockResolvedValue({ autoReceive: true })
  SetAutoReceive.mockResolvedValue(undefined)
  stubWebCrypto()
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  // The collection tests spy on performance.now; clearAllMocks alone would
  // leave a spy with no implementation installed for later tests.
  vi.restoreAllMocks()
})

describe('Create.vue — entropy choice', () => {
  it('does not generate any mnemonic at mount', async () => {
    const w = mount(Create)
    await flush()
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    expect(GenerateMnemonicWithEntropy).not.toHaveBeenCalled()
    expect(w.text()).toContain('Generate your recovery phrase')
    expect(btn(w, 'Add interaction randomness')).toBeTruthy()
    expect(btn(w, 'Generate without interaction')).toBeTruthy()
  })

  it('"Generate without interaction" uses enhanced generation with the empty-transcript digest', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    expect(GenerateMnemonicWithEntropy).toHaveBeenCalledTimes(1)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.version).toBe(1)
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
    expect(atob(req.rendererRandomBase64).length).toBe(32)
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    expect(w.text()).toContain('alpha')
  })

  it('enhanced-generation failure surfaces the error and never falls back silently', async () => {
    GenerateMnemonicWithEntropy.mockRejectedValueOnce(new Error('backend exploded'))
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(w.text()).toContain('backend exploded')
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    // still on the choice screen; user can retry
    expect(btn(w, 'Generate without interaction')).toBeTruthy()
  })

  it('a pure backend failure does NOT expose the backend-only action', async () => {
    GenerateMnemonicWithEntropy.mockRejectedValueOnce(new Error('backend exploded'))
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(w.text()).toContain('backend exploded')
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    // Web Crypto worked fine — the fallback is scoped to entropy-request failure
    expect(btn(w, 'Generate using backend randomness')).toBeFalsy()
  })

  it('Web Crypto present but failing also exposes the explicit backend-only action', async () => {
    vi.stubGlobal('crypto', {
      getRandomValues: counterRandom,
      subtle: {
        digest: async () => {
          throw new Error('digest blew up')
        },
      },
    })
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(w.text()).toContain('digest blew up')
    expect(GenerateMnemonic).not.toHaveBeenCalled() // no silent fallback
    const fallback = btn(w, 'Generate using backend randomness')
    expect(fallback).toBeTruthy()
    await fallback!.trigger('click')
    await flush()
    expect(GenerateMnemonic).toHaveBeenCalledTimes(1)
  })

  it('Web Crypto unavailability exposes the explicit backend-only action', async () => {
    vi.stubGlobal('crypto', undefined)
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await flush()
    expect(GenerateMnemonic).not.toHaveBeenCalled() // no silent fallback
    const fallback = btn(w, 'Generate using backend randomness')
    expect(fallback).toBeTruthy()
    await fallback!.trigger('click')
    await flush()
    expect(GenerateMnemonic).toHaveBeenCalledTimes(1)
    expect(w.text()).toContain('alpha')
  })
})

describe('Create.vue — interaction collection', () => {
  function mountCollecting() {
    vi.useFakeTimers()
    let now = 0
    vi.spyOn(performance, 'now').mockImplementation(() => now)
    const w = mount(Create)
    const setNow = (v: number) => { now = v }
    return { w, setNow }
  }

  async function startCollection(w: ReturnType<typeof mount>) {
    await btn(w, 'Add interaction randomness')!.trigger('click')
    return w.find('[aria-label="interaction collection area"]')
  }

  it('starting collection shows the scoped target and a disabled generate button', async () => {
    const { w } = mountCollecting()
    const target = await startCollection(w)
    expect(target.exists()).toBe(true)
    expect(w.text()).toContain('not the keys you press')
    expect(w.text()).toContain('Do not type a password or recovery phrase')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    expect(btn(w, 'Skip interaction')!.attributes('disabled')).toBeUndefined()
  })

  it('pointer collection reaches ready only after samples AND duration; then generates', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    // 40 pointer moves spaced 20ms apart -> >32 accepted samples by t=800ms
    for (let i = 1; i <= 40; i++) {
      setNow(i * 20)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    // samples met, duration not met -> still disabled
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300) // elapsed timer tick
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.interactionDigestBase64).not.toBe(EMPTY_SHA256_B64) // real transcript digested
    expect(w.text()).toContain('alpha')
  })

  it('keyboard-only collection reaches ready state', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 32; i++) {
      setNow(i * 30)
      await target.trigger('keydown', { repeat: false })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeUndefined()
  })

  it('duration alone is not enough — sample target also gates readiness', async () => {
    const { w, setNow } = mountCollecting()
    await startCollection(w)
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()
  })

  it('skip is available before readiness and sends the empty-transcript digest', async () => {
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    setNow(20)
    await target.trigger('pointermove', { clientX: 5, clientY: 6, pointerType: 'mouse' })
    await btn(w, 'Skip interaction')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    const req = GenerateMnemonicWithEntropy.mock.calls[0][0]
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
    expect(w.text()).toContain('alpha')
  })

  it('events outside the collection target are not collected', async () => {
    const { w, setNow } = mountCollecting()
    await startCollection(w)
    setNow(100)
    // dispatch on document body, not the target — no scoped listener fires
    document.body.dispatchEvent(new Event('pointermove'))
    document.body.dispatchEvent(new Event('keydown'))
    expect(w.text()).toContain('0 samples')
  })

  it('a failed generation from the collect stage clears the stale ready state', async () => {
    GenerateMnemonicWithEntropy.mockRejectedValueOnce(new Error('backend exploded'))
    const { w, setNow } = mountCollecting()
    const target = await startCollection(w)
    for (let i = 1; i <= 40; i++) {
      setNow(i * 20)
      await target.trigger('pointermove', { clientX: i * 3, clientY: i * 5, pointerType: 'mouse' })
    }
    setNow(6000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')

    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)

    expect(w.text()).toContain('backend exploded')
    expect(GenerateMnemonic).not.toHaveBeenCalled()
    // The collector was frozen+wiped by the failed attempt: the UI must not
    // still claim readiness, or a second click would submit an empty transcript.
    expect(w.text()).not.toContain('Enough interaction collected')
    expect(w.text()).toContain('0 samples')
    expect(btn(w, 'Generate recovery phrase')!.attributes('disabled')).toBeDefined()

    // Re-collecting works, and the retry carries a real transcript digest.
    for (let i = 1; i <= 40; i++) {
      setNow(6000 + i * 20)
      await target.trigger('pointermove', { clientX: i * 7, clientY: i * 11, pointerType: 'mouse' })
    }
    setNow(13000)
    await vi.advanceTimersByTimeAsync(300)
    expect(w.text()).toContain('Enough interaction collected')
    await btn(w, 'Generate recovery phrase')!.trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(0)
    expect(GenerateMnemonicWithEntropy).toHaveBeenCalledTimes(2)
    expect(GenerateMnemonicWithEntropy.mock.calls[1][0].interactionDigestBase64).not.toBe(EMPTY_SHA256_B64)
    expect(w.text()).toContain('alpha')
  })

  it('unmounting mid-collection clears the elapsed timer and state', async () => {
    const { w } = mountCollecting()
    await startCollection(w)
    w.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('Create.vue — existing flow after generation', () => {
  it('walks phrase -> verify -> password -> import/unlock/dashboard', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    expect(w.text()).toContain('alpha')

    await btn(w, "I've backed it up")!.trigger('click')
    const words = ['alpha', 'bravo', 'charlie']
    for (const input of w.findAll('input')) {
      const label = input.attributes('aria-label') || ''
      const m = label.match(/^word (\d+)$/)
      if (m) await input.setValue(words[Number(m[1]) - 1])
    }
    await btn(w, 'Continue')!.trigger('click')
    await w.find('input[aria-label="wallet name"]').setValue('New')
    await w.find('input[aria-label="password"]').setValue('pw')
    await w.find('input[aria-label="confirm password"]').setValue('pw')
    await btn(w, 'Create wallet')!.trigger('click')
    await flush()

    expect(ImportMnemonic).toHaveBeenCalledWith('New', 'pw', 'alpha bravo charlie')
    expect(Unlock).toHaveBeenCalledWith('abc.dat', 'pw')
    expect(push).toHaveBeenCalledWith('/dashboard')
    expect(SetAutoReceive).toHaveBeenCalledWith(false)
  })

  it('copies the seed phrase to the clipboard', async () => {
    const w = mount(Create)
    await btn(w, 'Generate without interaction')!.trigger('click')
    await flush()
    await w.findAll('button').find((b) => b.text().includes('Copy recovery phrase'))!.trigger('click')
    await flush()
    expect(ClipboardSetText).toHaveBeenCalledWith('alpha bravo charlie')
    expect(w.text()).toContain('Copied')
  })
})
