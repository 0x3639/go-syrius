// stores/wallet.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
// vi.hoisted so Lock exists when the hoisted vi.mock factory runs.
const Lock = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const GenerateMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue('w1 w2 w3'))
const ImportMnemonic = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'New.dat' }))
const ImportKeystore = vi.hoisted(() => vi.fn().mockResolvedValue({ name: 'Old.dat' }))
const PickKeystoreFile = vi.hoisted(() => vi.fn().mockResolvedValue('/tmp/k.dat'))
const RenameWallet = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ id: 'Main.dat', name: 'Main', baseAddress: 'z1qmain' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  RenameWallet,
  Lock,
  GenerateMnemonic,
  ImportMnemonic,
  ImportKeystore,
  PickKeystoreFile,
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qxxx', label: '' }]),
  SelectAccount: vi.fn().mockResolvedValue(undefined),
  SetAccountLabel: vi.fn().mockResolvedValue(undefined),
}))
// Captures the wallet:locked handler so tests can fire backend-initiated locks.
const eventHandlers = vi.hoisted(() => ({} as Record<string, (...a: any[]) => void>))
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, cb: (...a: any[]) => void) => { eventHandlers[name] = cb },
}))
import { useWalletStore } from './wallet'
beforeEach(() => setActivePinia(createPinia()))
describe('wallet store', () => {
  it('lists wallets and unlocks', async () => {
    const s = useWalletStore()
    await s.loadWallets()
    expect(s.wallets).toEqual([{ id: 'Main.dat', name: 'Main', baseAddress: 'z1qmain' }])
    expect(s.active).toBe('Main.dat')
    await s.unlock('Main.dat', 'pw'); expect(s.locked).toBe(false)
  })
  it('lock() re-locks the backend keystore, not just the UI', async () => {
    const s = useWalletStore()
    await s.unlock('Main', 'pw')
    s.lock()
    expect(Lock).toHaveBeenCalled()
    expect(s.locked).toBe(true)
    expect(s.active).toBe('')
  })
  it('lifecycle actions call the bindings', async () => {
    const s = useWalletStore()
    expect(await s.generateMnemonic()).toBe('w1 w2 w3')
    await s.importMnemonic('New.dat', 'pw', 'w1 w2 w3')
    expect(ImportMnemonic).toHaveBeenCalledWith('New.dat', 'pw', 'w1 w2 w3')
    await s.importKeystore('/tmp/k.dat')
    expect(ImportKeystore).toHaveBeenCalledWith('/tmp/k.dat', '')
    expect(await s.pickKeystoreFile()).toBe('/tmp/k.dat')
  })
  it('rename calls RenameWallet then reloads', async () => {
    const s = useWalletStore()
    await s.rename('Main.dat', 'Renamed')
    expect(RenameWallet).toHaveBeenCalledWith('Main.dat', 'Renamed')
    expect(s.wallets).toEqual([{ id: 'Main.dat', name: 'Main', baseAddress: 'z1qmain' }])
  })
  it('loads accounts on unlock and selects by index', async () => {
    const s = useWalletStore()
    await s.unlock('Main', 'pw')
    expect(s.accounts).toEqual([{ index: 0, address: 'z1qxxx', label: '' }])
    expect(s.activeAddress()).toBe('z1qxxx')
    await s.select(0)
    expect(s.activeIndex).toBe(0)
  })
  // The two lock-event tests reset modules (to clear the module-level
  // `lockEventInit`) and rebuild a fresh Pinia so the runtime mock's captured
  // wallet:locked handler binds THIS test's store — otherwise the second test
  // would fire the first test's stale closure.
  it('backend-initiated wallet:locked tears down the local session; already-locked is a no-op', async () => {
    vi.resetModules()
    const { setActivePinia, createPinia } = await import('pinia')
    setActivePinia(createPinia())
    const { useWalletStore } = await import('./wallet')
    const s = useWalletStore()
    await s.unlock('Main.dat', 'pw')
    s.initLockEvent()

    eventHandlers['wallet:locked']()
    expect(s.locked).toBe(true)
    expect(s.active).toBe('')
    expect(s.accounts).toEqual([])

    // Idempotent: the event also fires on manual lock — firing again on an
    // already-locked store leaves state unchanged and does not throw.
    expect(() => eventHandlers['wallet:locked']()).not.toThrow()
    expect(s.locked).toBe(true)
    expect(s.active).toBe('')
    expect(s.accounts).toEqual([])
  })

  it('manual lock() does not double-teardown via its own wallet:locked event', async () => {
    vi.resetModules()
    const { setActivePinia, createPinia } = await import('pinia')
    setActivePinia(createPinia())
    const { useWalletStore } = await import('./wallet')
    const s = useWalletStore()
    await s.unlock('Main.dat', 'pw')
    s.initLockEvent()
    s.lock()
    expect(s.locked).toBe(true)
    // Go Lock() emits wallet:locked after manual lock too; already-locked → no-op.
    expect(() => eventHandlers['wallet:locked']()).not.toThrow()
    expect(s.locked).toBe(true)
    expect(s.active).toBe('')
    expect(s.accounts).toEqual([])
  })
})

