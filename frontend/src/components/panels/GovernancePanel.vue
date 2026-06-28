<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useGovernanceStore } from '../../stores/governance'
import { useWalletStore } from '../../stores/wallet'
import GovernanceVote from './GovernanceVote.vue'
import GovernanceActions from './GovernanceActions.vue'
import GovernancePropose from './GovernancePropose.vue'

const props = defineProps<{ initialSub?: string }>()
const gov = useGovernanceStore()
const wallet = useWalletStore()

const sub = ref(props.initialSub || 'Actions')
watch(
  () => props.initialSub,
  (v) => {
    if (v) sub.value = v
  },
)

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
        <TabsTrigger value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Actions">Actions</TabsTrigger>
        <TabsTrigger value="Propose">Propose</TabsTrigger>
      </TabsList>
      <TabsContent value="Vote"><GovernanceVote /></TabsContent>
      <TabsContent value="Actions"><GovernanceActions /></TabsContent>
      <TabsContent value="Propose"><GovernancePropose /></TabsContent>
    </Tabs>
  </div>
</template>
