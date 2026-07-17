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

// The SignClient Verify attestation for a proposal or request. The metadata
// name/url are peer-controlled and spoofable; this is the only identity signal
// that is not.
export type WalletConnectVerify = {
  verifiedOrigin: string
  validation: 'VALID' | 'INVALID' | 'UNKNOWN'
  isScam: boolean
}

export type WalletConnectProposal = WalletConnectVerify & {
  id: number
  name: string
  description: string
  url: string
  icon: string
  methods: string[]
  events: string[]
  raw: any
  expiryTimestamp?: number
}

export type WalletConnectRequest = WalletConnectVerify & {
  topic: string
  id: number
  dapp: string
  preview: SendPreview
  status: 'awaiting' | 'publishing' | 'notifying' | 'error' | 'delivery-error' | 'unknown'
  error: string
  publishedResult: unknown | null
  publishedHash: string
  sessionEnded: boolean
  expiryTimestamp?: number
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

// Errors carrying this marker mean the signed block MAY be on chain (WC-01):
// they must enter the reconcile flow and never be answered with a rejection.
const OUTCOME_UNKNOWN_MARKER = 'walletconnect publication outcome unknown'

type WalletConnectPrepareOutcome = {
  outcome: 'prepare' | 'published' | 'unknown' | 'conflict' | 'none'
  preview?: SendPreview
  published?: unknown
  publishedHash?: string
}

function errorResponse(id: number, code: number, message: string) {
  return { id, jsonrpc: '2.0' as const, error: { code, message } }
}

function resultResponse(id: number, result: unknown) {
  return { id, jsonrpc: '2.0' as const, result }
}

// A missing/omitted verifyContext (older relay, Verify outage) degrades to
// UNKNOWN — shown as unverified, never as trusted.
function verifyOf(context: any): WalletConnectVerify {
  const verified = context?.verified ?? {}
  const validation = verified.validation
  return {
    verifiedOrigin: typeof verified.origin === 'string' ? verified.origin : '',
    validation: validation === 'VALID' || validation === 'INVALID' ? validation : 'UNKNOWN',
    isScam: verified.isScam === true,
  }
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
    // credential material anywhere except the root: basic-auth userinfo,
    // query/fragment tokens, or a hosted provider's path-embedded project key
    // (wss://host/v1/<token>). Only a bare ws(s) origin is ever disclosed.
    if (parsed.protocol !== 'ws:' && parsed.protocol !== 'wss:') return undefined
    if (parsed.username || parsed.password || parsed.search || parsed.hash) return undefined
    if (parsed.pathname !== '' && parsed.pathname !== '/') return undefined
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
      c.on('session_proposal', (raw: any) => { this.handleProposal(raw) })
      c.on('session_request', (event: any) => { void this.handleRequest(event) })
      c.on('session_request_expire', (event: any) => { void this.handleRequestExpired(event?.id) })
      c.on('proposal_expire', (event: any) => { void this.handleProposalExpired(event?.id) })
      c.on('session_delete', (event: any) => { void this.handleSessionEnded(event) })
      c.on('session_expire', (event: any) => { void this.handleSessionEnded(event) })
    },
    handleProposal(raw: any) {
      const required = raw.params?.requiredNamespaces ?? {}
      const optional = raw.params?.optionalNamespaces ?? {}
      const zenon = zenonProposalNamespace(required, optional)
      const methods = zenon?.methods ?? []
      const events = zenon?.events ?? []
      if (!isSupportedZenonProposal(required, optional)) {
        this.error = 'Connection rejected: the dapp requested an unsupported WalletConnect namespace'
        if (client) void client.reject({ id: raw.id, reason: { code: 5100, message: 'Unsupported WalletConnect namespace' } })
        return
      }
      const metadata = raw.params?.proposer?.metadata ?? {}
      const expiry = Number(raw.params?.expiryTimestamp)
      this.proposal = {
        id: raw.id,
        name: metadata.name ?? 'Unknown dapp',
        description: metadata.description ?? '',
        url: metadata.url ?? '',
        icon: metadata.icons?.[0] ?? '',
        methods: [...methods],
        events: [...events],
        raw,
        expiryTimestamp: Number.isFinite(expiry) && expiry > 0 ? expiry : undefined,
        ...verifyOf(raw.verifyContext),
      }
    },
    async handleProposalExpired(id: number) {
      // SignClient has already deleted the expired proposal; only clear the
      // matching one so a delayed expiry cannot wipe a newer proposal.
      if (this.proposal?.id === id) this.proposal = null
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
      // Defense in depth alongside the proposal_expire listener: approving an
      // already-expired proposal can only fail at the relay.
      if (this.proposal.expiryTimestamp && Date.now() >= this.proposal.expiryTimestamp * 1000) {
        this.proposal = null
        this.error = 'The connection proposal expired; ask the dapp for a fresh pairing'
        return
      }
      // Verify flagged this peer as a known scam; the proposal stays visible so
      // the user can reject it, but it can never be approved.
      if (this.proposal.isScam) {
        this.error = 'Connection blocked: WalletConnect Verify flagged this dapp as a known scam'
        return
      }
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
      // The Verify hard block runs before any DISCLOSURE or hold: a scam-
      // flagged peer must not learn the wallet address / node URL via znn_info
      // and must not create a znn_send hold. It is applied per-method, though —
      // NOT to a znn_send journal replay, which resolves a possibly-published
      // outcome (funds already moved) and must never become a rejection.
      const verify = verifyOf(event?.verifyContext)
      const wallet = useWalletStore()
      const node = useNodeStore()
      if (method === 'znn_info') {
        if (verify.isScam) {
          await this.failRequest(topic, id, 5000, 'Request blocked: WalletConnect Verify flagged this dapp as a known scam')
          return
        }
        if (wallet.locked || !wallet.activeAddress()) {
          await this.failRequest(topic, id, 9000, 'Wallet is locked')
          return
        }
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
      // Journal replay resolution comes FIRST — before the scam, existing-
      // request, and busy-tx gates below — so a redelivered published or
      // unknown outcome (funds may already have moved) is never turned into an
      // ordinary rejection. The lookup is journal-only: it creates no hold AND
      // does NOT touch the shared `preparingRequest` slot, so an earlier
      // in-flight preparation keeps receiving its own session/expiry events.
      let replay: WalletConnectPrepareOutcome
      try {
        replay = await Tx.LookupWalletConnectPublication(
          { ...(params.request.params as any), topic, requestId: id } as any,
        ) as unknown as WalletConnectPrepareOutcome
      } catch (error) {
        // A lookup THROW is a journal read/IPC failure: the true outcome is
        // UNKNOWN (the block may be published), so answer with a retryable
        // operational error, never a definite user-style rejection.
        await this.failRequest(topic, id, -32000, `Cannot resolve this request's status: ${messageOf(error)}`)
        return
      }
      if (replay.outcome === 'conflict') {
        // A reused request id carrying a different intent: a safe, definite
        // refusal of the NEW intent (which was never approved).
        await this.failRequest(topic, id, 5000, 'This WalletConnect request id was already used for a different transaction')
        return
      }
      if (replay.outcome === 'published' && replay.published) {
        await this.deliverReplayPublished(topic, id, replay, verify)
        return
      }
      if (replay.outcome === 'unknown') {
        // Never displace an already-displayed request or an in-flight
        // preparation — that would orphan its backend hold. The journal record
        // survives, so a later redelivery surfaces this once the wallet is free.
        if (this.request || this.preparingRequest) {
          this.error = 'A WalletConnect transaction with an unresolved status is pending; finish the current request to review it.'
          return
        }
        const session = this.sessions.find((item) => item.topic === topic)
        this.request = {
          topic,
          id,
          dapp: session?.name ?? 'Connected dapp',
          preview: (replay.preview ?? {}) as SendPreview,
          status: 'unknown',
          error: '',
          publishedResult: null,
          publishedHash: replay.publishedHash ?? '',
          sessionEnded: false,
          ...verify,
        }
        return
      }
      // outcome 'none' → a fresh request: apply the policy gates that replays
      // are allowed to skip, then claim the shared slot.
      if (this.request || this.preparingRequest) {
        await this.failRequest(topic, id, -32000, 'Another WalletConnect request is awaiting approval')
        return
      }
      const tx = useTxStore()
      if (tx.status === 'preparing' || tx.status === 'awaiting' || tx.status === 'publishing') {
        await this.failRequest(topic, id, -32000, 'The wallet is already handling another transaction')
        return
      }
      if (verify.isScam) {
        await this.failRequest(topic, id, 5000, 'Request blocked: WalletConnect Verify flagged this dapp as a known scam')
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
        const result = await Tx.PrepareWalletConnectSend(
          { ...(params.request.params as any), topic, requestId: id } as any,
        ) as unknown as WalletConnectPrepareOutcome
        // A journal record could appear between lookup and prepare (a race);
        // Prepare re-checks the journal, so honor a replay result here too.
        if (result.outcome === 'published' && result.published) {
          await this.deliverReplayPublished(topic, id, result, verify)
          return
        }
        if (result.outcome === 'unknown') {
          if (preparing.sessionEnded || preparing.cancelMessage) return
          const session = this.sessions.find((item) => item.topic === topic)
          this.request = {
            topic,
            id,
            dapp: session?.name ?? 'Connected dapp',
            preview: (result.preview ?? {}) as SendPreview,
            status: 'unknown',
            error: '',
            publishedResult: null,
            publishedHash: result.publishedHash ?? '',
            sessionEnded: preparing.sessionEnded,
            ...verify,
          }
          return
        }
        const preview = result.preview as SendPreview
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
        const expiry = Number(params?.request?.expiryTimestamp)
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
          expiryTimestamp: Number.isFinite(expiry) && expiry > 0 ? expiry : undefined,
          ...verify,
        }
        this.error = ''
      } catch (error) {
        if (preparing.sessionEnded) return
        if (preparing.cancelMessage) {
          await this.respondError(topic, id, preparing.cancelCode, preparing.cancelMessage).catch(() => {})
          return
        }
        if (wallet.locked || !wallet.activeAddress()) {
          await this.failRequest(topic, id, 9000, 'Wallet is locked')
          return
        }
        await this.failRequest(topic, id, -32602, messageOf(error))
      } finally {
        if (this.preparingRequest?.token === preparing.token) this.preparingRequest = null
      }
    },
    // deliverReplayPublished delivers a journaled published result (nothing is
    // signed) and acks only once the dapp has it. It claims the shared slot
    // ONLY when it is free, so session_delete / expire during the respond()
    // await can mark THIS delivery (round-3 finding 3) without displacing an
    // in-flight preparation. On a delivery failure it retains the standard
    // retryable delivery-error request when the wallet is idle, but when
    // another request is displayed it never clobbers it (round-4 finding P2):
    // the journal record survives for a later redelivery, and the failure is
    // surfaced non-destructively.
    async deliverReplayPublished(topic: string, id: number, replay: WalletConnectPrepareOutcome, verify: WalletConnectVerify) {
      const free = !this.request && !this.preparingRequest
      const marker: WalletConnectPreparingRequest | null = free
        ? { token: ++preparingRequestToken, topic, id, sessionEnded: false, cancelMessage: '', cancelCode: 5000 }
        : null
      if (marker) this.preparingRequest = marker
      try {
        const c = await this.ensureClient()
        await c.respond({ topic, response: resultResponse(id, replay.published) })
        await Tx.AckWalletConnectResult(topic, id).catch(() => {})
      } catch {
        const sessionEnded = marker?.sessionEnded ?? false
        const label = replay.publishedHash ? ` ${replay.publishedHash}` : ''
        if (!this.request) {
          const session = this.sessions.find((item) => item.topic === topic)
          this.request = {
            topic,
            id,
            dapp: session?.name ?? 'Connected dapp',
            preview: (replay.preview ?? {}) as SendPreview,
            status: 'delivery-error',
            error: sessionEnded
              ? `Transaction${label} was published, but the dapp session ended before it could be notified. Do not submit it again.`
              : `Transaction${label} was published, but the dapp could not be notified. Do not submit it again.`,
            publishedResult: replay.published,
            publishedHash: replay.publishedHash ?? '',
            sessionEnded,
            ...verify,
          }
        } else {
          // Another request occupies the modal: keep the journal record (it is
          // duplicate protection and survives) and inform without displacing it.
          this.error = `Transaction${label} was published, but the dapp could not be notified yet. Do not submit it again.`
        }
      } finally {
        if (marker && this.preparingRequest === marker) this.preparingRequest = null
      }
    },
    async approveRequest() {
      if (!this.request || this.request.status !== 'awaiting') return
      const current = this.request
      // Defense in depth alongside the session_request_expire listener: the
      // relay has already deleted an expired request, so approving it could
      // only publish funds with no counterpart to answer. Fail closed.
      if (current.expiryTimestamp && Date.now() >= current.expiryTimestamp * 1000) {
        this.request = null
        this.error = 'The WalletConnect request expired before it was approved'
        const holdID = current.preview.holdId ?? 0
        if (holdID) await Tx.CancelPending(holdID).catch(() => {})
        return
      }
      current.status = 'publishing'
      current.error = ''
      let result: unknown
      try {
        result = await Tx.ConfirmWalletConnectPublish(current.preview.holdId ?? 0)
      } catch (error) {
        const message = messageOf(error)
        if (message.includes(OUTCOME_UNKNOWN_MARKER)) {
          // The block may be on chain. The journaled signed block owns the
          // outcome now; only reconciliation may answer this request.
          current.status = 'unknown'
          current.error = message
          return
        }
        if (current.sessionEnded) {
          const holdID = current.preview.holdId ?? 0
          if (holdID) await Tx.CancelPending(holdID).catch(() => {})
          if (this.request === current) this.request = null
          return
        }
        current.status = 'error'
        current.error = message
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
        // The dapp holds the result; the journal record is no longer needed
        // for replay protection.
        await Tx.AckWalletConnectResult(current.topic, current.id).catch(() => {})
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
    async reconcileRequest() {
      const current = this.request
      if (!current || current.status !== 'unknown') return
      current.error = ''
      try {
        const result = await Tx.ReconcileWalletConnectPublication(current.topic, current.id)
        current.publishedResult = result
        current.publishedHash = String((result as any)?.hash ?? current.publishedHash ?? '')
        current.status = 'notifying'
        await this.deliverPublishedResult(current)
      } catch (error) {
        // Still unknown — retryable; never convert this into a rejection.
        current.status = 'unknown'
        current.error = messageOf(error)
      }
    },
    clearPublishedRequest() {
      // Closing locally never acks: the journal record must survive so a
      // redelivered request replays the stored outcome instead of re-signing.
      if (this.request?.status === 'delivery-error' || this.request?.status === 'unknown') this.request = null
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
      if (current.status === 'publishing' || current.status === 'notifying' || current.status === 'unknown') {
        // Let the single in-flight publication/delivery attempt settle. Calling
        // respond() again here could race it and produce two result responses.
        // An unknown outcome keeps its journal record for reconciliation.
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
    async handleRequestExpired(id: number) {
      const preparing = this.preparingRequest
      // The expired request no longer exists at the relay, so there is nothing
      // to answer; reuse the session-ended path, which cancels silently.
      if (preparing && preparing.id === id) preparing.sessionEnded = true
      const current = this.request
      if (!current || current.id !== id) return
      if (current.status === 'publishing' || current.status === 'notifying' || current.status === 'delivery-error' || current.status === 'unknown') {
        // Publication already started: only its actual outcome may drive the
        // terminal state. Marking the request ended suppresses any response
        // attempt for this id without inventing a rejection.
        current.sessionEnded = true
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
      if (this.request.status === 'publishing' || this.request.status === 'notifying' || this.request.status === 'delivery-error' || this.request.status === 'unknown') return
      const current = this.request
      this.request = null
      const holdID = current.preview.holdId ?? 0
      if (holdID) await Tx.CancelPending(holdID).catch(() => {})
      await this.respondError(current.topic, current.id, 9000, 'Wallet is locked')
    },
  },
})
