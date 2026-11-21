<script setup lang="ts">
import { watch } from 'vue'
import { useNodeStore } from '../stores/node'
import { useBalancesStore } from '../stores/balances'
import { ZNN_ZTS } from '../lib/useHoldings'
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
const balances = useBalancesStore()
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

// Holdings add staked ZNN to the spendable balance. The balance refreshes on
// every momentum but the stake snapshot does not, so a stake or withdrawal
// (here or from another client) would be counted twice or not at all until
// the next remount. Reload staking whenever the ZNN balance moves, including
// a ZNN row appearing or disappearing (GetBalances omits a zero balance, so
// an all-staked account gains its row when a stake is withdrawn). resync
// marks the old snapshot stale immediately, so holdings never add it to the
// new balance while the reload is in flight. Sync flush: the mark must be set
// before anything re-reads holdings.
watch(
  () => balances.items.find((b) => b.zts === ZNN_ZTS)?.amount ?? '',
  () => { if (node.connected) stake.resync() },
  { flush: 'sync' },
)
</script>

<template>
  <HomeEveryday v-if="ui.mode === 'everyday'" />
  <OverviewAdvanced v-else />
</template>