describe('select — overlapping selections resolve to the latest intent', () => {
  it('queues a selection made while one is in flight and applies the newest last', async () => {
    const { useWalletStore } = await import('./wallet')
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.SelectAccount as any).mockClear()
    const store = useWalletStore()
    store.locked = false

    // First selection hangs at the backend…
    let resolveFirst!: (v: unknown) => void
    ;(W.SelectAccount as any).mockReturnValueOnce(new Promise((r) => { resolveFirst = r }))
    const first = store.select(1)

    // …two more clicks land while it is in flight; only the LAST may apply.
    ;(W.SelectAccount as any).mockResolvedValue({ index: 3, address: 'z1acc3', label: '' })
    const second = store.select(2)
    const third = store.select(3)

    resolveFirst({ index: 1, address: 'z1acc1', label: '' })
    await Promise.all([first, second, third])

    expect(store.activeIndex).toBe(3)
    // The superseded intermediate selection (2) was never sent to the backend.
    const calls = (W.SelectAccount as any).mock.calls.map((c: unknown[]) => c[0])
    expect(calls).toEqual([1, 3])
  })

  it('renders the authoritative index returned by the backend', async () => {
    const { useWalletStore } = await import('./wallet')
    const W = await import('../../wailsjs/go/app/WalletService')
    const store = useWalletStore()
    store.locked = false
    ;(W.SelectAccount as any).mockResolvedValueOnce({ index: 5, address: 'z1acc5', label: '' })
    await store.select(5)
    expect(store.activeIndex).toBe(5)
  })
})

describe('select — wallet-session token rejects stale responses', () => {
  it('discards a selection that resolves after the wallet changed', async () => {
    const { useWalletStore } = await import('./wallet')
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.SelectAccount as any).mockClear()
    const store = useWalletStore()
    store.locked = false

    // A selection for the OLD wallet hangs at the backend…
    let resolveSelect!: (v: unknown) => void
    ;(W.SelectAccount as any).mockReturnValueOnce(new Promise((r) => { resolveSelect = r }))
    const pending = store.select(4)

    // …the user unlocks a different wallet meanwhile (activeIndex resets to 0)…
    await store.unlock('Other.dat', 'pw')
    expect(store.activeIndex).toBe(0)

    // …then the old wallet's selection finally resolves. It must NOT be
    // committed: the backend signer is account 0 of the NEW wallet.
    resolveSelect({ index: 4, address: 'z1oldwallet4', label: '' })
    await pending
    expect(store.activeIndex).toBe(0)
  })

  it('drops queued selections from before the wallet change', async () => {
    const { useWalletStore } = await import('./wallet')
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.SelectAccount as any).mockClear()
    const store = useWalletStore()
    store.locked = false

    let resolveSelect!: (v: unknown) => void
    ;(W.SelectAccount as any).mockReturnValueOnce(new Promise((r) => { resolveSelect = r }))
    const first = store.select(1)
    const second = store.select(2) // queued behind the in-flight call

    await store.unlock('Other.dat', 'pw')
    resolveSelect({ index: 1, address: 'z1old1', label: '' })
    await Promise.all([first, second])

    // Neither the stale response nor the queued pre-unlock intent applied.
    expect(store.activeIndex).toBe(0)
    const calls = (W.SelectAccount as any).mock.calls.map((c: unknown[]) => c[0])
    expect(calls).toEqual([1])
  })
})
