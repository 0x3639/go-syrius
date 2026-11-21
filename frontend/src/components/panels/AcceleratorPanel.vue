<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useAcceleratorStore } from '../../stores/accelerator'
import { usePillarStore } from '../../stores/pillar'
import { useWalletStore } from '../../stores/wallet'
import AcceleratorVote from './AcceleratorVote.vue'
import AcceleratorProjects from './AcceleratorProjects.vue'
import AcceleratorCreate from './AcceleratorCreate.vue'
import AcceleratorDonate from './AcceleratorDonate.vue'

const props = defineProps<{ initialSub?: string }>()
const acc = useAcceleratorStore()
const pillar = usePillarStore()
const wallet = useWalletStore()
const { ownsPillar } = storeToRefs(pillar)

const sub = ref(props.initialSub || (ownsPillar.value ? 'Vote' : 'Projects'))
watch(
  () => props.initialSub,
  (v) => {
    if (v) sub.value = v
  },
)

function load() {
  acc.refreshVotable()
  acc.loadProjects()
  acc.loadVotablePillars()
}
useNodeRefresh(load)
watch(() => wallet.activeIndex, load)
</script>

<template>
  <div>
    <Tabs v-model="sub">
      <TabsList class="justify-start">
        <TabsTrigger value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Projects">Projects</TabsTrigger>
        <TabsTrigger value="Create">Create</TabsTrigger>
        <TabsTrigger value="Donate">Donate</TabsTrigger>
      </TabsList>
      <TabsContent class="mt-5" value="Vote"><AcceleratorVote /></TabsContent>
      <TabsContent class="mt-5" value="Projects"><AcceleratorProjects /></TabsContent>
      <TabsContent class="mt-5" value="Create"><AcceleratorCreate /></TabsContent>
      <TabsContent class="mt-5" value="Donate"><AcceleratorDonate /></TabsContent>
    </Tabs>
  </div>
</template>
