<script setup lang="ts">
// Thin wrapper for each NoM feature route. Renders the panel named by the
// route's meta.panel. (Half-built tx state is reset by the router's global
// afterEach on every navigation, including between /network/* routes.)
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useUiStore } from '../stores/ui'
import { useNodeStore } from '../stores/node'
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
const node = useNodeStore()
const panelKey = computed(() => route.meta.panel as string)
const panel = computed(() => PANELS[panelKey.value])
// Accelerator deep-link: ?sub=Vote drives the panel's initial sub-view.
const initialSub = computed(() => (typeof route.query.sub === 'string' ? route.query.sub : ''))

// TESTNET-ONLY Governance: mirror the Sidebar's gate reactively on the panel
// itself, so the UI vanishes (rather than staying interactive) if the node
// connects to mainnet (chainId 1) or the Settings opt-in is turned off while
// the route is open.
const governanceBlocked = computed(
  () => panelKey.value === 'governance' && (!ui.showGovernance || node.chainId === 1),
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
