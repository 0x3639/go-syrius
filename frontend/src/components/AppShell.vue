<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import NomConfirm from './NomConfirm.vue'
import WalletConnectRequest from './WalletConnectRequest.vue'
import { usePriceStore } from '../stores/price'
import { useNodeStore } from '../stores/node'
import { useBalancesStore } from '../stores/balances'
import { usePlasmaStore } from '../stores/plasma'
import { useStakeStore } from '../stores/stake'
import { useCollateralStore } from '../stores/collateral'
import { useSentinelStore } from '../stores/sentinel'
import { currentRequestEpoch } from '../lib/requestEpoch'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useTxsStore } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
import { useWalletConnectStore } from '../stores/walletconnect'
import { NoteActivity } from '../../wailsjs/go/app/WalletService'

const route = useRoute()
const price = usePriceStore()
const node = useNodeStore()
const balances = useBalancesStore()
const plasma = usePlasmaStore()
const stake = useStakeStore()
const collateral = useCollateralStore()
const sentinel = useSentinelStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const txs = useTxsStore()
const unreceived = useUnreceivedStore()
const ui = useUiStore()
const autoReceive = useAutoReceiveStore()
const wallet = useWalletStore()
const tx = useTxStore()
const walletConnect = useWalletConnectStore()
// The dashboard route is Home in Everyday and the operator Overview in Advanced.
const HOME_TITLES = {
  everyday: ['Home', 'What you own, and anything that needs you.'],
  advanced: ['Overview', 'What this account holds, runs and needs to act on.'],
} as const
const title = computed(() => route.name === 'dashboard' ? HOME_TITLES[ui.mode][0] : (route.meta.title as string) ?? '')
const subtitle = computed(() => route.name === 'dashboard' ? HOME_TITLES[ui.mode][1] : (route.meta.subtitle as string) ?? '')

// Global bootstrap. AppShell wraps every authenticated route and unmounts only
// on lock, so this is the single place the app re-hydrates after an unlock —
// relocated here from the deleted Home.vue (the tab deep-link applyQuery() is
// intentionally dropped; NetworkPage handles route.query.sub now).
// Coalesced: momentum ticks arrive faster than seven RPC groups can resolve,
// so overlapping calls collapse into the running one plus at most one trailing
// re-run (the trailing run guarantees an account switch mid-refresh still
// lands its data).
let activeRefresh: { epoch: number; queued: boolean } | null = null
async function refresh() {
  if (wallet.locked || !node.connected) return
  const epoch = currentRequestEpoch()
  if (activeRefresh?.epoch === epoch) {
    activeRefresh.queued = true
    return
  }
  const run = { epoch, queued: false }
  activeRefresh = run
  try {
    await Promise.all([
      balances.load(),
      plasma.refresh(),
      pillar.refreshDelegation(),
      pillar.refreshMyPillar(),
      accelerator.refreshVotable(),
      txs.load(),
      unreceived.load(),
    ])
  } finally {
    if (activeRefresh === run) {
      activeRefresh = null
      if (run.queued && epoch === currentRequestEpoch()) refresh()
    }
  }
}

function invalidateWalletData() {
  balances.invalidate()
  txs.resetPage()
  plasma.invalidate()
  unreceived.invalidate()
  stake.invalidate()
  collateral.invalidate()
  pillar.invalidate()
  sentinel.invalidate()
  accelerator.invalidate()
}

// On account switch: reset history paging, refresh data, re-point auto-receive.
async function onActiveChange(i: number) {
  const epoch = currentRequestEpoch()
  invalidateWalletData()
  pillar.stopPolling()
  sentinel.stopPolling()
  refresh()
  await autoReceive.followAccount(i)
  if (wallet.locked || epoch !== currentRequestEpoch()) return
  await walletConnect.updateAccount(wallet.activeAddress())
}

watch(
  () => wallet.activeIndex,
  (i) => { if (!wallet.locked && i >= 0) onActiveChange(i) },
  { flush: 'sync' },
)

watch(() => node.connected, (connected) => {
  invalidateWalletData()
  if (connected) {
    refresh()
    // Disconnect stops the backend worker even when its saved preference and
    // account are unchanged. Reconnect must explicitly restore that worker.
    if (!wallet.locked && autoReceive.enabled && wallet.activeIndex >= 0) {
      autoReceive.start(wallet.activeIndex).catch(() => {})
    }
  } else tx.discard()
}, { flush: 'sync' })

// Activity pings for the backend auto-lock watchdog. Throttled: the watchdog
// only needs coarse "user is here" signals, one binding call per 15s max.
// pointerdown/keydown/wheel cover genuine interaction without mousemove chatter.
const ACTIVITY_THROTTLE_MS = 15_000
const ACTIVITY_EVENTS = ['pointerdown', 'keydown', 'wheel'] as const
let lastActivityPing = 0
function onUserActivity() {
  const now = Date.now()
  if (now - lastActivityPing < ACTIVITY_THROTTLE_MS) return
  lastActivityPing = now
  NoteActivity().catch(() => {})
}

onMounted(async () => {
  price.start()
  tx.initEvents() // wires tx:pow-progress so the confirm dialog shows live PoW state
  node.initEvents(refresh) // wires node:status/sync/momentum:tick + drives the sync pill + live refresh
  // Register the backend-initiated lock listener (auto-lock watchdog) for local
  // session teardown. Navigation on lock is owned by App.vue's `wallet.locked`
  // watcher (App.vue:34-39), not this call.
  wallet.initLockEvent()
  for (const e of ACTIVITY_EVENTS) window.addEventListener(e, onUserActivity, { capture: true, passive: true })
  refresh() // initial aggregate load (balances etc.)
  ui.init() // restore persisted appearance preferences
  await autoReceive.init(wallet.activeIndex)
  if (walletConnect.projectId()) {
    try {
      await walletConnect.ensureClient()
      // Restored WalletConnect sessions can still advertise the account from
      // before an app restart. Reconcile them immediately after unlock.
      await walletConnect.updateAccount(wallet.activeAddress())
    } catch { /* relay initialization remains retryable from the WC screen */ }
  }
})
onBeforeUnmount(() => {
  activeRefresh = null
  price.stop()
  node.clearTick() // stop momentum-driven refreshes while locked
  for (const e of ACTIVITY_EVENTS) window.removeEventListener(e, onUserActivity, { capture: true })
  walletConnect.walletLocked().catch(() => {})
})
</script>

<template>
  <div class="flex h-screen bg-background">
    <Sidebar />
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <TopBar :title="title" :subtitle="subtitle" />
      <main class="min-h-0 flex-1 overflow-y-auto px-4 pb-8 pt-4 lg:px-8">
        <router-view />
      </main>
    </div>

    <!-- Global confirm-what-you-sign dialog. Every NoM panel (Rewards, Pillars,
         Sentinels, Staking, Plasma, Accelerator, Tokens) prepares a
         call and hands the built-block preview to tx.awaitConfirm, then relies on
         this dialog to confirm + publish. Rendered app-wide EXCEPT on the Transfer
         route, which drives its own inline TxModal/TxResult (avoids a double
         dialog on the same tx status). -->
    <NomConfirm v-if="route.name !== 'transfer'" />
    <WalletConnectRequest />
  </div>
</template>
