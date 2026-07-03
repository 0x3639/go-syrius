<script setup lang="ts">
// Thin wrapper for each NoM feature route. Renders the panel named by the
// route's meta.panel. (Half-built tx state is reset by the router's global
// afterEach on every navigation, including between /network/* routes.)
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useUiStore } from '../stores/ui'
import { useTxStore } from '../stores/tx'
import PlasmaPanel from '../components/panels/PlasmaPanel.vue'
import StakingPanel from '../components/panels/StakingPanel.vue'
import PillarPanel from '../components/panels/PillarPanel.vue'
import SentinelsPanel from '../components/panels/SentinelsPanel.vue'
import AcceleratorPanel from '../components/panels/AcceleratorPanel.vue'
import RewardsPanel from '../components/panels/RewardsPanel.vue'
import GovernancePanel from '../components/panels/GovernancePanel.vue'

const PANELS: Record<string, any> = {
  plasma: PlasmaPanel, staking: StakingPanel, pillars: PillarPanel,
  sentinels: SentinelsPanel, accelerator: AcceleratorPanel, rewards: RewardsPanel,
  governance: GovernancePanel,
}
const route = useRoute()
const ui = useUiStore()
const panelKey = computed(() => route.meta.panel as string)
const panel = computed(() => PANELS[panelKey.value])
// Accelerator deep-link: ?sub=Vote drives the panel's initial sub-view.
const initialSub = computed(() => (typeof route.query.sub === 'string' ? route.query.sub : ''))

// TESTNET-ONLY Governance: gate the panel itself reactively (single predicate
// shared with the Sidebar via ui.governanceAllowed), so the UI vanishes rather
// than staying interactive if the node connects to mainnet or the Settings
// opt-in is turned off while the route is open.
const governanceBlocked = computed(
  () => panelKey.value === 'governance' && !ui.governanceAllowed,
)
// While the gate is shut, no prepared-but-unconfirmed block may survive:
// CANCEL it via the same path as NomConfirm's dialog-close (backend
// CancelPending releases the held block, then the frontend state resets).
// Watching BOTH the gate and the tx status covers the in-flight-Prepare hole:
// a Prepare RPC that was already running when the gate closed resolves later
// and only then flips status to 'awaiting' — the watcher re-fires and cancels
// it before the dialog can publish a testnet-only block on mainnet. Only
// 'awaiting' is cancelled: once publishing has started the block is already
// submitted and its result/error must surface normally.
const tx = useTxStore()
watch(
  [governanceBlocked, () => tx.status],
  ([blocked, status]) => { if (blocked && status === 'awaiting') tx.cancel() },
)
</script>

<template>
  <!-- Cap every NoM page to the same centered width as the Tokens page so the
       section reads consistently instead of sprawling full-width. -->
  <div class="mx-auto max-w-[48rem]">
    <p v-if="governanceBlocked" class="text-sm text-muted-foreground">
      Governance is testnet-only. Enable it in Settings and connect to a testnet node.
    </p>
    <component :is="panel" v-else v-bind="panelKey === 'accelerator' ? { initialSub } : {}" />
  </div>
</template>
