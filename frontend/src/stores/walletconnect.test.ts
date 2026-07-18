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
    lookup: vi.fn(),
    confirm: vi.fn(),
    cancel: vi.fn(),
    reconcile: vi.fn(),
    ack: vi.fn(),
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
  LookupWalletConnectPublication: h.lookup,
  ConfirmWalletConnectPublish: h.confirm,
  CancelPending: h.cancel,
  ReconcileWalletConnectPublication: h.reconcile,
  AckWalletConnectResult: h.ack,
}))
vi.mock('../../wailsjs/go/app/NodeService', () => ({
  NodeStatus: h.nodeStatus,
  GetNodeConfig: h.getConfig,
}))
vi.mock('../../wailsjs/go/app/WalletService', () => ({}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import {
  __resetWalletConnectModuleState,
  isSupportedZenonProposal,
  publicWalletConnectNodeURL,
  useWalletConnectStore,
} from './walletconnect'
import { useWalletStore } from './wallet'
import { useNodeStore } from './node'
import { useTxStore } from './tx'

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

function sendEvent(id: number, topic = 'topic', expiryTimestamp?: number) {
  return {
    topic,
    id,
    params: {
      chainId: 'zenon:1',
      request: { method: 'znn_send', params: { fromAddress: 'z1qold', accountBlock: {} }, expiryTimestamp },
    },
  }
}

async function prepareRequest(id = 7, topic = 'topic') {
  const wc = useWalletConnectStore()
  h.prepare.mockResolvedValueOnce({ outcome: 'prepare', preview: { ...preview } })
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
    expect(publicWalletConnectNodeURL('wss://mainnet.example/')).toBe('wss://mainnet.example/')
    expect(publicWalletConnectNodeURL('ws://127.0.0.1:35998')).toBe('ws://127.0.0.1:35998')
    expect(publicWalletConnectNodeURL('wss://user:secret@mainnet.example')).toBeUndefined()
    expect(publicWalletConnectNodeURL('wss://mainnet.example?apikey=secret')).toBeUndefined()
    expect(publicWalletConnectNodeURL('wss://mainnet.example#secret')).toBeUndefined()
    expect(publicWalletConnectNodeURL('not a URL')).toBeUndefined()
  })

  it('does not expose node URLs with path-embedded provider tokens or non-ws schemes', () => {
    expect(publicWalletConnectNodeURL('wss://node.example/v1/secret-project-token')).toBeUndefined()
    expect(publicWalletConnectNodeURL('wss://node.example/%76%31/token')).toBeUndefined()
    expect(publicWalletConnectNodeURL('https://node.example')).toBeUndefined()
    expect(publicWalletConnectNodeURL('http://node.example')).toBeUndefined()
    expect(publicWalletConnectNodeURL('file:///etc/hosts')).toBeUndefined()
  })
})

