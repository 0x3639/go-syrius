<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useGovernanceStore } from '../../stores/governance'
import { useWalletStore } from '../../stores/wallet'
import GovernanceVote from './GovernanceVote.vue'
import GovernanceActions from './GovernanceActions.vue'
import GovernancePropose from './GovernancePropose.vue'

const props = defineProps<{ initialSub?: string }>()
const gov = useGovernanceStore()
const wallet = useWalletStore()
const { votablePillars } = storeToRefs(gov)

// Only a pillar owner can vote, so the Vote sub-tab is shown only when the
// active wallet account owns at least one pillar (switch accounts to vote as a
// pillar). The pillar set is per-account (GetVotablePillars(activeAddress)).
const ownsPillar = computed(() => votablePillars.value.length > 0)

const sub = ref(props.initialSub || 'Actions')
watch(
  () => props.initialSub,
  (v) => {
    if (v) sub.value = v
  },
)
// If the active account loses pillar ownership (account switch) while Vote is
// open, fall back to Actions so the Tabs body isn't left empty.
watch(ownsPillar, (owns) => {
  if (!owns && sub.value === 'Vote') sub.value = 'Actions'
})

function load() {
  gov.loadActions()
  gov.loadVotablePillars()
  gov.loadActivePillarCount()
  gov.loadProposeKinds()
}
onMounted(load)
watch(() => wallet.activeIndex, load)
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger v-if="ownsPillar" value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Actions">Actions</TabsTrigger>
        <TabsTrigger value="Propose">Propose</TabsTrigger>
      </TabsList>
      <TabsContent v-if="ownsPillar" value="Vote"><GovernanceVote /></TabsContent>
      <TabsContent value="Actions"><GovernanceActions /></TabsContent>
      <TabsContent value="Propose"><GovernancePropose /></TabsContent>
    </Tabs>
  </div>
</template>
