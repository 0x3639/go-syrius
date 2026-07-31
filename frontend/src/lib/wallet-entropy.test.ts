import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createHash } from 'node:crypto'
import {
  InteractionCollector,
  createEntropyRequest,
  pickDistinctIndexes,
  toBase64,
  wipe,
  webCryptoAvailable,
  POINTER_SAMPLE_INTERVAL_MS,
  COLLECTION_MAX_SAMPLES,
  KIND_POINTER,
  KIND_TOUCH,
  KIND_KEY,
} from './wallet-entropy'

const EMPTY_SHA256_B64 = '47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='

// Deterministic Web Crypto stub (spec §11.2: no ambient jsdom randomness).
// getRandomValues MUST vary across calls — a constant stub would spin
// pickDistinctIndexes' rejection-sampling loop forever.
let randCtr = 0
function stubWebCrypto() {
  randCtr = 0
  vi.stubGlobal('crypto', {
    getRandomValues: (arr: ArrayBufferView) => {
      const bytes = new Uint8Array(arr.buffer, arr.byteOffset, arr.byteLength)
      for (let i = 0; i < bytes.length; i++) bytes[i] = randCtr++ & 0xff
      return arr
    },
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

beforeEach(stubWebCrypto)
afterEach(() => vi.unstubAllGlobals())

function view(transcript: Uint8Array) {
  return new DataView(transcript.buffer, transcript.byteOffset, transcript.byteLength)
}

describe('InteractionCollector record format', () => {
  it('encodes a pointer record as 13 little-endian bytes', () => {
    const c = new InteractionCollector()
    expect(c.addPointerSample(10.4, 20.6, 100)).toBe(true)
    expect(c.addPointerSample(30, 40, 120)).toBe(true)
    const t = c.freeze()
    expect(t.length).toBe(26)
    const v = view(t)
    expect(v.getUint8(0)).toBe(KIND_POINTER)
    expect(v.getUint32(1, true)).toBe(0) // first record has no previous sample
    expect(v.getInt32(5, true)).toBe(Math.round(10.4 * 1024)) // 10650
    expect(v.getInt32(9, true)).toBe(Math.round(20.6 * 1024)) // 21094
    // second record: 20 ms after the previous accepted sample = 20000 µs
    expect(v.getUint8(13)).toBe(KIND_POINTER)
    expect(v.getUint32(14, true)).toBe(20_000)
  })

  it('uses kind 2 for touch and kind 3 for keys with zeroed positions', () => {
    const c = new InteractionCollector()
    c.addPointerSample(1, 2, 0, 'touch')
    c.addKeySample(20)
    const t = c.freeze()
    const v = view(t)
    expect(v.getUint8(0)).toBe(KIND_TOUCH)
    expect(v.getUint8(13)).toBe(KIND_KEY)
    expect(v.getInt32(18, true)).toBe(0)
    expect(v.getInt32(22, true)).toBe(0)
  })

  it('serializes identically for different keys with the same timing (no key identity)', () => {
    // The API cannot even receive key identity; equal timings ⇒ equal bytes.
    const a = new InteractionCollector()
    a.addKeySample(5)
    a.addKeySample(30)
    const b = new InteractionCollector()
    b.addKeySample(5)
    b.addKeySample(30)
    expect(a.freeze()).toEqual(b.freeze())
  })

  it('ignores repeated keydown', () => {
    const c = new InteractionCollector()
    expect(c.addKeySample(5, true)).toBe(false)
    expect(c.sampleCount).toBe(0)
  })

  it('throttles pointer samples to one per 16 ms; keys are not throttled', () => {
    const c = new InteractionCollector()
    expect(c.addPointerSample(1, 1, 0)).toBe(true)
    expect(c.addPointerSample(2, 2, 10)).toBe(false)
    expect(c.addPointerSample(3, 3, POINTER_SAMPLE_INTERVAL_MS)).toBe(true)
    expect(c.addKeySample(POINTER_SAMPLE_INTERVAL_MS + 1)).toBe(true)
    expect(c.addKeySample(POINTER_SAMPLE_INTERVAL_MS + 2)).toBe(true)
    expect(c.sampleCount).toBe(4)
  })

  it('encodes non-finite/negative timing as zero and clamps large deltas to uint32', () => {
    // Order matters: the NaN sample must come last — it poisons lastSampleMs,
    // so any record after it would encode delta 0 regardless of clamping.
    const c = new InteractionCollector()
    c.addKeySample(1000)
    c.addKeySample(500) // negative delta -> 0
    c.addKeySample(500 + 5_000_000) // 5e9 µs > uint32 max -> clamp
    c.addKeySample(Number.NaN) // non-finite -> 0
    const v = view(c.freeze())
    expect(v.getUint32(14, true)).toBe(0)
    expect(v.getUint32(27, true)).toBe(4_294_967_295)
    expect(v.getUint32(40, true)).toBe(0)
  })

  it('clamps coordinates to the signed 32-bit range', () => {
    const c = new InteractionCollector()
    c.addPointerSample(3e9, -3e9, 0)
    const v = view(c.freeze())
    expect(v.getInt32(5, true)).toBe(2_147_483_647)
    expect(v.getInt32(9, true)).toBe(-2_147_483_648)
  })

  it('stops appending at 2,048 records', () => {
    const c = new InteractionCollector()
    for (let i = 0; i < COLLECTION_MAX_SAMPLES + 10; i++) c.addKeySample(i * 2)
    expect(c.sampleCount).toBe(COLLECTION_MAX_SAMPLES)
    expect(c.isFull).toBe(true)
    expect(c.addKeySample(99_999)).toBe(false)
    expect(c.freeze().length).toBe(COLLECTION_MAX_SAMPLES * 13)
  })

  it('reset wipes the buffer and restarts state', () => {
    const c = new InteractionCollector()
    c.addPointerSample(9, 9, 0)
    const before = c.freeze()
    expect(before.some((b) => b !== 0)).toBe(true)
    c.reset()
    expect(c.sampleCount).toBe(0)
    expect(c.isFull).toBe(false)
    expect(c.freeze().length).toBe(0)
    // reset also restarts throttle/delta state: a new first sample is accepted
    c.reset()
    expect(c.addPointerSample(1, 1, 5)).toBe(true)
  })
})

describe('createEntropyRequest', () => {
  it('digests the empty transcript to the standard SHA-256 value', async () => {
    const req = await createEntropyRequest(new Uint8Array(0))
    expect(req.version).toBe(1)
    expect(req.interactionDigestBase64).toBe(EMPTY_SHA256_B64)
  })

  it('produces a 32-byte renderer value in padded base64', async () => {
    const req = await createEntropyRequest(new Uint8Array(0))
    const decoded = atob(req.rendererRandomBase64)
    expect(decoded.length).toBe(32)
    expect(req.rendererRandomBase64.endsWith('=')).toBe(true)
  })

  it('wipes the caller transcript buffer even on success', async () => {
    const transcript = new Uint8Array([1, 2, 3, 4])
    await createEntropyRequest(transcript)
    expect([...transcript]).toEqual([0, 0, 0, 0])
  })

  it('throws when Web Crypto is unavailable', async () => {
    vi.stubGlobal('crypto', undefined)
    expect(webCryptoAvailable()).toBe(false)
    await expect(createEntropyRequest(new Uint8Array(0))).rejects.toThrow()
  })
})

describe('encoding helpers', () => {
  it('round-trips all byte values through base64', () => {
    const all = new Uint8Array(256)
    for (let i = 0; i < 256; i++) all[i] = i
    const decoded = atob(toBase64(all))
    expect(decoded.length).toBe(256)
    for (let i = 0; i < 256; i++) expect(decoded.charCodeAt(i)).toBe(i)
  })

  it('wipe zeroes every buffer passed', () => {
    const a = new Uint8Array([1, 2])
    const b = new Uint8Array([3])
    wipe(a, b)
    expect([...a]).toEqual([0, 0])
    expect([...b]).toEqual([0])
  })
})

describe('pickDistinctIndexes', () => {
  it('returns three unique in-range sorted indexes', () => {
    const idx = pickDistinctIndexes(3, 24)
    expect(idx).toHaveLength(3)
    expect(new Set(idx).size).toBe(3)
    for (const i of idx) expect(i >= 0 && i < 24).toBe(true)
    expect([...idx].sort((x, y) => x - y)).toEqual(idx)
  })

  it('works without Web Crypto via the deterministic spread fallback', () => {
    vi.stubGlobal('crypto', undefined)
    expect(pickDistinctIndexes(3, 24)).toEqual([0, 11, 23])
  })
})
