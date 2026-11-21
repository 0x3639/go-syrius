<script setup lang="ts">
import { watch } from 'vue'
import { useNodeStore } from '../stores/node'
import { useStakeStore } from '../stores/stake'
import { usePillarStore } from '../stores/pillar'
import { useSentinelStore } from '../stores/sentinel'
import { useWalletStore } from '../stores/wallet'
import { useUiStore } from '../stores/ui'
import { useNodeRefresh } from '../lib/useNodeRefresh'
import HomeEveryday from './HomeEveryday.vue'
import OverviewAdvanced from './OverviewAdvanced.vue'

// The dashboard route is Home in Everyday and the operator Overview in
// Advanced. Both read the same stores, so switching mode re-renders in place:
// same account, same network, same data.
const node = useNodeStore()
const stake = useStakeStore()
const pillar = usePillarStore()
const sentinel = useSentinelStore()
const wallet = useWalletStore()
const ui = useUiStore()

// The shell's per-momentum refresh covers balances, plasma and delegation but
// not staking or reward snapshots, which only their own panels load. Fetch
// them here — on mount, on reconnect and on account switch — so committed
// amounts and rewards are not blank until the user opens Staking or Rewards.
function refreshEarning() {
  if (!node.connected) return
  stake.refresh()
  pillar.refresh()
  sentinel.refresh()
}
useNodeRefresh(refreshEarning)
watch(() => wallet.activeIndex, refreshEarning)
</script>

<template>
  <HomeEveryday v-if="ui.mode === 'everyday'" />
  <OverviewAdvanced v-else />
</template>