describe('WalletConnect request handling', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    __resetWalletConnectModuleState()
    h.sessions.splice(0)
    h.respond.mockReset().mockResolvedValue(undefined)
    h.nodeStatus.mockReset()
    h.prepare.mockReset()
    h.confirm.mockReset()
    h.cancel.mockReset().mockResolvedValue(undefined)
    h.lookup.mockReset().mockResolvedValue({ outcome: 'none' })
    h.reconcile.mockReset()
    h.ack.mockReset().mockResolvedValue(undefined)
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

  it('answers a locked-wallet fresh znn_send with 9000 after consulting the backend journal', async () => {
    // The backend must be consulted even while locked: a journaled outcome
    // must replay. Only a FRESH request maps the backend's locked error to 9000.
    const wc = useWalletConnectStore()
    h.prepare.mockRejectedValueOnce(new Error('wallet is locked'))
    await wc.handleRequest(sendEvent(9))

    expect(h.prepare).toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: { code: 9000, message: 'Wallet is locked' } }),
    }))
  })

  it('rejects a locked-wallet znn_info without touching the node', async () => {
    const wc = useWalletConnectStore()
    await wc.handleRequest({ topic: 'topic', id: 10, params: { chainId: 'zenon:1', request: { method: 'znn_info' } } })

    expect(h.nodeStatus).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: { code: 9000, message: 'Wallet is locked' } }),
    }))
  })

  it('replays a journaled published result even while the wallet is locked', async () => {
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'locked-replay-hash' }, publishedHash: 'locked-replay-hash' })

    await wc.handleRequest(sendEvent(61, 'locked-replay'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'locked-replay',
      response: expect.objectContaining({ result: { hash: 'locked-replay-hash' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('locked-replay', 61)
  })

  it('blocks a scam-flagged znn_info before disclosing anything', async () => {
    unlock()
    const wc = useWalletConnectStore()
    await wc.handleRequest({
      topic: 'topic',
      id: 62,
      params: { chainId: 'zenon:1', request: { method: 'znn_info' } },
      verifyContext: { verified: { origin: 'https://scam.example', validation: 'INVALID', isScam: true } },
    })

    expect(h.nodeStatus).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: expect.objectContaining({ code: 5000 }) }),
    }))
  })

  it('keeps a retryable delivery-error state when replay delivery fails', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'published',
      published: { hash: 'replay-retry-hash' },
      preview: { ...preview, holdId: 0 },
      publishedHash: 'replay-retry-hash',
    })
    h.respond.mockRejectedValueOnce(new Error('relay unavailable'))

    await wc.handleRequest(sendEvent(63, 'replay-retry'))

    expect(h.ack).not.toHaveBeenCalled()
    expect(wc.request?.status).toBe('delivery-error')
    expect(wc.request?.publishedHash).toBe('replay-retry-hash')
    expect(wc.request?.sessionEnded).toBe(false)

    h.respond.mockResolvedValueOnce(undefined)
    await wc.retryPublishedResponse()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'replay-retry',
      response: expect.objectContaining({ result: { hash: 'replay-retry-hash' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('replay-retry', 63)
    expect(wc.request).toBeNull()
  })

  it('marks the delivery-error session-ended when the session dies during replay delivery', async () => {
    // Round-3 finding 3: if the session ends while respond() is pending, the
    // retained delivery-error must carry sessionEnded so no retry targets a
    // dead session.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'published',
      published: { hash: 'ended-replay-hash' },
      preview: { ...preview, holdId: 0 },
      publishedHash: 'ended-replay-hash',
    })
    h.respond.mockImplementationOnce(async () => {
      // Session ends mid-respond.
      await wc.handleSessionEnded({ topic: 'ended-replay' })
      throw new Error('relay closed')
    })

    await wc.handleRequest(sendEvent(64, 'ended-replay'))

    expect(wc.request?.status).toBe('delivery-error')
    expect(wc.request?.sessionEnded).toBe(true)
    expect(wc.request?.error).toContain('session ended')

    // Retry is a no-op against a dead session.
    h.respond.mockClear()
    await wc.retryPublishedResponse()
    expect(h.respond).not.toHaveBeenCalled()
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
    finishPrepare({ outcome: 'prepare', preview: { ...preview } })
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

    finishPrepare({ outcome: 'prepare', preview: { ...preview } })
    await first
    expect(wc.request?.id).toBe(12)
    expect(wc.request?.topic).toBe('first-topic')
  })

  it('registers the request/proposal expiry listeners on the SignClient', async () => {
    const wc = useWalletConnectStore()
    await wc.ensureClient()
    expect(h.handlers.session_request_expire).toBeTypeOf('function')
    expect(h.handlers.proposal_expire).toBeTypeOf('function')
  })

  it('releases an awaiting hold without responding when its request expires', async () => {
    unlock()
    const wc = await prepareRequest(21, 'expiring-topic')

    await wc.handleRequestExpired(21)

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()

    await wc.approveRequest()
    expect(h.confirm).not.toHaveBeenCalled()
  })

  it('ignores an expiry for a different request id', async () => {
    unlock()
    const wc = await prepareRequest(22)

    await wc.handleRequestExpired(99)

    expect(h.cancel).not.toHaveBeenCalled()
    expect(wc.request?.id).toBe(22)
  })

  it('cancels the exact hold when a request expires while preparation is in flight', async () => {
    unlock()
    let finishPrepare!: (result: unknown) => void
    h.prepare.mockReturnValueOnce(new Promise((resolve) => { finishPrepare = resolve }))
    const wc = useWalletConnectStore()

    const preparing = wc.handleRequest(sendEvent(23, 'expire-during-prepare'))
    await vi.waitFor(() => expect(wc.preparingRequest?.id).toBe(23))
    await wc.handleRequestExpired(23)
    finishPrepare({ outcome: 'prepare', preview: { ...preview } })
    await preparing

    expect(h.cancel).toHaveBeenCalledWith(42)
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
  })

  it('fails approval closed past the request expiry deadline even without an event', async () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date('2026-07-17T12:00:00Z'))
      unlock()
      const expirySeconds = Math.floor(Date.now() / 1000) + 300
      const wc = useWalletConnectStore()
      h.prepare.mockResolvedValueOnce({ outcome: 'prepare', preview: { ...preview } })
      await wc.handleRequest(sendEvent(24, 'deadline-topic', expirySeconds))
      expect(wc.request?.id).toBe(24)

      vi.setSystemTime(new Date('2026-07-17T12:06:00Z'))
      await wc.approveRequest()

      expect(h.confirm).not.toHaveBeenCalled()
      expect(h.cancel).toHaveBeenCalledWith(42)
      expect(wc.request).toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not let an expiry race an in-flight publication with an error response', async () => {
    unlock()
    const wc = await prepareRequest(25, 'expire-during-publish')
    let finishPublish!: (result: unknown) => void
    h.confirm.mockReturnValue(new Promise((resolve) => { finishPublish = resolve }))

    const publishing = wc.approveRequest()
    await Promise.resolve()
    await wc.handleRequestExpired(25)

    expect(h.respond).not.toHaveBeenCalled()
    expect(h.cancel).not.toHaveBeenCalled()

    finishPublish({ hash: 'published-despite-expiry' })
    await publishing

    expect(wc.request?.status).toBe('delivery-error')
    expect(wc.request?.publishedHash).toBe('published-despite-expiry')
    expect(wc.request?.error).toContain('Do not submit it again')
    expect(h.respond.mock.calls.some(([call]) => 'error' in (call?.response ?? {}))).toBe(false)
  })

  it('clears only the matching expired proposal', async () => {
    unlock()
    const wc = useWalletConnectStore()
    wc.handleProposal({
      id: 31,
      params: { requiredNamespaces: bridgeNamespaces, proposer: { metadata: { name: 'Bridge A' } } },
    })
    expect(wc.proposal?.id).toBe(31)

    await wc.handleProposalExpired(99)
    expect(wc.proposal?.id).toBe(31)

    await wc.handleProposalExpired(31)
    expect(wc.proposal).toBeNull()
  })

  it('fails proposal approval closed past its expiry deadline', async () => {
    vi.useFakeTimers()
    try {
      vi.setSystemTime(new Date('2026-07-17T12:00:00Z'))
      unlock()
      const wc = useWalletConnectStore()
      wc.handleProposal({
        id: 32,
        params: {
          requiredNamespaces: bridgeNamespaces,
          expiryTimestamp: Math.floor(Date.now() / 1000) + 300,
          proposer: { metadata: { name: 'Bridge B' } },
        },
      })
      expect(wc.proposal?.id).toBe(32)

      vi.setSystemTime(new Date('2026-07-17T12:06:00Z'))
      await wc.approveProposal()

      expect(fakeClient.approve).not.toHaveBeenCalled()
      expect(wc.proposal).toBeNull()
      expect(wc.error).toContain('expired')
    } finally {
      vi.useRealTimers()
    }
  })

  it('captures the SignClient Verify identity on proposals and defaults to UNKNOWN', async () => {
    unlock()
    const wc = useWalletConnectStore()
    wc.handleProposal({
      id: 41,
      params: { requiredNamespaces: bridgeNamespaces, proposer: { metadata: { name: 'Bridge', url: 'https://evil.example' } } },
      verifyContext: { verified: { origin: 'https://bridge.0x3639.com', validation: 'VALID', isScam: false } },
    })
    expect(wc.proposal?.verifiedOrigin).toBe('https://bridge.0x3639.com')
    expect(wc.proposal?.validation).toBe('VALID')
    expect(wc.proposal?.isScam).toBe(false)

    wc.handleProposal({
      id: 42,
      params: { requiredNamespaces: bridgeNamespaces, proposer: { metadata: { name: 'Bridge' } } },
    })
    expect(wc.proposal?.validation).toBe('UNKNOWN')
    expect(wc.proposal?.isScam).toBe(false)
    expect(wc.proposal?.verifiedOrigin).toBe('')
  })

  it('refuses to approve a scam-flagged proposal', async () => {
    unlock()
    const wc = useWalletConnectStore()
    wc.handleProposal({
      id: 43,
      params: { requiredNamespaces: bridgeNamespaces, proposer: { metadata: { name: 'Definitely The Real Bridge' } } },
      verifyContext: { verified: { origin: 'https://scam.example', validation: 'INVALID', isScam: true } },
    })
    fakeClient.approve.mockClear()

    await wc.approveProposal()

    expect(fakeClient.approve).not.toHaveBeenCalled()
    expect(wc.proposal).not.toBeNull()
    expect(wc.error).toContain('scam')
  })

  it('captures the Verify identity on znn_send requests for the approval dialog', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.prepare.mockResolvedValueOnce({ ...preview })
    const event = sendEvent(44)
    ;(event as any).verifyContext = { verified: { origin: 'https://bridge.0x3639.com', validation: 'VALID', isScam: false } }
    await wc.handleRequest(event)

    expect(wc.request?.verifiedOrigin).toBe('https://bridge.0x3639.com')
    expect(wc.request?.validation).toBe('VALID')
    expect(wc.request?.isScam).toBe(false)
  })

  it('refuses to prepare a scam-flagged request and answers with a rejection', async () => {
    unlock()
    const wc = useWalletConnectStore()
    const event = sendEvent(45)
    ;(event as any).verifyContext = { verified: { origin: 'https://scam.example', validation: 'INVALID', isScam: true } }
    await wc.handleRequest(event)

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ error: expect.objectContaining({ code: 5000 }) }),
    }))
    expect(wc.request).toBeNull()
  })

  it('passes the WalletConnect request identity to the backend prepare', async () => {
    unlock()
    await prepareRequest(51, 'identity-topic')

    expect(h.prepare).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'identity-topic',
      requestId: 51,
      fromAddress: 'z1qold',
    }))
  })

  it('replays a journaled published result without a modal and acks after delivery', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'journaled-hash' }, publishedHash: 'journaled-hash' })

    await wc.handleRequest(sendEvent(52, 'replay-topic'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.confirm).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'replay-topic',
      response: expect.objectContaining({ result: { hash: 'journaled-hash' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('replay-topic', 52)
  })

  it('enters the reconcile flow for an unknown-outcome replay instead of re-preparing', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({ outcome: 'unknown', preview: { ...preview, holdId: 0 }, publishedHash: 'maybe-hash' })

    await wc.handleRequest(sendEvent(53, 'unknown-topic'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(wc.request?.status).toBe('unknown')
    expect(wc.request?.publishedHash).toBe('maybe-hash')
    expect(h.respond).not.toHaveBeenCalled()
    expect(h.ack).not.toHaveBeenCalled()
  })

  it('replays a journaled outcome before the scam gate, existing request, and busy tx', async () => {
    // Round-3 finding 1: a redelivered journaled request must resolve before
    // the frontend scam/existing/busy gates can turn it into a rejection.
    unlock()
    const wc = useWalletConnectStore()
    wc.request = { // an unrelated awaiting request occupies the single slot
      topic: 'other', id: 999, dapp: 'Other', preview: { ...preview },
      status: 'awaiting', error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, verifiedOrigin: '', validation: 'UNKNOWN', isScam: false,
    } as any
    const tx = useTxStore()
    tx.status = 'publishing'
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'gated-replay' }, publishedHash: 'gated-replay' })

    const event = sendEvent(71, 'scam-replay')
    ;(event as any).verifyContext = { verified: { origin: 'https://scam.example', validation: 'INVALID', isScam: true } }
    await wc.handleRequest(event)

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'scam-replay',
      response: expect.objectContaining({ result: { hash: 'gated-replay' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('scam-replay', 71)
  })

  it('rejects a reused-id conflict outcome with code 5000', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({ outcome: 'conflict' })

    await wc.handleRequest(sendEvent(72, 'reuse-topic'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'reuse-topic',
      response: expect.objectContaining({ error: expect.objectContaining({ code: 5000 }) }),
    }))
    expect(wc.request).toBeNull()
  })

  it('never answers the dapp when the journal lookup fails', async () => {
    // Round-5 finding P1b: a lookup throw leaves the true outcome UNKNOWN (the
    // block may be published). Any JSON-RPC response — even an error code —
    // could make the dapp retry under a NEW id and bypass the journal
    // identity, risking a duplicate. Leave it unanswered and retryable.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockRejectedValueOnce(new Error('cannot read the publication journal: disk error'))

    await wc.handleRequest(sendEvent(73, 'readfail-topic'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.error).not.toBe('')
  })

  it('does not create a fresh hold when the request expires during journal lookup', async () => {
    // Round-5 finding P1a: an expiry that fires while the lookup is awaiting
    // must be observed, or a fresh (approvable) hold could be created and
    // published after the request already expired.
    unlock()
    const wc = useWalletConnectStore()
    let finishLookup!: (r: unknown) => void
    h.lookup.mockReturnValueOnce(new Promise((resolve) => { finishLookup = resolve }))

    const pending = wc.handleRequest(sendEvent(74, 'expire-during-lookup'))
    await vi.waitFor(() => expect(h.lookup).toHaveBeenCalled())
    await wc.handleRequestExpired(74)
    finishLookup({ outcome: 'none' })
    await pending

    expect(h.prepare).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
    expect(wc.preparingRequest).toBeNull()
  })

  it('does not create a fresh hold when the session ends during journal lookup', async () => {
    unlock()
    const wc = useWalletConnectStore()
    let finishLookup!: (r: unknown) => void
    h.lookup.mockReturnValueOnce(new Promise((resolve) => { finishLookup = resolve }))

    const pending = wc.handleRequest(sendEvent(75, 'session-end-during-lookup'))
    await vi.waitFor(() => expect(h.lookup).toHaveBeenCalled())
    await wc.handleSessionEnded({ topic: 'session-end-during-lookup' })
    finishLookup({ outcome: 'none' })
    await pending

    expect(h.prepare).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
  })

  it('refuses a cross-topic duplicate with a neutral identity and recovers locally', async () => {
    // Round-10: an identical intent from ANOTHER session must not be auto-
    // replayed, must not build a hold, must NOT inherit the new (rejected)
    // dapp's Verify identity, and its recovery must be local-only (the original
    // session is gone) — reconcile, show the result, then explicitly clear.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'duplicate',
      preview: { ...preview, holdId: 0 },
      publishedHash: 'other-block',
      journalTopic: 'old-sess',
      journalRequestId: 100,
    })
    const event = sendEvent(500, 'new-sess')
    ;(event as any).verifyContext = { verified: { origin: 'https://new-dapp.example', validation: 'VALID', isScam: false } }

    await wc.handleRequest(event)

    // The new dapp request is refused (no hold, no disclosure of the old result).
    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'new-sess',
      response: expect.objectContaining({ id: 500, error: expect.objectContaining({ code: 5000 }) }),
    }))
    // The retained record is surfaced with a NEUTRAL identity — never the new
    // (VALID) dapp's origin next to an old transaction.
    expect(wc.request?.status).toBe('unknown')
    expect(wc.request?.topic).toBe('old-sess')
    expect(wc.request?.journalRequestId).toBe(100)
    expect(wc.request?.validation).toBe('UNKNOWN')
    expect(wc.request?.verifiedOrigin).toBe('')

    // Local-only recovery: reconcile the original key; NO response is sent to
    // the (dead) original session; the result is shown locally.
    h.respond.mockClear()
    h.reconcile.mockResolvedValueOnce({ hash: 'other-block' })
    await wc.reconcileRequest()
    expect(h.reconcile).toHaveBeenCalledWith('old-sess', 100)
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request?.status).toBe('recovered')
    expect(h.ack).not.toHaveBeenCalled()

    // Explicit clear acknowledges (deletes) the original record.
    await wc.acknowledgeRecovered()
    expect(h.ack).toHaveBeenCalledWith('old-sess', 100)
    expect(wc.request).toBeNull()
  })

  it('keeps the recovered record when acknowledgement fails, then clears on retry', async () => {
    // Round-11 P2: Ack must succeed before the modal clears; on failure the
    // recovered state stays with an actionable error.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'duplicate', preview: { ...preview, holdId: 0 }, publishedHash: 'blk',
      journalTopic: 'old-sess', journalRequestId: 100,
    })
    await wc.handleRequest(sendEvent(500, 'new-sess'))
    h.reconcile.mockResolvedValueOnce({ hash: 'blk' })
    await wc.reconcileRequest()
    expect(wc.request?.status).toBe('recovered')

    h.ack.mockRejectedValueOnce(new Error('disk full'))
    await wc.acknowledgeRecovered()
    expect(wc.request?.status).toBe('recovered')
    expect(wc.request?.error).toContain('disk full')

    h.ack.mockResolvedValueOnce(undefined)
    await wc.acknowledgeRecovered()
    expect(h.ack).toHaveBeenCalledWith('old-sess', 100)
    expect(wc.request).toBeNull()
  })

  it('preserves a local-recovery record across session end, expiry, and wallet lock', async () => {
    // Round-11 P2: local recovery is journal-owned and must survive lifecycle
    // events without being discarded or answered on a dead session.
    unlock()
    const wc = useWalletConnectStore()
    const surface = async () => {
      h.lookup.mockResolvedValueOnce({
        outcome: 'duplicate', preview: { ...preview, holdId: 0 }, publishedHash: 'blk',
        journalTopic: 'old-sess', journalRequestId: 100,
      })
      await wc.handleRequest(sendEvent(500, 'new-sess'))
      h.respond.mockClear()
      h.cancel.mockClear()
    }

    // Exercise the RECOVERED state (the one not previously guarded).
    await surface()
    h.reconcile.mockResolvedValueOnce({ hash: 'blk' })
    await wc.reconcileRequest()
    expect(wc.request?.status).toBe('recovered')
    h.respond.mockClear(); h.cancel.mockClear()

    await wc.handleSessionEnded({ topic: 'old-sess' })
    expect(wc.request?.status).toBe('recovered')
    expect(h.respond).not.toHaveBeenCalled()

    await wc.handleRequestExpired(100)
    expect(wc.request?.status).toBe('recovered')

    await wc.walletLocked()
    expect(wc.request?.status).toBe('recovered')
    expect(h.respond).not.toHaveBeenCalled()
    expect(h.cancel).not.toHaveBeenCalled()
  })

  it('handles a duplicate returned by the prepare-time recheck', async () => {
    // Round-10: a cross-topic record can appear during the lookup->prepare
    // window; Prepare then returns duplicate. It must refuse + recover, never
    // install an approvable awaiting request.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({ outcome: 'none' })
    h.prepare.mockResolvedValueOnce({
      outcome: 'duplicate',
      preview: { ...preview, holdId: 0 },
      publishedHash: 'race-block',
      journalTopic: 'old-sess',
      journalRequestId: 100,
    })

    await wc.handleRequest(sendEvent(501, 'new-sess'))

    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'new-sess',
      response: expect.objectContaining({ id: 501, error: expect.objectContaining({ code: 5000 }) }),
    }))
    expect(wc.request?.status).toBe('unknown')
    expect(wc.request?.topic).toBe('old-sess')
    expect(wc.request?.validation).toBe('UNKNOWN')
  })

  it('delivers a cross-id published replay to the new id but acks the original journal key', async () => {
    // P1: a dapp reissuing an identical intent under a new id resolves to the
    // record journaled under the ORIGINAL id — respond to the new id, but ack
    // the original key so the right record is cleared and no second block is
    // ever built.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'published',
      published: { hash: 'orig-block' },
      publishedHash: 'orig-block',
      journalTopic: 'sess',
      journalRequestId: 100,
    })

    await wc.handleRequest(sendEvent(101, 'sess'))

    expect(h.prepare).not.toHaveBeenCalled()
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'sess',
      response: expect.objectContaining({ id: 101, result: { hash: 'orig-block' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('sess', 100)
  })

  it('reconciles the original journal key for a cross-id unknown replay', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockResolvedValueOnce({
      outcome: 'unknown',
      preview: { ...preview, holdId: 0 },
      publishedHash: 'maybe-orig',
      journalTopic: 'sess',
      journalRequestId: 100,
    })

    await wc.handleRequest(sendEvent(101, 'sess'))
    expect(wc.request?.status).toBe('unknown')

    h.reconcile.mockResolvedValueOnce({ hash: 'resolved-orig' })
    await wc.reconcileRequest()

    // Reconcile and ack target the ORIGINAL key; the response goes to the new id.
    expect(h.reconcile).toHaveBeenCalledWith('sess', 100)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ id: 101, result: { hash: 'resolved-orig' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('sess', 100)
  })

  it('cancels a retained failed lookup when the session ends, creating no hold', async () => {
    // Round-7 finding P1a: a scheduled retry must not run after session_delete
    // and fall through to a fresh hold for a dead session.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockRejectedValueOnce(new Error('cannot read the publication journal: disk error'))
    await wc.handleRequest(sendEvent(92, 'dead-session'))

    await wc.handleSessionEnded({ topic: 'dead-session' })

    // The retry timer fires after the session ended → must be a no-op.
    h.lookup.mockClear()
    h.prepare.mockClear()
    h.lookup.mockResolvedValueOnce({ outcome: 'none' })
    await wc.retryFailedLookup('dead-session', 92)

    expect(h.lookup).not.toHaveBeenCalled()
    expect(h.prepare).not.toHaveBeenCalled()
    expect(wc.request).toBeNull()
  })

  it('keeps retrying a failed lookup past the attempt count until the request expires', async () => {
    // Round-7 finding P1b: with an expiry known, same-id protection must hold
    // until expiry, not be abandoned after a fixed attempt count.
    unlock()
    const wc = useWalletConnectStore()
    const farExpiry = Math.floor(Date.now() / 1000) + 3600
    h.lookup.mockRejectedValueOnce(new Error('disk error'))
    await wc.handleRequest(sendEvent(93, 'long-lived', farExpiry))

    // Simulate many failed retries; the entry must survive (not be dropped)
    // because the request has not expired.
    for (let i = 0; i < 12; i++) {
      h.lookup.mockRejectedValueOnce(new Error('disk error'))
      await wc.retryFailedLookup('long-lived', 93)
    }

    // Still protected: a subsequent successful retry delivers under the same id.
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'late-hash' }, publishedHash: 'late-hash' })
    await wc.retryFailedLookup('long-lived', 93)
    await vi.waitFor(() => expect(h.ack).toHaveBeenCalledWith('long-lived', 93))
  })

  it('drains a queued replay after a session-ended publish fails', async () => {
    // Round-7 finding P2: publish fails after the session ended → the request
    // clears, and a replay queued behind it must surface.
    unlock()
    const wc = useWalletConnectStore()
    // A fresh request prepared and awaiting.
    h.lookup.mockResolvedValueOnce({ outcome: 'none' })
    h.prepare.mockResolvedValueOnce({ outcome: 'prepare', preview: { ...preview, holdId: 30 } })
    await wc.handleRequest(sendEvent(94, 'pub-topic'))
    expect(wc.request?.status).toBe('awaiting')

    // A replay for another dapp queues behind it (slot busy).
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'queued-after-pub' }, publishedHash: 'queued-after-pub' })
    h.respond.mockRejectedValueOnce(new Error('busy'))
    await wc.handleRequest(sendEvent(95, 'queued-after-pub-topic'))
    expect(wc.request?.id).toBe(94)

    // Approve; the session ends during publish and the publish then fails.
    let failPublish!: (e: Error) => void
    h.confirm.mockReturnValue(new Promise((_, reject) => { failPublish = reject }))
    const approving = wc.approveRequest()
    await Promise.resolve()
    await wc.handleSessionEnded({ topic: 'pub-topic' })
    h.respond.mockResolvedValue(undefined)
    failPublish(new Error('not connected'))
    await approving

    await vi.waitFor(() => expect(h.ack).toHaveBeenCalledWith('queued-after-pub-topic', 95))
  })

  it('retries a failed journal lookup and delivers the resolved result', async () => {
    // Round-6 finding P1: an unanswered lookup failure must be actively
    // retried (SignClient suppresses same-id redelivery until restart), so the
    // original block's outcome resolves before the dapp times out.
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockRejectedValueOnce(new Error('cannot read the publication journal: disk error'))

    await wc.handleRequest(sendEvent(90, 'retry-topic'))
    expect(h.respond).not.toHaveBeenCalled()

    // The journal becomes readable; the retained request retries the SAME id.
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'retried-hash' }, publishedHash: 'retried-hash' })
    await wc.retryFailedLookup('retry-topic', 90)

    await vi.waitFor(() => expect(h.ack).toHaveBeenCalledWith('retry-topic', 90))
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'retry-topic',
      response: expect.objectContaining({ result: { hash: 'retried-hash' } }),
    }))
  })

  it('stops retrying a failed lookup once the request expiry has passed', async () => {
    unlock()
    const wc = useWalletConnectStore()
    h.lookup.mockRejectedValueOnce(new Error('disk error'))
    // expiryTimestamp already in the past (epoch+1s).
    await wc.handleRequest(sendEvent(91, 'expired-retry', 1))

    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'x' } })
    await wc.retryFailedLookup('expired-retry', 91)

    // Past expiry: give up — never re-run the lookup or publish/deliver.
    expect(h.lookup).toHaveBeenCalledTimes(1)
    expect(h.respond).not.toHaveBeenCalled()
  })

  it('continues draining after a queued published result is delivered', async () => {
    // Round-6 finding P2c: multiple queued published results must all deliver.
    unlock()
    const wc = useWalletConnectStore()
    wc.request = {
      topic: 'displayed', id: 700, dapp: 'D', preview: { ...preview, holdId: 9 },
      status: 'awaiting', error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, verifiedOrigin: '', validation: 'UNKNOWN', isScam: false,
    } as any
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'q1' }, publishedHash: 'q1' })
    h.respond.mockRejectedValueOnce(new Error('busy'))
    await wc.handleRequest(sendEvent(71, 'q1-topic'))
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'q2' }, publishedHash: 'q2' })
    h.respond.mockRejectedValueOnce(new Error('busy'))
    await wc.handleRequest(sendEvent(72, 'q2-topic'))

    h.respond.mockResolvedValue(undefined)
    await wc.rejectRequest()

    await vi.waitFor(() => {
      expect(h.ack).toHaveBeenCalledWith('q1-topic', 71)
      expect(h.ack).toHaveBeenCalledWith('q2-topic', 72)
    })
  })

  it('drains a queued replay when an in-flight preparation clears without a modal', async () => {
    // Round-6 finding P2a: a replay queued behind a preparation that then fails
    // (no modal) must surface once the preparing slot is released.
    unlock()
    const wc = useWalletConnectStore()
    let failPrepare!: (e: Error) => void
    h.lookup.mockResolvedValueOnce({ outcome: 'none' })
    h.prepare.mockReturnValueOnce(new Promise((_, reject) => { failPrepare = reject }))
    const p = wc.handleRequest(sendEvent(80, 'prep-topic'))
    await vi.waitFor(() => expect(wc.preparingRequest?.id).toBe(80))

    // A published replay arrives while preparing → its delivery attempt fails
    // (slot busy), so it is queued rather than shown.
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'qp' }, publishedHash: 'qp' })
    h.respond.mockRejectedValueOnce(new Error('busy'))
    await wc.handleRequest(sendEvent(81, 'qp-topic'))
    expect(wc.request).toBeNull()

    // The preparation fails without a modal; the finally must drain the queue.
    h.respond.mockResolvedValue(undefined)
    failPrepare(new Error('not connected'))
    await p

    await vi.waitFor(() => expect(h.ack).toHaveBeenCalledWith('qp-topic', 81))
  })

  it('drains a queued replay after approval-time expiry drops the request', async () => {
    // Round-6 finding P2b.
    unlock()
    const wc = useWalletConnectStore()
    wc.request = {
      topic: 'displayed', id: 700, dapp: 'D', preview: { ...preview, holdId: 3 },
      status: 'awaiting', error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, expiryTimestamp: 1, verifiedOrigin: '', validation: 'UNKNOWN', isScam: false,
    } as any
    h.lookup.mockResolvedValueOnce({ outcome: 'unknown', preview: { ...preview, holdId: 0 }, publishedHash: 'maybe' })
    await wc.handleRequest(sendEvent(71, 'unknown-q'))
    expect(wc.request?.id).toBe(700)

    await wc.approveRequest() // expiryTimestamp:1 is long past → drop + drain

    await vi.waitFor(() => expect(wc.request?.id).toBe(71))
    expect(wc.request?.status).toBe('unknown')
  })

  it('surfaces a queued replay once the displayed request clears', async () => {
    // Round-5 finding P2: a published replay that fails to deliver while busy
    // is queued and delivered when the modal clears, not left to redelivery.
    unlock()
    const wc = useWalletConnectStore()
    wc.request = {
      topic: 'displayed', id: 600, dapp: 'Displayed', preview: { ...preview, holdId: 5 },
      status: 'awaiting', error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, verifiedOrigin: '', validation: 'UNKNOWN', isScam: false,
    } as any
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'queued-replay' }, preview: { ...preview, holdId: 0 }, publishedHash: 'queued-replay' })
    h.respond.mockRejectedValueOnce(new Error('busy relay')) // delivery attempt while busy fails

    await wc.handleRequest(sendEvent(76, 'queued-topic'))
    expect(wc.request?.id).toBe(600) // displayed request intact

    // Clear the displayed request; the queued replay must now be delivered
    // (drain is fire-and-forget, so wait for its async chain to settle).
    h.respond.mockResolvedValue(undefined)
    await wc.rejectRequest()

    await vi.waitFor(() => expect(h.ack).toHaveBeenCalledWith('queued-topic', 76))
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      topic: 'queued-topic',
      response: expect.objectContaining({ result: { hash: 'queued-replay' } }),
    }))
  })

  it('does not hijack an earlier in-flight preparation while looking up a second request', async () => {
    // Round-4 finding P1a: the lookup of request B must not replace request A's
    // preparing marker, or A's session/expiry events would be lost.
    unlock()
    const wc = useWalletConnectStore()
    // A: a fresh request that is mid-prepare (preparing marker held).
    let finishPrepareA!: (r: unknown) => void
    h.lookup.mockResolvedValueOnce({ outcome: 'none' })
    h.prepare.mockReturnValueOnce(new Promise((resolve) => { finishPrepareA = resolve }))
    const a = wc.handleRequest(sendEvent(81, 'topic-a'))
    await vi.waitFor(() => expect(wc.preparingRequest?.id).toBe(81))

    // B: a second request; its lookup must NOT overwrite A's preparing marker.
    let finishLookupB!: (r: unknown) => void
    h.lookup.mockReturnValueOnce(new Promise((resolve) => { finishLookupB = resolve }))
    const b = wc.handleRequest(sendEvent(82, 'topic-b'))
    await Promise.resolve()
    await Promise.resolve()
    expect(wc.preparingRequest?.id).toBe(81)

    // A's request expires while B is mid-lookup — the event must reach A.
    await wc.handleRequestExpired(81)
    expect(wc.preparingRequest?.sessionEnded).toBe(true)

    finishLookupB({ outcome: 'none' })
    finishPrepareA({ outcome: 'prepare', preview: { ...preview, holdId: 0 } })
    await Promise.all([a, b])
  })

  it('does not overwrite a displayed request when an unknown replay arrives', async () => {
    // Round-4 finding P2: an unknown replay must not clobber another displayed
    // request (which would orphan its backend hold).
    unlock()
    const wc = useWalletConnectStore()
    const displayed = {
      topic: 'displayed', id: 500, dapp: 'Displayed', preview: { ...preview, holdId: 77 },
      status: 'awaiting' as const, error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, verifiedOrigin: '', validation: 'UNKNOWN' as const, isScam: false,
    }
    wc.request = { ...displayed }
    h.lookup.mockResolvedValueOnce({ outcome: 'unknown', preview: { ...preview, holdId: 0 }, publishedHash: 'maybe' })

    await wc.handleRequest(sendEvent(83, 'unknown-busy'))

    // The displayed request is intact; its hold was never cancelled.
    expect(wc.request?.id).toBe(500)
    expect(wc.request?.status).toBe('awaiting')
    expect(h.cancel).not.toHaveBeenCalled()
    expect(wc.error).not.toBe('')
  })

  it('does not overwrite a displayed request when replay delivery fails', async () => {
    unlock()
    const wc = useWalletConnectStore()
    wc.request = {
      topic: 'displayed', id: 501, dapp: 'Displayed', preview: { ...preview, holdId: 88 },
      status: 'awaiting', error: '', publishedResult: null, publishedHash: '',
      sessionEnded: false, verifiedOrigin: '', validation: 'UNKNOWN', isScam: false,
    } as any
    h.lookup.mockResolvedValueOnce({ outcome: 'published', published: { hash: 'busy-replay' }, preview: { ...preview, holdId: 0 }, publishedHash: 'busy-replay' })
    h.respond.mockRejectedValueOnce(new Error('relay down'))

    await wc.handleRequest(sendEvent(84, 'published-busy'))

    expect(wc.request?.id).toBe(501)
    expect(wc.request?.status).toBe('awaiting')
    expect(h.cancel).not.toHaveBeenCalled()
    expect(h.ack).not.toHaveBeenCalled()
    expect(wc.error).toContain('busy-replay')
  })

  it('routes an unknown-outcome confirm error into reconcile, never a rejection', async () => {
    unlock()
    const wc = await prepareRequest(54, 'confirm-unknown')
    h.confirm.mockRejectedValueOnce(new Error('walletconnect publication outcome unknown: connection reset. The signed block abc is preserved'))

    await wc.approveRequest()

    expect(wc.request?.status).toBe('unknown')
    expect(h.respond).not.toHaveBeenCalled()

    // Neither rejection path may answer an unknown outcome with an error.
    await wc.rejectRequest()
    await wc.clearRequestError()
    expect(h.respond).not.toHaveBeenCalled()
    expect(wc.request?.status).toBe('unknown')
  })

  it('reconcile success delivers the stored result and acks the journal', async () => {
    unlock()
    const wc = await prepareRequest(55, 'reconcile-topic')
    h.confirm.mockRejectedValueOnce(new Error('walletconnect publication outcome unknown: timeout'))
    await wc.approveRequest()
    expect(wc.request?.status).toBe('unknown')

    h.reconcile.mockResolvedValueOnce({ hash: 'reconciled-hash' })
    await wc.reconcileRequest()

    expect(h.reconcile).toHaveBeenCalledWith('reconcile-topic', 55)
    expect(h.respond).toHaveBeenCalledWith(expect.objectContaining({
      response: expect.objectContaining({ result: { hash: 'reconciled-hash' } }),
    }))
    expect(h.ack).toHaveBeenCalledWith('reconcile-topic', 55)
    expect(wc.request).toBeNull()
  })

  it('keeps the unknown state retryable when reconcile fails', async () => {
    unlock()
    const wc = await prepareRequest(56, 'retry-topic')
    h.confirm.mockRejectedValueOnce(new Error('walletconnect publication outcome unknown: timeout'))
    await wc.approveRequest()

    h.reconcile.mockRejectedValueOnce(new Error('walletconnect publication outcome unknown: still unreachable'))
    await wc.reconcileRequest()

    expect(wc.request?.status).toBe('unknown')
    expect(wc.request?.error).toContain('unreachable')
    expect(h.respond).not.toHaveBeenCalled()
  })

  it('acks the journal after a normal publish result is delivered', async () => {
    unlock()
    const wc = await prepareRequest(57, 'ack-topic')
    h.confirm.mockResolvedValueOnce({ hash: 'delivered-hash' })

    await wc.approveRequest()

    expect(wc.request).toBeNull()
    expect(h.ack).toHaveBeenCalledWith('ack-topic', 57)
  })

  it('does not disturb an unknown-outcome request on wallet lock', async () => {
    unlock()
    const wc = await prepareRequest(58, 'lock-unknown')
    h.confirm.mockRejectedValueOnce(new Error('walletconnect publication outcome unknown: timeout'))
    await wc.approveRequest()

    await wc.walletLocked()

    expect(wc.request?.status).toBe('unknown')
    expect(h.respond).not.toHaveBeenCalled()
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
