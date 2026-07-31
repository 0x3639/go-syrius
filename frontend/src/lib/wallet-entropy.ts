// Phase 7f additive user entropy — canonical interaction transcript, digesting,
// and request assembly (spec docs/superpowers/specs/2026-07-31-additive-user-
// entropy-design.md §8). All values here are contributions mixed by the Go
// backend via HKDF; nothing in this file is a security estimate.

export const POINTER_SAMPLE_INTERVAL_MS = 16
export const COLLECTION_MIN_DURATION_MS = 5_000
export const COLLECTION_TARGET_SAMPLES = 32
export const COLLECTION_MAX_SAMPLES = 2_048

export const KIND_POINTER = 1
export const KIND_TOUCH = 2
export const KIND_KEY = 3

const RECORD_SIZE = 13
const INT32_MIN = -2_147_483_648
const INT32_MAX = 2_147_483_647
const UINT32_MAX = 4_294_967_295

// Matches the Go MnemonicEntropyRequest JSON contract (app/dto.go).
export type EntropyRequest = {
  version: number
  rendererRandomBase64: string
  interactionDigestBase64: string
}

function clampInt32(v: number): number {
  if (!Number.isFinite(v)) return 0
  return Math.min(INT32_MAX, Math.max(INT32_MIN, Math.round(v)))
}

// Non-finite or negative deltas encode as zero rather than throwing (spec §8.1).
function deltaMicros(nowMs: number, prevMs: number | null): number {
  if (prevMs === null) return 0
  const d = (nowMs - prevMs) * 1000
  if (!Number.isFinite(d) || d < 0) return 0
  return Math.min(UINT32_MAX, Math.round(d))
}

// Bounded transcript of fixed 13-byte little-endian records:
//   [0]=kind u8, [1..4]=µs since previous accepted sample u32,
//   [5..8]=A i32, [9..12]=B i32. Key records never carry key identity.
export class InteractionCollector {
  private buf = new Uint8Array(COLLECTION_MAX_SAMPLES * RECORD_SIZE)
  private dv = new DataView(this.buf.buffer)
  private count = 0
  private lastSampleMs: number | null = null
  private lastPointerMs: number | null = null
  private frozen = false

  get sampleCount(): number {
    return this.count
  }

  get isFull(): boolean {
    return this.count >= COLLECTION_MAX_SAMPLES
  }

  addPointerSample(x: number, y: number, nowMs: number, pointerType = 'mouse'): boolean {
    if (this.frozen || this.isFull) return false
    if (this.lastPointerMs !== null && nowMs - this.lastPointerMs < POINTER_SAMPLE_INTERVAL_MS) return false
    const kind = pointerType === 'touch' ? KIND_TOUCH : KIND_POINTER
    this.append(kind, deltaMicros(nowMs, this.lastSampleMs), clampInt32(x * 1024), clampInt32(y * 1024))
    this.lastPointerMs = nowMs
    this.lastSampleMs = nowMs
    return true
  }

  addKeySample(nowMs: number, repeat = false): boolean {
    if (this.frozen || this.isFull || repeat) return false
    this.append(KIND_KEY, deltaMicros(nowMs, this.lastSampleMs), 0, 0)
    this.lastSampleMs = nowMs
    return true
  }

  private append(kind: number, delta: number, a: number, b: number): void {
    const off = this.count * RECORD_SIZE
    this.dv.setUint8(off, kind)
    this.dv.setUint32(off + 1, delta, true)
    this.dv.setInt32(off + 5, a, true)
    this.dv.setInt32(off + 9, b, true)
    this.count++
  }

  /** Stop accepting events and return a copy of the exact transcript bytes. */
  freeze(): Uint8Array {
    this.frozen = true
    return this.buf.slice(0, this.count * RECORD_SIZE)
  }

  /** Best-effort wipe of the sample buffer; restarts all collection state. */
  reset(): void {
    this.buf.fill(0)
    this.count = 0
    this.lastSampleMs = null
    this.lastPointerMs = null
    this.frozen = false
  }
}

export function webCryptoAvailable(): boolean {
  const c = globalThis.crypto as Crypto | undefined
  return (
    !!c &&
    typeof c.getRandomValues === 'function' &&
    !!c.subtle &&
    typeof c.subtle.digest === 'function'
  )
}

export function toBase64(bytes: Uint8Array): string {
  let s = ''
  for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i])
  return btoa(s)
}

export function wipe(...buffers: Uint8Array[]): void {
  for (const b of buffers) b.fill(0)
}

/**
 * Digest the frozen transcript, draw the fresh 32-byte renderer contribution
 * (at finalization, never at mount — spec §8.3), and assemble the version-1
 * request. Mutable buffers, including the caller's transcript, are wiped in
 * `finally`. Throws when Web Crypto is unavailable or fails — the caller must
 * surface that and offer the explicit backend-only action, never silently
 * fall back (spec §8.4).
 */
export async function createEntropyRequest(transcript: Uint8Array): Promise<EntropyRequest> {
  let digest: Uint8Array | null = null
  let renderer: Uint8Array | null = null
  try {
    if (!webCryptoAvailable()) throw new Error('Enhanced generation is unavailable: this environment has no Web Crypto support')
    // TS 5.7's BufferSource only admits ArrayBuffer-backed views; the transcript
    // is always ArrayBuffer-backed (never SharedArrayBuffer), so widen it here.
    digest = new Uint8Array(await crypto.subtle.digest('SHA-256', transcript as BufferSource))
    renderer = crypto.getRandomValues(new Uint8Array(32))
    return {
      version: 1,
      rendererRandomBase64: toBase64(renderer),
      interactionDigestBase64: toBase64(digest),
    }
  } finally {
    if (digest) wipe(digest)
    if (renderer) wipe(renderer)
    wipe(transcript)
  }
}

/**
 * n distinct uniform indexes in [0, max), sorted ascending, via rejection
 * sampling over crypto.getRandomValues (replaces Math.random in the wallet-
 * creation audit surface — spec §9.4). Backup positions are a UX check, not
 * entropy: in the degenerate no-Web-Crypto environment (only reachable on the
 * explicit backend-only path) fixed spread positions are used instead.
 */
export function pickDistinctIndexes(n: number, max: number): number[] {
  if (n > max) throw new Error('not enough positions to pick from')
  const c = globalThis.crypto as Crypto | undefined
  if (!c || typeof c.getRandomValues !== 'function') {
    return Array.from({ length: n }, (_, i) => Math.floor((i * (max - 1)) / Math.max(1, n - 1)))
  }
  const picked = new Set<number>()
  const buf = new Uint32Array(1)
  const limit = Math.floor(0x1_0000_0000 / max) * max
  while (picked.size < n) {
    c.getRandomValues(buf)
    if (buf[0] >= limit) continue
    picked.add(buf[0] % max)
  }
  return [...picked].sort((a, b) => a - b)
}
