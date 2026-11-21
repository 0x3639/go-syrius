<script setup lang="ts">
import { ref } from 'vue'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useNodeStore } from '../stores/node'
import RewardsPanel from '../components/panels/RewardsPanel.vue'
import StakingPanel from '../components/panels/StakingPanel.vue'
import PillarDelegate from '../components/panels/PillarDelegate.vue'
import BalanceNotice from '../components/BalanceNotice.vue'

// Everyday's home for everything that earns: the existing rewards, staking and
// delegation panels, unchanged, under one destination. Delegating to a pillar
// is a holder action and lives here; running a pillar is an Advanced tool.
const node = useNodeStore()
const tab = ref('Rewards')
</script>

<template>
  <div class="page-content">
    <BalanceNotice />
    <div v-show="node.connected">
      <Tabs v-model="tab">
        <TabsList class="justify-start">
          <TabsTrigger value="Rewards">Rewards</TabsTrigger>
          <TabsTrigger value="Staking">Staking</TabsTrigger>
          <TabsTrigger value="Delegation">Delegation</TabsTrigger>
        </TabsList>
        <TabsContent class="mt-5" value="Rewards"><RewardsPanel /></TabsContent>
        <TabsContent class="mt-5" value="Staking"><StakingPanel /></TabsContent>
        <TabsContent class="mt-5" value="Delegation"><PillarDelegate /></TabsContent>
      </Tabs>
    </div>
  </div>
</template>
