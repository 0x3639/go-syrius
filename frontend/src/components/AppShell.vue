<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
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
const title = computed(() => (route.meta.title as string) ?? '')

// Global bootstrap. AppShell wraps every authenticated route and unmounts only
// on lock, so this is the single place the app re-hydrates after an unlock —
// relocated here from the deleted Home.vue (the tab deep-link applyQuery() is
// intentionally dropped; NetworkPage handles route.query.sub now).
async function refresh() {
  await Promise.all([
    balances.load(),
    plasma.refresh(),
    pillar.refreshDelegation(),
    pillar.refreshMyPillar(),
    accelerator.refreshVotable(),
    txs.load(),
    unreceived.load(),
  ])
}

// On account switch: reset history paging, refresh data, re-point auto-receive.
async function onActiveChange(i: number) {
  txs.resetPage()
  refresh()
  await autoReceive.followAccount(i)
}

watch(
  () => wallet.activeIndex,
  (i) => { if (i >= 0) onActiveChange(i) },
)

onMounted(async () => {
  price.start()
  node.initEvents(refresh) // wires node:status/sync/momentum:tick + drives the sync pill + live refresh
  refresh() // initial aggregate load (balances etc.)
  ui.init() // restore persisted theme + showGovernance
  await autoReceive.init(wallet.activeIndex)
})
onBeforeUnmount(() => price.stop())
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
  </div>
</template>
