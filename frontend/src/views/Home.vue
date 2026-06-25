<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import { useBalancesStore } from '../stores/balances'
import { useTxsStore } from '../stores/txs'
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useNodeStore } from '../stores/node'
import { useTxStore } from '../stores/tx'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import * as N from '../../wailsjs/go/app/NodeService'
import AccountSwitcher from '../components/AccountSwitcher.vue'
import BalanceCard from '../components/BalanceCard.vue'
import ActionCard from '../components/ActionCard.vue'
import StatusStrip from '../components/StatusStrip.vue'
import TokensPanel from '../components/TokensPanel.vue'
import PanelPlaceholder from '../components/PanelPlaceholder.vue'
import TxHistory from '../components/TxHistory.vue'
import SendModal from '../components/SendModal.vue'
import ReceiveModal from '../components/ReceiveModal.vue'

const router = useRouter()
const wallet = useWalletStore()
const balances = useBalancesStore()
const txs = useTxsStore()
const unreceived = useUnreceivedStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const node = useNodeStore()
const tx = useTxStore()

const TABS = ['Tokens', 'Rewards', 'Plasma', 'Pillar', 'Staking', 'Sentinels', 'Accelerator']
const active = ref('Tokens')
const sendOpen = ref(false)
const receiveOpen = ref(false)
const autoReceive = ref(false)

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
    txs.load(),
    unreceived.load(),
  ])
}

// Auto-receive is bound to a single account on the backend (it subscribes +
// sweeps for whichever address is active when it starts). startAR() always
// stops first so it re-points at the CURRENT active account — StartAutoReceive
// alone would early-return as "already running" and keep watching the old one.
let arAccount = -1
async function startAR() {
  await N.StopAutoReceive()
  await N.StartAutoReceive()
  arAccount = wallet.activeIndex
}
async function stopAR() {
  await N.StopAutoReceive()
  arAccount = -1
}

async function toggleAutoReceive() {
  try {
    const s = await Cfg.GetSettings()
    s.autoReceive = autoReceive.value
    await Cfg.SetSettings(s)
    if (autoReceive.value) await startAR()
    else await stopAR()
  } catch {}
}

async function onActiveChange(active: number) {
  refresh()
  // Follow account switches: re-sweep + re-subscribe for the new account.
  if (autoReceive.value && active !== arAccount) await startAR()
}

// Drive onActiveChange off the active account index (the Svelte `$: if
// ($wallet.active >= 0)` reactive statement).
watch(
  () => wallet.activeIndex,
  (i) => { if (i >= 0) onActiveChange(i) },
)

onMounted(async () => {
  node.initEvents(refresh)
  refresh()
  try {
    autoReceive.value = (await Cfg.GetSettings()).autoReceive
    if (autoReceive.value) await startAR() // resume (subscription doesn't survive a restart)
  } catch {}
})
</script>

<template>
  <div class="mx-auto mt-6 w-[56rem] max-w-full space-y-4 px-4">
    <div class="flex items-center justify-between">
      <AccountSwitcher />
      <div class="flex items-center gap-3">
        <label class="flex items-center gap-1 text-xs text-muted-foreground">
          <input type="checkbox" v-model="autoReceive" @change="toggleAutoReceive" /> Auto-receive
        </label>
        <Button variant="ghost" aria-label="Settings" @click="router.push('/settings')">Settings</Button>
        <Button variant="ghost" @click="wallet.lock()">Lock</Button>
      </div>
    </div>

    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <BalanceCard symbol="ZNN" :amount="znn?.amount ?? '0'" :decimals="znn?.decimals ?? 8" tint="green" />
      <BalanceCard symbol="QSR" :amount="qsr?.amount ?? '0'" :decimals="qsr?.decimals ?? 8" tint="blue" />
      <ActionCard label="Send" direction="send" @click="sendOpen = true" />
      <ActionCard
        label="Receive"
        direction="receive"
        :badge="unreceived.items.length"
        @click="receiveOpen = true"
      />
    </div>

    <StatusStrip />

    <div class="rounded border border-border bg-card">
      <Tabs v-model="active">
        <TabsList class="w-full justify-start overflow-x-auto">
          <TabsTrigger v-for="t in TABS" :key="t" :value="t">{{ t }}</TabsTrigger>
        </TabsList>
        <TabsContent value="Tokens"><TokensPanel /></TabsContent>
        <TabsContent value="Rewards"><PanelPlaceholder name="Rewards" /></TabsContent>
        <TabsContent value="Plasma"><PanelPlaceholder name="Plasma" /></TabsContent>
        <TabsContent value="Pillar"><PanelPlaceholder name="Pillar" /></TabsContent>
        <TabsContent value="Staking"><PanelPlaceholder name="Staking" /></TabsContent>
        <TabsContent value="Sentinels"><PanelPlaceholder name="Sentinels" /></TabsContent>
        <TabsContent value="Accelerator"><PanelPlaceholder name="Accelerator" /></TabsContent>
      </Tabs>
    </div>

    <TxHistory />
  </div>

  <SendModal v-model:open="sendOpen" />
  <ReceiveModal v-model:open="receiveOpen" />
</template>
