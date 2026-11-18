import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'

const GetSettings = vi.hoisted(() => vi.fn().mockResolvedValue({ autoReceive: false }))
const SetAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/ConfigService', () => ({ GetSettings, SetAutoReceive }))
const StartAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const StopAutoReceive = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/NodeService', () => ({ StartAutoReceive, StopAutoReceive }))
const onCalls = vi.hoisted(() => [] as Array<[string, (v: unknown) => void]>)
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: (n: string, cb: (v: unknown) => void) => onCalls.push([n, cb]) }))
import { useAutoReceiveStore } from './autoReceive'

beforeEach(() => {
  setActivePinia(createPinia())
  GetSettings.mockReset().mockResolvedValue({ autoReceive: false })
  SetAutoReceive.mockReset().mockResolvedValue(undefined)
  StartAutoReceive.mockReset().mockResolvedValue(undefined)
  StopAutoReceive.mockReset().mockResolvedValue(undefined)
  onCalls.length = 0
})

function deferred<T = void>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

describe('autoReceive store', () => {
  it('toggle persists the flag and starts/stops the engine', async () => {
    const s = useAutoReceiveStore()
    await s.toggle(0)
    expect(s.enabled).toBe(true)
    expect(SetAutoReceive).toHaveBeenCalledWith(true)
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

  it('re-arms the same account after connection recovery while locked', async () => {
    GetSettings.mockResolvedValue({ autoReceive: true })
    const s = useAutoReceiveStore()
    await s.init(0)
    StartAutoReceive.mockClear()
    StopAutoReceive.mockClear()
    // The backend worker was stopped with the lost connection while the
    // authenticated shell was absent. Its frontend account value survives.
    await s.init(0)
    expect(StopAutoReceive).toHaveBeenCalledTimes(1)
    expect(StartAutoReceive).toHaveBeenCalledTimes(1)
    expect(s.account).toBe(0)
  })

  it('does not enable auto-receive on unlock when the preference is off', async () => {
    const s = useAutoReceiveStore()
    s.account = 0
    await s.init(0)
    expect(StartAutoReceive).not.toHaveBeenCalled()
    expect(s.enabled).toBe(false)
  })

  it('does not start when disabled while the unlock stop is pending', async () => {
    GetSettings.mockResolvedValue({ autoReceive: true })
    const stopped = deferred()
    StopAutoReceive.mockReturnValueOnce(stopped.promise)
    const s = useAutoReceiveStore()
    const unlock = s.init(0)
    await flushPromises()
    expect(StopAutoReceive).toHaveBeenCalledTimes(1)
    const off = s.toggle(0)
    expect(s.enabled).toBe(false)
    stopped.resolve()
    await Promise.all([unlock, off])
    expect(StartAutoReceive).not.toHaveBeenCalled()
    expect(StopAutoReceive).toHaveBeenCalledTimes(2)
    expect(s.account).toBe(-1)
    expect(s.enabled).toBe(false)
  })

  it('stops an already-dispatched start before completing the off toggle', async () => {
    const started = deferred()
    StartAutoReceive.mockReturnValueOnce(started.promise)
    const s = useAutoReceiveStore()
    s.enabled = true
    const starting = s.start(0)
    await flushPromises()
    expect(StartAutoReceive).toHaveBeenCalledTimes(1)
    const off = s.toggle(0)
    started.resolve()
    await Promise.all([starting, off])
    expect(StopAutoReceive).toHaveBeenCalledTimes(2)
    expect(s.enabled).toBe(false)
    expect(s.account).toBe(-1)
  })

  it.each([false, true])('serializes account changes and follows the latest account (switch back: %s)', async (switchBack) => {
    let activeAccount = switchBack ? 1 : 0
    let workerAccount = -1
    const started = deferred()
    StartAutoReceive.mockImplementationOnce(() => {
      workerAccount = activeAccount
      return started.promise
    }).mockImplementation(async () => { workerAccount = activeAccount })
    StopAutoReceive.mockImplementation(async () => { workerAccount = -1 })
    const s = useAutoReceiveStore()
    s.enabled = true
    s.account = switchBack ? 0 : -1
    const first = s.start(activeAccount)
    await flushPromises()
    activeAccount = switchBack ? 0 : 1
    const latest = s.followAccount(activeAccount)
    await flushPromises()
    expect(StartAutoReceive).toHaveBeenCalledTimes(1)
    started.resolve()
    await Promise.all([first, latest])
    expect(StartAutoReceive).toHaveBeenCalledTimes(2)
    expect(s.account).toBe(activeAccount)
    expect(workerAccount).toBe(activeAccount)
  })

  it('uses the latest account when settings return after an account switch', async () => {
    const settings = deferred<{ autoReceive: boolean }>()
    GetSettings.mockReturnValueOnce(settings.promise)
    const s = useAutoReceiveStore()
    const unlock = s.init(0)
    await s.followAccount(1)
    settings.resolve({ autoReceive: true })
    await unlock
    expect(s.account).toBe(1)
    expect(StartAutoReceive).toHaveBeenCalledTimes(1)
  })

  it('ignores a settings response superseded by a toggle', async () => {
    const settings = deferred<{ autoReceive: boolean }>()
    GetSettings.mockReturnValueOnce(settings.promise)
    const s = useAutoReceiveStore()
    s.enabled = true
    const unlock = s.init(0)
    await s.toggle(0)
    settings.resolve({ autoReceive: true })
    await unlock
    expect(s.enabled).toBe(false)
    expect(s.account).toBe(-1)
    expect(StartAutoReceive).not.toHaveBeenCalled()
  })

  it('persists rapid toggles in order without starting the superseded worker', async () => {
    const saved = deferred()
    SetAutoReceive.mockReturnValueOnce(saved.promise)
    const s = useAutoReceiveStore()
    const on = s.toggle(0)
    await flushPromises()
    const off = s.toggle(0)
    saved.resolve()
    await Promise.all([on, off])
    expect(SetAutoReceive.mock.calls).toEqual([[true], [false]])
    expect(StartAutoReceive).not.toHaveBeenCalled()
    expect(s.enabled).toBe(false)
  })

  it('continues with the latest account after a previous start fails', async () => {
    const started = deferred()
    StartAutoReceive.mockReturnValueOnce(started.promise)
    const s = useAutoReceiveStore()
    s.enabled = true
    const first = s.start(0).catch(() => {})
    await flushPromises()
    const latest = s.followAccount(1)
    started.reject(new Error('old connection ended'))
    await Promise.all([first, latest])
    expect(s.account).toBe(1)
    expect(StartAutoReceive).toHaveBeenCalledTimes(2)
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

  it('records backend auto-receive:error events ({hash,error} + bumped counter)', () => {
    const s = useAutoReceiveStore()
    s.wireEvents()
    const entry = onCalls.find(([n]) => n === 'auto-receive:error')
    expect(entry).toBeTruthy()
    // Backend emits an object { hash, error } (map[string]string).
    entry![1]({ hash: 'z1abc', error: 'plasma exhausted' })
    expect(s.lastError).toBe('plasma exhausted')
    expect(s.errorCount).toBe(1)
    // Missing/garbled payload falls back to a generic message.
    entry![1]({ hash: 'z1abc' })
    expect(s.lastError).toBe('Auto-receive failed')
    expect(s.errorCount).toBe(2)
    // A bare string is tolerated too.
    entry![1]('boom')
    expect(s.lastError).toBe('boom')
    expect(s.errorCount).toBe(3)
  })
})
