// stores/token.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const { GetMyTokens, SearchTokens } = vi.hoisted(() => ({
  GetMyTokens: vi.fn(),
  SearchTokens: vi.fn().mockResolvedValue([]),
}))
vi.mock('../../wailsjs/go/app/NomService', () => ({ GetMyTokens, SearchTokens }))

import { useTokenStore } from './token'

beforeEach(() => {
  setActivePinia(createPinia())
  GetMyTokens.mockReset()
})

describe('token store refresh', () => {
  it('populates myTokens on success', async () => {
    GetMyTokens.mockResolvedValue([{ zts: 'zts1a', name: 'A' }])
    const s = useTokenStore()
    await s.refresh()
    expect(s.myTokens).toHaveLength(1)
  })

  it('clears myTokens when the refresh fails (no stale cross-account leak)', async () => {
    GetMyTokens.mockResolvedValue([{ zts: 'zts1a', name: 'A' }])
    const s = useTokenStore()
    await s.refresh()
    // Account switched, then the post-switch refresh fails: the previous
    // account's list must NOT keep rendering as the new account's (CWE-200).
    GetMyTokens.mockRejectedValue(new Error('not connected'))
    await s.refresh()
    expect(s.myTokens).toEqual([])
  })
})
