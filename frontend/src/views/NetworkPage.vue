<script setup lang="ts">
// Thin wrapper for each NoM feature route. Renders the panel named by the
// route's meta.panel. (Half-built tx state is reset by the router's global
// afterEach on every navigation, including between /network/* routes.)
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PlasmaPanel from '../components/panels/PlasmaPanel.vue'
import StakingPanel from '../components/panels/StakingPanel.vue'
import PillarPanel from '../components/panels/PillarPanel.vue'
import SentinelsPanel from '../components/panels/SentinelsPanel.vue'
import AcceleratorPanel from '../components/panels/AcceleratorPanel.vue'
import RewardsPanel from '../components/panels/RewardsPanel.vue'
import BalanceNotice from '../components/BalanceNotice.vue'
import { useNodeStore } from '../stores/node'

const PANELS: Record<string, any> = {
  plasma: PlasmaPanel, staking: StakingPanel, pillars: PillarPanel,
  sentinels: SentinelsPanel, accelerator: AcceleratorPanel, rewards: RewardsPanel,
}
const route = useRoute()
const node = useNodeStore()
const panelKey = computed(() => route.meta.panel as string)
const panel = computed(() => PANELS[panelKey.value])
// Accelerator deep-link: ?sub=Vote drives the panel's initial sub-view.
const initialSub = computed(() => (typeof route.query.sub === 'string' ? route.query.sub : ''))
</script>

<template>
  <!-- Cap every NoM page to the same centered width as the Tokens page so the
       section reads consistently instead of sprawling full-width. -->
  <div class="page-content">
    <BalanceNotice />
    <component :is="panel" v-if="node.connected" v-bind="panelKey === 'accelerator' ? { initialSub } : {}" />
  </div>
</template>
