import { defineStore } from 'pinia'
import { SignClient } from '@walletconnect/sign-client'
import * as Node from '../../wailsjs/go/app/NodeService'
import * as Tx from '../../wailsjs/go/app/TxService'
import { useWalletStore } from './wallet'
import { useNodeStore } from './node'
import { useTxStore, type SendPreview } from './tx'

const ZENON_CHAIN = 'zenon:1'
const SUPPORTED_METHODS = ['znn_info', 'znn_sign', 'znn_send'] as const
const SUPPORTED_EVENTS = ['chainIdChange', 'addressChange'] as const
type Client = Awaited<ReturnType<typeof SignClient.init>>

export type WalletConnectSession = {
  topic: string
  name: string
  url: string
  icon: string
  accounts: string[]
}

export type WalletConnectProposal = {
  id: number
  name: string
  description: string
  url: string
  icon: string
  methods: string[]
  events: string[]
  raw: any
}

export type WalletConnectRequest = {
  topic: string
  id: number
  dapp: string
  preview: SendPreview
  status: 'awaiting' | 'publishing' | 'notifying' | 'error' | 'delivery-error'
  error: string
  publishedResult: unknown | null
  publishedHash: string
  sessionEnded: boolean
}

type WalletConnectPreparingRequest = {
  token: number
  topic: string
  id: number
  sessionEnded: boolean
  cancelMessage: string
  cancelCode: number
}

let client: Client | null = null
let initPromise: Promise<Client> | null = null
let listenersReady = false
let preparingRequestToken = 0

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error)
}

function errorResponse(id: number, code: number, message: string) {
  return { id, jsonrpc: '2.0' as const, error: { code, message } }
}

function resultResponse(id: number, result: unknown) {
  return { id, jsonrpc: '2.0' as const, result }
}

export function zenonProposalNamespace(
  required: Record<string, any>,
  optional: Record<string, any> = {},
): Record<string, any> | null {
  // SignClient 2.23.x normalizes an unregistered custom namespace such as
  // `zenon` into optionalNamespaces on the receiving wallet, even when the
  // dapp supplied it in requiredNamespaces. Required foreign namespaces still
  // fail closed because this wallet cannot satisfy them; unrelated OPTIONAL
  // namespaces can simply be omitted from the approved session.
  const otherNamespaces = Object.keys(required).filter((key) => key !== 'zenon')
  if (otherNamespaces.length > 0) return null
  return required.zenon ?? optional.zenon ?? null
}

export function isSupportedZenonProposal(
  required: Record<string, any>,
  optional: Record<string, any> = {},
): boolean {
  const zenon = zenonProposalNamespace(required, optional)
  const chains = zenon?.chains ?? []
  const methods = zenon?.methods ?? []
  const events = zenon?.events ?? []
  return Boolean(zenon)
    && chains.length > 0
    && chains.every((chain: string) => chain === ZENON_CHAIN)
    && methods.every((method: string) => SUPPORTED_METHODS.includes(method as any))
    && events.every((event: string) => SUPPORTED_EVENTS.includes(event as any))
}

export function publicWalletConnectNodeURL(value: string): string | undefined {
  try {
    const parsed = new URL(value)
    // znn_info crosses a dapp trust boundary. A configured endpoint may carry
    // basic-auth, query-token, or fragment material, which must never be
    // disclosed to a session.
    if (parsed.username || parsed.password || parsed.search || parsed.hash) return undefined
    return value
  } catch {
    return undefined
  }
}

