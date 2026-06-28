<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import { useBalancesStore } from '../stores/balances'
import { useTxsStore } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useNodeStore } from '../stores/node'
import { useTxStore } from '../stores/tx'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useUiStore } from '../stores/ui'
import TopBar from '../components/TopBar.vue'
import BalanceCard from '../components/BalanceCard.vue'
import ActionCard from '../components/ActionCard.vue'
import StatusStrip from '../components/StatusStrip.vue'
import TokensPanel from '../components/TokensPanel.vue'
import RewardsPanel from '../components/panels/RewardsPanel.vue'
import PlasmaPanel from '../components/panels/PlasmaPanel.vue'
import PillarPanel from '../components/panels/PillarPanel.vue'
import StakingPanel from '../components/panels/StakingPanel.vue'
import SentinelsPanel from '../components/panels/SentinelsPanel.vue'
import AcceleratorPanel from '../components/panels/AcceleratorPanel.vue'
import GovernancePanel from '../components/panels/GovernancePanel.vue'
import TxHistory from '../components/TxHistory.vue'
import SendModal from '../components/SendModal.vue'
import ReceiveModal from '../components/ReceiveModal.vue'
import NomConfirm from '../components/NomConfirm.vue'

const wallet = useWalletStore()
const balances = useBalancesStore()
const txs = useTxsStore()
const unreceived = useUnreceivedStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const node = useNodeStore()
const tx = useTxStore()
const autoReceive = useAutoReceiveStore()
const ui = useUiStore()
const route = useRoute()

// Governance is an experimental, testnet-only tab revealed via Settings; the
// other NoM tabs are always present.
const TABS = computed(() => {
  const base = ['Tokens', 'Rewards', 'Plasma', 'Pillar', 'Staking', 'Sentinels', 'Accelerator']
  return ui.showGovernance ? [...base, 'Governance'] : base
})
const active = ref('Tokens')
const initialSub = ref('')
const sendOpen = ref(false)
const receiveOpen = ref(false)

// Deep-link support: the top-bar ballot badge pushes ?tab=Accelerator&sub=Vote.
// Mirror those into the active tab + the panel's initial sub-view.
function applyQuery() {
  const t = route.query.tab
  if (typeof t === 'string' && TABS.value.includes(t)) active.value = t
  const sub = route.query.sub
  initialSub.value = typeof sub === 'string' ? sub : ''
}

// Reset the tx flow when switching tabs (mirrors the Svelte resetTx on tab
// change) so a half-built block doesn't leak across panels.
watch(active, () => tx.reset())

const znn = computed(() => balances.items.find((b) => b.symbol === 'ZNN'))
const qsr = computed(() => balances.items.find((b) => b.symbol === 'QSR'))

async function refresh() {
  await Promise.all([
    balances.load(),
    plasma.refresh(),
    pillar.refreshDelegation(),
    pillar.refreshMyPillar(),
    accelerator.refreshVotable(),
    txs.load(),
    unreceived.load(),
  ])
}

// On account switch: refresh data + re-point auto-receive at the new account.
async function onActiveChange(i: number) {
  txs.resetPage() // a new account starts at the first page of its history
  refresh()
  await autoReceive.followAccount(i)
}

watch(
  () => wallet.activeIndex,
  (i) => { if (i >= 0) onActiveChange(i) },
)

watch(() => route.query, applyQuery)

onMounted(async () => {
  applyQuery()
  node.initEvents(refresh)
  refresh()
  ui.init()
  await autoReceive.init(wallet.activeIndex)
})
</script>

<template>
  <TopBar />

  <div class="mx-auto mt-6 w-[56rem] max-w-full space-y-4 px-4 pb-12">
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <BalanceCard symbol="ZNN" :amount="znn?.amount ?? '0'" :decimals="znn?.decimals ?? 8" tint="green" />
      <BalanceCard symbol="QSR" :amount="qsr?.amount ?? '0'" :decimals="qsr?.decimals ?? 8" tint="blue" />
      <ActionCard label="Send" direction="send" @click="sendOpen = true" />
      <ActionCard
        label="Receive"
        direction="receive"
        :badge="unreceived.items.length"
        :receiving="autoReceive.receiving"
        @click="receiveOpen = true"
      />
    </div>

    <StatusStrip />

    <div class="rounded border border-border bg-card">
      <Tabs v-model="active">
        <TabsList class="w-full justify-start overflow-x-auto">
          <TabsTrigger v-for="t in TABS" :key="t" :value="t">{{ t }}</TabsTrigger>
        </TabsList>
        <TabsContent value="Tokens">
          <TokensPanel />
          <!-- Inset to match TokensPanel's p-4 so the history card lines up with
               the content above instead of meeting the outer card's border. -->
          <div class="px-4 pb-4">
            <TxHistory />
          </div>
        </TabsContent>
        <TabsContent value="Rewards"><RewardsPanel /></TabsContent>
        <TabsContent value="Plasma"><PlasmaPanel /></TabsContent>
        <TabsContent value="Pillar"><PillarPanel /></TabsContent>
        <TabsContent value="Staking"><StakingPanel /></TabsContent>
        <TabsContent value="Sentinels"><SentinelsPanel /></TabsContent>
        <TabsContent value="Accelerator"><AcceleratorPanel :initial-sub="initialSub" /></TabsContent>
        <TabsContent v-if="ui.showGovernance" value="Governance"><GovernancePanel /></TabsContent>
      </Tabs>
    </div>
  </div>

  <SendModal v-model:open="sendOpen" />
  <ReceiveModal v-model:open="receiveOpen" />
  <!-- Global confirm for panel-triggered tx. Gated so it never collides with the
       Send/Receive dialogs, which render their own TxModal. -->
  <NomConfirm v-if="!sendOpen && !receiveOpen" />
</template>
