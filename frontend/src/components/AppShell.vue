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
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useTxsStore } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useWalletStore } from '../stores/wallet'
import { useTxStore } from '../stores/tx'
import { useWalletConnectStore } from '../stores/walletconnect'

const route = useRoute()
const price = usePriceStore()
const node = useNodeStore()
const balances = useBalancesStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const txs = useTxsStore()
const unreceived = useUnreceivedStore()
const ui = useUiStore()
const autoReceive = useAutoReceiveStore()
const wallet = useWalletStore()
const tx = useTxStore()
const walletConnect = useWalletConnectStore()
const title = computed(() => (route.meta.title as string) ?? '')

// Global bootstrap. AppShell wraps every authenticated route and unmounts only
// on lock, so this is the single place the app re-hydrates after an unlock —
// relocated here from the deleted Home.vue (the tab deep-link applyQuery() is
// intentionally dropped; NetworkPage handles route.query.sub now).
// Coalesced: momentum ticks arrive faster than seven RPC groups can resolve,
// so overlapping calls collapse into the running one plus at most one trailing
// re-run (the trailing run guarantees an account switch mid-refresh still
// lands its data).
let refreshing = false
let refreshQueued = false
async function refresh() {
  if (refreshing) {
    refreshQueued = true
    return
  }
  refreshing = true
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
    refreshing = false
    if (refreshQueued) {
      refreshQueued = false
      refresh()
    }
  }
}

// On account switch: reset history paging, refresh data, re-point auto-receive.
async function onActiveChange(i: number) {
  txs.resetPage()
  refresh()
  await autoReceive.followAccount(i)
  await walletConnect.updateAccount(wallet.activeAddress())
}

watch(
  () => wallet.activeIndex,
  (i) => { if (i >= 0) onActiveChange(i) },
)

onMounted(async () => {
  price.start()
  tx.initEvents() // wires tx:pow-progress so the confirm dialog shows live PoW state
  node.initEvents(refresh) // wires node:status/sync/momentum:tick + drives the sync pill + live refresh
  refresh() // initial aggregate load (balances etc.)
  ui.init() // restore persisted theme + showGovernance
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
  price.stop()
  node.clearTick() // stop momentum-driven refreshes while locked
  walletConnect.walletLocked().catch(() => {})
})
</script>

<template>
  <div class="flex h-screen bg-background">
    <Sidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <TopBar :title="title" />
      <main class="flex-1 overflow-y-auto p-7">
        <router-view />
      </main>
    </div>

    <!-- Global confirm-what-you-sign dialog. Every NoM panel (Rewards, Pillars,
         Sentinels, Staking, Plasma, Accelerator, Governance, Tokens) prepares a
         call and hands the built-block preview to tx.awaitConfirm, then relies on
         this dialog to confirm + publish. Rendered app-wide EXCEPT on the Transfer
         route, which drives its own inline TxModal/TxResult (avoids a double
         dialog on the same tx status). -->
    <NomConfirm v-if="route.name !== 'transfer'" />
    <WalletConnectRequest />
  </div>
</template>