export const useWalletConnectStore = defineStore('walletconnect', {
  state: () => ({
    initialized: false,
    initializing: false,
    pairing: false,
    error: '',
    proposal: null as WalletConnectProposal | null,
    sessions: [] as WalletConnectSession[],
    request: null as WalletConnectRequest | null,
    preparingRequest: null as WalletConnectPreparingRequest | null,
  }),
  actions: {
    projectId(): string {
      return (import.meta.env.VITE_WALLETCONNECT_PROJECT_ID as string | undefined)?.trim() ?? ''
    },
    refreshSessions() {
      if (!client) { this.sessions = []; return }
      this.sessions = client.session.getAll().map((session: any) => ({
        topic: session.topic,
        name: session.peer?.metadata?.name ?? 'Connected dapp',
        url: session.peer?.metadata?.url ?? '',
        icon: session.peer?.metadata?.icons?.[0] ?? '',
        accounts: session.namespaces?.zenon?.accounts ?? [],
      }))
    },
    async ensureClient(): Promise<Client> {
      if (client) return client
      const projectId = this.projectId()
      if (!projectId || projectId === 'REPLACE_ME_WC_PROJECT_ID') {
        throw new Error('WalletConnect is not configured. Set VITE_WALLETCONNECT_PROJECT_ID for this build.')
      }
      if (!initPromise) {
        this.initializing = true
        initPromise = SignClient.init({
          projectId,
          metadata: {
            name: 'go-syrius',
            description: 'Zenon Network of Momentum desktop wallet',
            url: 'https://github.com/0x3639/go-syrius',
            icons: [],
          },
        })
      }
      try {
        client = await initPromise
        this.installListeners(client)
        this.initialized = true
        this.refreshSessions()
        return client
      } catch (error) {
        // A transient relay/init failure must not poison WalletConnect until
        // the whole desktop app restarts. Later calls get a fresh client init.
        client = null
        initPromise = null
        this.initialized = false
        throw error
      } finally {
        this.initializing = false
      }
    },
    installListeners(c: Client) {
      if (listenersReady) return
      listenersReady = true
      c.on('session_proposal', (raw: any) => {
        const required = raw.params?.requiredNamespaces ?? {}
        const optional = raw.params?.optionalNamespaces ?? {}
        const zenon = zenonProposalNamespace(required, optional)
        const methods = zenon?.methods ?? []
        const events = zenon?.events ?? []
        if (!isSupportedZenonProposal(required, optional)) {
          this.error = 'Connection rejected: the dapp requested an unsupported WalletConnect namespace'
          void c.reject({ id: raw.id, reason: { code: 5100, message: 'Unsupported WalletConnect namespace' } })
          return
        }
        const metadata = raw.params?.proposer?.metadata ?? {}
        this.proposal = {
          id: raw.id,
          name: metadata.name ?? 'Unknown dapp',
          description: metadata.description ?? '',
          url: metadata.url ?? '',
          icon: metadata.icons?.[0] ?? '',
          methods: [...methods],
          events: [...events],
          raw,
        }
      })
      c.on('session_request', (event: any) => { void this.handleRequest(event) })
      c.on('session_delete', (event: any) => { void this.handleSessionEnded(event) })
      c.on('session_expire', (event: any) => { void this.handleSessionEnded(event) })
    },
    async pair(uri: string) {
      this.error = ''
      const value = uri.trim()
      if (!value.startsWith('wc:')) throw new Error('Paste a valid wc: pairing URI')
      this.pairing = true
      try {
        const c = await this.ensureClient()
        await c.core.pairing.pair({ uri: value })
      } catch (error) {
        this.error = messageOf(error)
        throw error
      } finally {
        this.pairing = false
      }
    },
    async approveProposal() {
      if (!this.proposal) return
      const c = await this.ensureClient()
      const wallet = useWalletStore()
      const address = wallet.activeAddress()
      if (wallet.locked || !address) throw new Error('Unlock a wallet before approving a session')
      const proposal = this.proposal
      this.error = ''
      try {
        const { acknowledged } = await c.approve({
          id: proposal.id,
          namespaces: {
            zenon: {
              accounts: [`${ZENON_CHAIN}:${address}`],
              methods: proposal.methods,
              events: proposal.events,
            },
          },
        })
        await acknowledged()
        this.proposal = null
        this.refreshSessions()
      } catch (error) {
        this.error = messageOf(error)
        throw error
      }
    },
    async rejectProposal() {
      if (!this.proposal) return
      const c = await this.ensureClient()
      const id = this.proposal.id
      this.proposal = null
      await c.reject({ id, reason: { code: 5000, message: 'User rejected' } })
    },
    async disconnect(topic: string) {
      const c = await this.ensureClient()
      await c.disconnect({ topic, reason: { code: 6000, message: 'User disconnected' } })
      await this.handleSessionEnded({ topic })
    },
    async respondError(topic: string, id: number, code: number, message: string) {
      const c = await this.ensureClient()
      await c.respond({ topic, response: errorResponse(id, code, message) })
    },
    async failRequest(topic: string, id: number, code: number, message: string) {
      // Keep the wallet-side reason visible. Some dapps flatten several
      // WalletConnect error codes into a generic "rejected" message, which
      // otherwise makes node/chain configuration failures indistinguishable
      // from an explicit user rejection.
      this.error = `WalletConnect request failed: ${message}`
      await this.respondError(topic, id, code, message)
    },
    async handleRequest(event: any) {
      const c = await this.ensureClient()
      const { topic, id, params } = event
      const method = params?.request?.method
      if (params?.chainId !== ZENON_CHAIN) {
        await this.failRequest(topic, id, 5100, 'Unsupported Zenon chain')
        return
      }
      const wallet = useWalletStore()
      const node = useNodeStore()
      if (wallet.locked || !wallet.activeAddress()) {
        await this.failRequest(topic, id, 9000, 'Wallet is locked')
        return
      }
      if (method === 'znn_info') {
        // Read the authoritative backend snapshot for every handshake. The
        // Pinia copy is event-driven and can briefly reflect a prior node while
        // Settings is reconnecting, which made a valid mainnet session fail
        // with WalletConnect code 5100.
        let status: { connected?: boolean; chainId?: number }
        try {
          status = await Node.NodeStatus()
          node.applyStatus(status)
        } catch {
          await this.failRequest(topic, id, -32000, 'Unable to read the connected Zenon node status')
          return
        }
        if (!status.connected) {
          await this.failRequest(topic, id, -32000, 'Wallet is not connected to a Zenon node')
          return
        }
        if (status.chainId !== 1) {
          await this.failRequest(topic, id, 5100, `Wallet node is on Zenon chain ${status.chainId ?? 0}; expected chain 1`)
          return
        }
        const cfg = await node.getConfig().catch(() => null)
        const configuredNodeUrl = cfg
          ? (cfg.mode === 'remote' ? cfg.remoteUrl : cfg.mode === 'local' ? cfg.localUrl : 'ws://127.0.0.1:35998')
          : undefined
        const nodeUrl = configuredNodeUrl ? publicWalletConnectNodeURL(configuredNodeUrl) : undefined
        await c.respond({
          topic,
          response: resultResponse(id, { address: wallet.activeAddress(), chainId: status.chainId, nodeUrl }),
        })
        this.error = ''
        return
      }
      if (method === 'znn_sign') {
        await this.failRequest(topic, id, 4200, 'Arbitrary signing is not supported')
        return
      }
      if (method !== 'znn_send') {
        await this.failRequest(topic, id, 4200, `Unsupported method ${method}`)
        return
      }
      if (this.request || this.preparingRequest) {
        await this.failRequest(topic, id, -32000, 'Another WalletConnect request is awaiting approval')
        return
      }
      const tx = useTxStore()
      if (tx.status === 'preparing' || tx.status === 'awaiting' || tx.status === 'publishing') {
        await this.failRequest(topic, id, -32000, 'The wallet is already handling another transaction')
        return
      }
      const preparing: WalletConnectPreparingRequest = {
        token: ++preparingRequestToken,
        topic,
        id,
        sessionEnded: false,
        cancelMessage: '',
        cancelCode: 5000,
      }
      this.preparingRequest = preparing
      try {
        const preview = await Tx.PrepareWalletConnectSend(params.request.params as any) as unknown as SendPreview
        // The session/account may have ended while the backend prepared the
        // hold. Release that exact hold and never resurrect a stale modal.
        if (this.preparingRequest?.token !== preparing.token || preparing.sessionEnded || preparing.cancelMessage) {
          const holdID = preview?.holdId ?? 0
          if (holdID) await Tx.CancelPending(holdID).catch(() => {})
          if (!preparing.sessionEnded && preparing.cancelMessage) {
            await this.respondError(topic, id, preparing.cancelCode, preparing.cancelMessage).catch(() => {})
          }
          return
        }
        const session = this.sessions.find((item) => item.topic === topic)
        this.request = {
          topic,
          id,
          dapp: session?.name ?? 'Connected dapp',
          preview,
          status: 'awaiting',
          error: '',
          publishedResult: null,
          publishedHash: '',
          sessionEnded: false,
        }
        this.error = ''
      } catch (error) {
        if (preparing.sessionEnded) return
        if (preparing.cancelMessage) {
          await this.respondError(topic, id, preparing.cancelCode, preparing.cancelMessage).catch(() => {})
          return
        }
        await this.failRequest(topic, id, -32602, messageOf(error))
      } finally {
        if (this.preparingRequest?.token === preparing.token) this.preparingRequest = null
      }
    },
    async approveRequest() {
      if (!this.request || this.request.status !== 'awaiting') return
      const current = this.request
      current.status = 'publishing'
      current.error = ''
      let result: unknown
      try {
        result = await Tx.ConfirmWalletConnectPublish(current.preview.holdId ?? 0)
      } catch (error) {
        if (current.sessionEnded) {
          const holdID = current.preview.holdId ?? 0
          if (holdID) await Tx.CancelPending(holdID).catch(() => {})
          if (this.request === current) this.request = null
          return
        }
        current.status = 'error'
        current.error = messageOf(error)
        return
      }
      // From this line onward funds moved. Never convert a relay/session
      // delivery failure into a rejection response for this request id.
      current.publishedResult = result
      current.publishedHash = String((result as any)?.hash ?? '')
      current.status = 'notifying'
      await this.deliverPublishedResult(current)
    },
    async deliverPublishedResult(current: WalletConnectRequest) {
      if (!current.publishedResult || this.request !== current) return
      const label = current.publishedHash ? ` ${current.publishedHash}` : ''
      if (current.sessionEnded) {
        current.status = 'delivery-error'
        current.error = `Transaction${label} was published, but the dapp session ended before it could be notified. Do not submit it again.`
        return
      }
      try {
        const c = await this.ensureClient()
        await c.respond({ topic: current.topic, response: resultResponse(current.id, current.publishedResult) })
        if (this.request === current) this.request = null
      } catch (error) {
        current.status = 'delivery-error'
        current.error = current.sessionEnded
          ? `Transaction${label} was published, but the dapp session ended before it could be notified. Do not submit it again.`
          : `Transaction${label} was published, but the dapp could not be notified: ${messageOf(error)}. Do not submit it again.`
      }
    },
    async retryPublishedResponse() {
      if (!this.request || this.request.status !== 'delivery-error' || !this.request.publishedResult || this.request.sessionEnded) return
      const current = this.request
      current.status = 'notifying'
      current.error = ''
      await this.deliverPublishedResult(current)
    },
    clearPublishedRequest() {
      if (this.request?.status === 'delivery-error') this.request = null
    },
    async rejectRequest(message = 'User rejected') {
      if (!this.request || (this.request.status !== 'awaiting' && this.request.status !== 'error')) return
      const current = this.request
      this.request = null
      const holdID = current.preview.holdId ?? 0
      if (holdID) await Tx.CancelPending(holdID).catch(() => {})
      await this.respondError(current.topic, current.id, 5000, message)
    },
    async clearRequestError() {
      if (!this.request || this.request.status !== 'error') return
      await this.rejectRequest(this.request.error || 'Wallet could not publish the request')
    },
    async updateAccount(address: string) {
      if (!client || !address) return
      // A preview is bound to the account displayed in it. Cancel it before
      // advertising the new account. An in-flight publish is different: its
      // single eventual result must remain authoritative, so never pre-empt it
      // with an "account changed" rejection.
      if (this.preparingRequest) {
        this.preparingRequest.cancelMessage = 'Wallet account changed'
        this.preparingRequest.cancelCode = 5000
      }
      if (this.request && (this.request.status === 'awaiting' || this.request.status === 'error')) {
        const current = this.request
        this.request = null
        const holdID = current.preview.holdId ?? 0
        if (holdID) await Tx.CancelPending(holdID).catch(() => {})
        await this.respondError(current.topic, current.id, 5000, 'Wallet account changed').catch(() => {})
      }
      for (const session of client.session.getAll() as any[]) {
        const zenon = session.namespaces?.zenon
        if (!zenon) continue
        const namespaces = {
          ...session.namespaces,
          zenon: { ...zenon, accounts: [`${ZENON_CHAIN}:${address}`] },
        }
        try {
          const { acknowledged } = await client.update({ topic: session.topic, namespaces })
          await acknowledged()
          await client.emit({
            topic: session.topic,
            chainId: ZENON_CHAIN,
            event: { name: 'addressChange', data: address },
          })
        } catch { /* stale session; session_delete/expire will clean it up */ }
      }
      this.refreshSessions()
    },
    async handleSessionEnded(event: any) {
      const topic = typeof event === 'string' ? event : event?.topic
      this.refreshSessions()
      if (!topic) return
      const preparing = this.preparingRequest
      if (preparing && preparing.topic === topic) preparing.sessionEnded = true
      if (!this.request || this.request.topic !== topic) return
      const current = this.request
      current.sessionEnded = true
      if (current.status === 'publishing' || current.status === 'notifying') {
        // Let the single in-flight publication/delivery attempt settle. Calling
        // respond() again here could race it and produce two result responses.
        return
      }
      if (current.status === 'delivery-error') {
        const label = current.publishedHash ? ` ${current.publishedHash}` : ''
        current.error = `Transaction${label} was published, but the dapp session ended before it could be notified. Do not submit it again.`
        return
      }
      this.request = null
      const holdID = current.preview.holdId ?? 0
      if (holdID) await Tx.CancelPending(holdID).catch(() => {})
    },
    async walletLocked() {
      if (this.preparingRequest) {
        this.preparingRequest.cancelMessage = 'Wallet is locked'
        this.preparingRequest.cancelCode = 9000
      }
      if (!this.request) return
      // Once publication starts, only its actual outcome may answer the dapp.
      // Sending 9000 here can race a successful publish and invite a duplicate.
      if (this.request.status === 'publishing' || this.request.status === 'notifying' || this.request.status === 'delivery-error') return
      const current = this.request
      this.request = null
      const holdID = current.preview.holdId ?? 0
      if (holdID) await Tx.CancelPending(holdID).catch(() => {})
      await this.respondError(current.topic, current.id, 9000, 'Wallet is locked')
    },
  },
})
