import { describe, it, expect, beforeEach, vi } from 'vitest'
import { get } from 'svelte/store'

// tx.ts imports the bound TxService; stub it so the store can load in jsdom.
vi.mock('../../../wailsjs/go/app/TxService', () => ({
  PrepareSend: vi.fn(),
  ConfirmPublish: vi.fn(),
  CancelPending: vi.fn().mockResolvedValue(undefined),
}))

import { tx, resetTx } from './tx'
import { view } from './nav'

describe('tx store', () => {
  beforeEach(() => {
    resetTx()
    view.set('dashboard')
    resetTx() // the view.set above triggers a reset too; normalize to idle
  })

  it('resetTx clears state back to idle', () => {
    tx.set({ status: 'error', preview: null, hash: '', error: 'boom' })
    resetTx()
    const s = get(tx)
    expect(s.status).toBe('idle')
    expect(s.error).toBe('')
    expect(s.preview).toBeNull()
  })

  it('resets a lingering tx result when the view changes (no stale modal on another route)', () => {
    tx.set({ status: 'done', preview: null, hash: '0xdeadbeef', error: '' })
    view.set('tokens')
    const s = get(tx)
    expect(s.status).toBe('idle')
    expect(s.hash).toBe('')
  })
})
