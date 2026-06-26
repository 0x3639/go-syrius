<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Tabs, TabsList, TabsTrigger, TabsContent } from 'nom-ui'
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
import { plasmaLevel, plasmaColorClass } from '../lib/plasma'
import AccountSlotPicker from '../components/AccountSlotPicker.vue'
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
import TxHistory from '../components/TxHistory.vue'
import SendModal from '../components/SendModal.vue'
import ReceiveModal from '../components/ReceiveModal.vue'
import NomConfirm from '../components/NomConfirm.vue'

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

// Plasma bolt indicator: level + colour (off → red → yellow → green).
const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaColor = computed(() => plasmaColorClass(plasmaLvl.value))

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

// Icon button: flip the state then run the persist + start/stop logic.
async function clickAutoReceive() {
  autoReceive.value = !autoReceive.value
  await toggleAutoReceive()
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
  <header class="w-full border-b border-border bg-card px-6 py-3">
    <div class="flex items-center justify-between">
      <AccountSlotPicker />
      <div class="flex items-center gap-1">
        <button
          type="button"
          :title="`Plasma: ${plasmaLvl}`"
          :aria-label="`Plasma: ${plasmaLvl}`"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors hover:bg-foreground/[0.06]"
          :class="plasmaColor"
          @click="active = 'Plasma'"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/></svg>
        </button>
        <button
          type="button"
          :title="autoReceive ? 'Auto-receive: on' : 'Auto-receive: off'"
          :aria-label="autoReceive ? 'Auto-receive on' : 'Auto-receive off'"
          :aria-pressed="autoReceive"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors hover:bg-foreground/[0.06]"
          :class="autoReceive ? 'text-primary' : 'text-muted-foreground'"
          @click="clickAutoReceive"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 8v8M8 12l4 4 4-4"/></svg>
        </button>
        <span class="mx-1 h-5 w-px bg-border"></span>
        <button
          type="button"
          title="Lock wallet"
          aria-label="Lock wallet"
          class="grid h-9 w-9 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
          @click="wallet.lock()"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/></svg>
        </button>
        <button
          type="button"
          title="Settings"
          aria-label="Settings"
          class="grid h-9 w-9 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
          @click="router.push('/settings')"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        </button>
      </div>
    </div>
  </header>

  <div class="mx-auto mt-6 w-[56rem] max-w-full space-y-4 px-4">
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
        <TabsContent value="Rewards"><RewardsPanel /></TabsContent>
        <TabsContent value="Plasma"><PlasmaPanel /></TabsContent>
        <TabsContent value="Pillar"><PillarPanel /></TabsContent>
        <TabsContent value="Staking"><StakingPanel /></TabsContent>
        <TabsContent value="Sentinels"><SentinelsPanel /></TabsContent>
        <TabsContent value="Accelerator"><AcceleratorPanel /></TabsContent>
      </Tabs>
    </div>

    <TxHistory />
  </div>

  <SendModal v-model:open="sendOpen" />
  <ReceiveModal v-model:open="receiveOpen" />
  <!-- Global confirm for panel-triggered tx. Gated so it never collides with the
       Send/Receive dialogs, which render their own TxModal. -->
  <NomConfirm v-if="!sendOpen && !receiveOpen" />
</template>
