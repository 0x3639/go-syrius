import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const h = vi.hoisted(() => {
  const handlers: Record<string, (event: any) => void> = {}
  const sessions: any[] = []
  return {
    handlers,
    sessions,
    init: vi.fn(),
    on: vi.fn((name: string, callback: (event: any) => void) => { handlers[name] = callback }),
    nodeStatus: vi.fn(),
    respond: vi.fn(),
    prepare: vi.fn(),
    confirm: vi.fn(),
    cancel: vi.fn(),
    update: vi.fn(),
    emit: vi.fn(),
    disconnect: vi.fn(),
    pair: vi.fn(),
    getConfig: vi.fn(),
  }
})

const fakeClient = {
  on: h.on,
  respond: h.respond,
  approve: vi.fn(),
  reject: vi.fn(),
  update: h.update,
  emit: h.emit,
  disconnect: h.disconnect,
  session: { getAll: () => h.sessions },
  core: { pairing: { pair: h.pair } },
}

vi.mock('@walletconnect/sign-client', () => ({ SignClient: { init: h.init } }))
vi.mock('../../wailsjs/go/app/TxService', () => ({
  PrepareWalletConnectSend: h.prepare,
  ConfirmWalletConnectPublish: h.confirm,
  CancelPending: h.cancel,
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  NodeStatus: h.nodeStatus,
  GetNodeConfig: h.getConfig,
}))
vi.mock('../../wailsjs/go/app/WalletService', () => ({}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import {
  isSupportedZenonProposal,
  publicWalletConnectNodeURL,
  useWalletConnectStore,
} from './walletconnect'
import { useWalletStore } from './wallet'
import { useNodeStore } from './node'

const bridgeNamespaces = {
  zenon: {
    chains: ['zenon:1'],
    methods: ['znn_info', 'znn_sign', 'znn_send'],
    events: ['chainIdChange', 'addressChange'],
  },
}

const preview = {
  holdId: 42,
  fromAddress: 'z1qold',
  toAddress: 'z1qxemdeddedxbridgexxxxxxxxxxxxxxxs6f5v0',
  symbol: 'ZNN',
  zts: 'zts1znnxxxxxxxxxxxxx9z4ulx',
  amount: '100000000',
  decimals: 8,
  usedPlasma: 0,
  difficulty: 1,
  hash: 'preview-hash',
  needsPoW: true,
  summary: 'Bridge.WrapToken',
}

function unlock(address = 'z1qold') {
  const wallet = useWalletStore()
  wallet.locked = false
  wallet.accounts = [{ index: 0, address, label: '' }]
  wallet.activeIndex = 0
  return wallet
}

function sendEvent(id: number, topic = 'topic') {
  return {
    topic,
    id,
    params: {
      chainId: 'zenon:1',
      request: { method: 'znn_send', params: { fromAddress: 'z1qold', accountBlock: {} } },
    },
  }
}

async function prepareRequest(id = 7, topic = 'topic') {
  const wc = useWalletConnectStore()
  h.prepare.mockResolvedValueOnce({ ...preview })
  await wc.handleRequest(sendEvent(id, topic))
  return wc
}

describe('WalletConnect Zenon namespace compatibility', () => {
  it('accepts the frozen namespace used by both bridge-dapp and nom-bridge', () => {
    expect(isSupportedZenonProposal(bridgeNamespaces)).toBe(true)
  })

  it('accepts SignClient normalization of the custom namespace into optionalNamespaces', () => {
    expect(isSupportedZenonProposal({}, bridgeNamespaces)).toBe(true)
  })

  it('rejects other chains, namespaces, methods, and events', () => {
    expect(isSupportedZenonProposal({ zenon: { ...bridgeNamespaces.zenon, chains: ['zenon:73404'] } })).toBe(false)
    expect(isSupportedZenonProposal({ ...bridgeNamespaces, eip155: { chains: ['eip155:1'] } })).toBe(false)
    expect(isSupportedZenonProposal({ zenon: { ...bridgeNamespaces.zenon, methods: ['znn_send', 'znn_exportSeed'] } })).toBe(false)
    expect(isSupportedZenonProposal({ zenon: { ...bridgeNamespaces.zenon, events: ['accountsChanged'] } })).toBe(false)
  })

  it('ignores unrelated optional namespaces but rejects required ones', () => {
    const optional = { ...bridgeNamespaces, eip155: { chains: ['eip155:1'], methods: [], events: [] } }
    expect(isSupportedZenonProposal({}, optional)).toBe(true)
    expect(isSupportedZenonProposal({ eip155: optional.eip155 }, bridgeNamespaces)).toBe(false)
  })

  it('does not expose node URLs containing credentials to dapps', () => {
    expect(publicWalletConnectNodeURL('wss://mainnet.example')).toBe('wss://mainnet.example')
    expect(publicWalletConnectNodeURL('wss://user:secret@mainnet.example')).toBeUndefined()
    expect(publicWalletConnectNodeURL('wss://mainnet.example?apikey=secret')).toBeUndefined()
    expect(publicWalletConnectNodeURL('not a URL')).toBeUndefined()
  })
})

describe('WalletConnect request handling', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    h.sessions.splice(0)
    h.respond.mockReset().mockResolvedValue(undefined)
    h.nodeStatus.mockReset()
    h.prepare.mockReset()
    h.confirm.mockReset()
    h.cancel.mockReset().mockResolvedValue(undefined)
    h.update.mockReset().mockResolvedValue({ acknowledged: async () => {} })
    h.emit.mockReset().mockResolvedValue(undefined)
    h.disconnect.mockReset().mockResolvedValue(undefined)
    h.pair.mockReset().mockResolvedValue(undefined)
    h.getConfig.mockReset().mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://mainnet.example', localUrl: '' })
    h.init.mockReset().mockResolvedValue(fakeClient)
    vi.stubEnv('VITE_WALLETCONNECT_PROJECT_ID', 'test-project-id')
  })

  it('retries SignClient initialization after a transient failure', async () => {
    const wc = useWalletConnectStore()
    h.init.mockRejectedValueOnce(new Error('relay offline')).mockResolvedValueOnce(fakeClient)

    await expect(wc.ensureClient()).rejects.toThrow('relay offline')
    await expect(wc.ensureClient()).resolves.toBe(fakeClient)

    expect(h.init).toHaveBeenCalledTimes(2)
    expect(h.handlers.session_delete).toBeTypeOf('function')
    expect(h.handlers.session_expire).toBeTypeOf('function')
  })

  it('answers znn_info from authoritative backend status and omits credentialed URLs', async () => {
    h.nodeStatus.mockResolvedValue({ mode: 'remote', connected: true, chainId: 1, height: 10 })
    h.getConfig.mockResolvedValue({ mode: 'remote', remoteUrl: 'wss://user:secret@mainnet.example', localUrl: '' })
    const wallet = unlock('z1qvalid')
    const node = useNodeStore()
    node.chainId = 73404
    node.connected = true

    const wc = useWalletConnectStore()
    await wc.handleRequest({
      topic: 'topic',
      id: 7,
      params: { chainId: 'zenon:1', request: { method: 'znn_info' } },
    })

    expect(h.nodeStatus).toHaveBeenCalledOnce()
    expect(node.chainId).toBe(1)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'topic',
      response: expect.objectContaining({
        id: 7,
        result: { address: wallet.activeAddress(), chainId: 1, nodeUrl: undefined },
      }),
    }))
  })

  it('reports a disconnected node as an operational error with a visible reason', async () => {
    h.nodeStatus.mockResolvedValue({ mode: 'remote', connected: false, chainId: 0, height: 0 })
    unlock('z1qvalid')

    const wc = useWalletConnectStore()
    await wc.handleRequest({
      topic: 'topic',
      id: 8,
      params: { chainId: 'zenon:1', request: { method: 'znn_info' } },
    })

    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({
        error: { code: -32000, message: 'Wallet is not connected to a Zenon node' },
      }),
    }))
    expect(wc.error).toContain('Wallet is not connected to a Zenon node')
  })

  it('rejects requests while the wallet is locked before preparing anything', async () => {
    const wc = useWalletConnectStore()
    await wc.handleRequest(sendEvent(9))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: { code: 9000, message: 'Wallet is locked' } }),
    }))
  })

  it('cancels the exact backend hold before answering a user rejection', async () => {
    unlock()
    const wc = await prepareRequest()

    await wc.rejectRequest()

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.cancel.mock.invocationCallOrder[0]).toBeLessThan(h.respond.mock.invocationCallOrder[0])
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: { code: 5000, message: 'User rejected' } }),
    }))
    expect(wc.request).toBeNull()
  })

  it('never sends a rejection after publication when result delivery fails', async () => {
    unlock()
    const wc = await prepareRequest()
    h.confirm.mockResolvedValue({ hash: 'published-hash', height: 1 })
    h.respond.mockRejectedValueOnce(new Error('relay unavailable'))

    await wc.approveRequest()

    expect(wc.request?.status).toBe('delivery-error')
    expect(wc.request?.publishedHash).toBe('published-hash')
    expect(wc.request?.error).toContain('Do not submit it again')
    expect(h.respond).toHaveBeenCalledTimes(1)
    expect(h.respond.mock.calls.some(([call]) => 'error' in call.response)).toBe(false)

    h.respond.mockResolvedValueOnce(undefined)
    await wc.retryPublishedResponse()
    expect(wc.request).toBeNull()
    expect(h.confirm).toHaveBeenCalledTimes(1)
    expect(h.respond).toHaveBeenCalledTimes(2)
  })

  it('does not reject a publishing transaction when the wallet locks', async () => {
    unlock()
    const wc = await prepareRequest()
    let finishPublish!: (result: unknown) => void
    h.confirm.mockReturnValue(new Promise((resolve) => { finishPublish = resolve }))

    const publishing = wc.approveRequest()
    await Promise.resolve()
    expect(wc.request?.status).toBe('publishing')

    await wc.walletLocked()
    expect(wc.request?.status).toBe('publishing')
    expect(h.cancel).not.toHaveBeenCalled()
    expect(h.respond).not.toHaveBeenCalled()

    finishPublish({ hash: 'published-after-lock' })
    await publishing
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ result: { hash: 'published-after-lock' } }),
    }))
  })

  it('releases an awaiting hold without responding after its session ends', async () => {
    unlock()
    const wc = await prepareRequest(11, 'ended-topic')

    await wc.handleSessionEnded({ topic: 'ended-topic' })

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
  })

  it('does not answer an ended session when its in-flight publication settles', async () => {
    unlock()
    const wc = await prepareRequest(15, 'ended-during-publish')
    let finishPublish!: (result: unknown) => void
    h.confirm.mockReturnValue(new Promise((resolve) => { finishPublish = resolve }))

    const publishing = wc.approveRequest()
    await Promise.resolve()
    await wc.handleSessionEnded({ topic: 'ended-during-publish' })
    finishPublish({ hash: 'published-with-ended-session' })
    await publishing

    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request?.status).toBe('delivery-error')
    expect(wc.request?.sessionEnded).toBe(true)
    expect(wc.request?.error).toContain('session ended')
  })

  it('releases a retained hold when publication fails after the session ended', async () => {
    unlock()
    const wc = await prepareRequest(16, 'failed-ended-session')
    let failPublish!: (error: Error) => void
    h.confirm.mockReturnValue(new Promise((_, reject) => { failPublish = reject }))

    const publishing = wc.approveRequest()
    await Promise.resolve()
    await wc.handleSessionEnded({ topic: 'failed-ended-session' })
    failPublish(new Error('not connected'))
    await publishing

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
  })

  it('uses wallet-locked code 9000 when lock cancels an in-flight prepare', async () => {
    unlock()
    let finishPrepare!: (result: unknown) => void
    h.prepare.mockReturnValueOnce(new Promise((resolve) => { finishPrepare = resolve }))
    const wc = useWalletConnectStore()

    const preparing = wc.handleRequest(sendEvent(17, 'lock-during-prepare'))
    await vi.waitFor(() => expect(wc.preparingRequest?.id).toBe(17))
    await wc.walletLocked()
    finishPrepare({ ...preview })
    await preparing

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'lock-during-prepare',
      response: expect.objectContaining({ error: { code: 9000, message: 'Wallet is locked' } }),
    }))
    expect(wc.request).toBeNull()
  })

  it('serializes znn_send preparation so duplicate requests cannot strand a hold', async () => {
    unlock()
    let finishPrepare!: (result: unknown) => void
    h.prepare.mockReturnValueOnce(new Promise((resolve) => { finishPrepare = resolve }))
    const wc = useWalletConnectStore()

    const first = wc.handleRequest(sendEvent(12, 'first-topic'))
    await Promise.resolve()
    await Promise.resolve()
    await wc.handleRequest(sendEvent(13, 'second-topic'))

    expect(h.prepare).toHaveBeenCalledTimes(1)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'second-topic',
      response: expect.objectContaining({ error: expect.objectContaining({ code: -32000 }) }),
    }))

    finishPrepare({ ...preview })
    await first
    expect(wc.request?.id).toBe(12)
    expect(wc.request?.topic).toBe('first-topic')
  })

  it('cancels a stale awaiting preview before advertising an account change', async () => {
    unlock()
    h.sessions.push({
      topic: 'topic',
      namespaces: { zenon: { accounts: ['zenon:1:z1qold'], methods: ['znn_send'], events: ['addressChange'] } },
      peer: { metadata: { name: 'Bridge' } },
    })
    const wc = await prepareRequest()

    await wc.updateAccount('z1qnew')

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: { code: 5000, message: 'Wallet account changed' } }),
    }))
    expect(h.update).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'topic',
      namespaces: expect.objectContaining({
        zenon: expect.objectContaining({ accounts: ['zenon:1:z1qnew'] }),
      }),
    }))
    expect(h.emit).toHaveBeenCalledWith(expect.objectContaining({
      event: { name: 'addressChange', data: 'z1qnew' },
    }))
    expect(wc.request).toBeNull()
  })
})
