<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { usePillarStore } from '../../stores/pillar'
import PillarDelegate from './PillarDelegate.vue'
import PillarLaunch from './PillarLaunch.vue'
import PillarActive from './PillarActive.vue'

// Container: "Delegate" keeps the existing delegation flow; "Run a Pillar" shows
// the owned-pillar view if one exists, else the registration wizard. The wizard
// step is derived from chain state by the children.
const pillarStore = usePillarStore()
const { ownsPillar } = storeToRefs(pillarStore)
const sub = ref('Delegate')

onMounted(() => pillarStore.refreshRegistration())
onUnmounted(() => pillarStore.stopPolling())
</script>

<template>
  <div class="p-4">
    <Tabs v-model="sub">
      <TabsList class="w-full justify-start">
        <TabsTrigger value="Delegate">Delegate</TabsTrigger>
        <TabsTrigger value="Run a Pillar">Run a Pillar</TabsTrigger>
      </TabsList>
      <TabsContent value="Delegate"><PillarDelegate /></TabsContent>
      <TabsContent value="Run a Pillar">
        <PillarActive v-if="ownsPillar" />
        <PillarLaunch v-else />
      </TabsContent>
    </Tabs>
  </div>
</template>
