import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ autoReceive: false }))
const SetSettings = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetSettings }))
const StartAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const StopAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ StartAutoReceive, StopAutoReceive }))
const onCalls = vi.hoisted(() => [] as Array<[string, (v: unknown) => void]>)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: (n: string, cb: (v: unknown) => void) => onCalls.push([n, cb]) }))
import { useAutoReceiveStore } from './autoReceive'

beforeEach(() => {
  setActivePinia(createPinia())
  GetSettings.mockResolvedValue({ autoReceive: false })
  SetSettings.mockClear()
  StartAutoReceive.mockClear()
  StopAutoReceive.mockClear()
  onCalls.length = 0
})

describe('autoReceive store', () => {
  it('toggle persists the flag and starts/stops the engine', async () => {
    const s = useAutoReceiveStore()
    await s.toggle(0)
    expect(s.enabled).toBe(true)
    expect(SetSettings).toHaveBeenCalledWith(expect.objectContaining({ autoReceive: true }))
    expect(StartAutoReceive).toHaveBeenCalled()
    await s.toggle(0)
    expect(s.enabled).toBe(false)
    expect(StopAutoReceive).toHaveBeenCalled()
  })

  it('init resumes when enabled in settings', async () => {
    GetSettings.mockResolvedValueOnce({ autoReceive: true })
    const s = useAutoReceiveStore()
    await s.init(0)
    expect(s.enabled).toBe(true)
    expect(StartAutoReceive).toHaveBeenCalled()
    expect(s.account).toBe(0)
  })

  it('followAccount re-points only when the account changes', async () => {
    GetSettings.mockResolvedValueOnce({ autoReceive: true })
    const s = useAutoReceiveStore()
    await s.init(0) // account = 0
    StartAutoReceive.mockClear()
    await s.followAccount(0) // same account → no restart
    expect(StartAutoReceive).not.toHaveBeenCalled()
    await s.followAccount(1) // changed → restart
    expect(StartAutoReceive).toHaveBeenCalled()
    expect(s.account).toBe(1)
  })

  it('reflects the backend receiving:active event', () => {
    const s = useAutoReceiveStore()
    s.wireEvents()
    const entry = onCalls.find(([n]) => n === 'auto-receive:active')
    expect(entry).toBeTruthy()
    entry![1](true)
    expect(s.receiving).toBe(true)
    entry![1](false)
    expect(s.receiving).toBe(false)
  })
})
