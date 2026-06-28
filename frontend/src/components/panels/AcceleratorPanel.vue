<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
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
onMounted(load)
watch(() => wallet.activeIndex, load)
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger value="Vote">Vote</TabsTrigger>
        <TabsTrigger value="Projects">Projects</TabsTrigger>
        <TabsTrigger value="Create">Create</TabsTrigger>
        <TabsTrigger value="Donate">Donate</TabsTrigger>
      </TabsList>
      <TabsContent value="Vote"><AcceleratorVote /></TabsContent>
      <TabsContent value="Projects"><AcceleratorProjects /></TabsContent>
      <TabsContent value="Create"><AcceleratorCreate /></TabsContent>
      <TabsContent value="Donate"><AcceleratorDonate /></TabsContent>
    </Tabs>
  </div>
</template>
