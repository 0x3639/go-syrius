<script setup lang="ts">
// Thin wrapper for each NoM feature route. Renders the panel named by the
// route's meta.panel and resets the tx flow on enter so a half-built block
// can't leak in from another feature.
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
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
const tx = useTxStore()
const panelKey = computed(() => route.meta.panel as string)
const panel = computed(() => PANELS[panelKey.value])
// Accelerator deep-link: ?sub=Vote drives the panel's initial sub-view.
const initialSub = computed(() => (typeof route.query.sub === 'string' ? route.query.sub : ''))

watch(panelKey, () => tx.reset())
</script>

<template>
  <component :is="panel" v-bind="panelKey === 'accelerator' ? { initialSub } : {}" />
</template>
