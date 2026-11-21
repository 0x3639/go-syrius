<script setup lang="ts">
import { computed, watch } from 'vue'
import { useModeSwitch } from '../lib/useModeSwitch'
import { networkName } from '../lib/network'
import { useRouter, useRoute } from 'vue-router'
import { useToast } from 'nom-ui'
import {
  SunIcon, MoonIcon, ZapIcon, ArrowDownCircleIcon, RocketIcon, LockIcon,
  Building2Icon, LinkIcon, GlobeIcon,
} from '@lucide/vue'
import { useWalletStore } from '../stores/wallet'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import { useNodeStore } from '../stores/node'
import { useWalletConnectStore } from '../stores/walletconnect'
import { plasmaLevel, plasmaColorClass } from '../lib/plasma'

// Only rendered inside AppShell, i.e. only while unlocked — no locked variant.
defineProps<{ title?: string; subtitle?: string }>()

const router = useRouter()
const route = useRoute()
const wallet = useWalletStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const autoReceive = useAutoReceiveStore()
const unreceived = useUnreceivedStore()
const ui = useUiStore()
const node = useNodeStore()
const walletConnect = useWalletConnectStore()

const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaAvailable = computed(() => node.connected && plasma.info !== null)
const plasmaColor = computed(() => plasmaAvailable.value ? plasmaColorClass(plasmaLvl.value) : 'text-muted-foreground')

const switchMode = useModeSwitch()

// Tooltip for the incoming button. "No payments waiting" is a claim, so it is
// made only after a lookup has actually succeeded.
const incomingTitle = computed(() => {
  const n = unreceived.items.length
  if (unreceived.failed) return `Couldn't check for incoming payments — open Receive to try again`
  if (n > 0) return `${n} payment(s) waiting — open Receive`
  if (!node.connected) return 'Incoming payments: not connected'
  if (unreceived.lookup !== 'ok') return 'Checking for incoming payments…'
  return autoReceive.enabled ? 'No payments waiting · auto-receive is on' : 'No payments waiting'
})

// The chain the connected node reports. Named for the two known networks so a
// mainnet session is never mistaken for a test one.
const network = computed(() => {
  if (!node.connected || !node.chainId) return null
  return { name: networkName(node.chainId), chainId: node.chainId }
})

watch(() => ui.modeNotice, (m) => {
  if (!m) return
  toast?.show(m, 'info')
  ui.clearModeNotice()
})

function gotoVotes() {
  router.push({ path: '/network/accelerator', query: { sub: 'Vote' } })
}

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { /* no Toaster in tests */ }
watch(
  () => autoReceive.errorCount,
  () => { if (autoReceive.lastError) toast?.show(autoReceive.lastError, 'error') },
)

// The UI locks either way (the store tears down locally), but a backend Lock
// failure means the keystore is still decrypted in Go memory — surface it.
async function lockWallet() {
  try {
    await wallet.lock()
  } catch {
    toast?.show('The wallet screen is locked, but the backend reported a lock error — restart go-syrius to be safe', 'error')
  }
}
</script>

<template>
  <header class="flex flex-none flex-wrap items-center gap-x-4 gap-y-2 px-4 pb-2 pt-5 lg:px-8">
    <div class="min-w-0 flex-1 basis-64">
      <h1 class="text-2xl font-semibold tracking-tight text-foreground">
        {{ title }}<template v-if="route.name === 'net-pillars' && pillar.ownsPillar"> — <span class="text-success">{{ pillar.myPillar?.name }}</span></template>
      </h1>
      <p v-if="subtitle" class="mt-0.5 text-sm text-muted-foreground">{{ subtitle }}</p>
    </div>

    <div class="flex items-center gap-0.5 rounded-xl border border-border bg-card p-1" role="group" aria-label="Wallet mode">
      <span class="px-2 text-xs font-semibold text-muted-foreground" aria-hidden="true">Mode</span>
      <button v-for="m in (['everyday', 'advanced'] as const)" :key="m" type="button"
        :aria-pressed="ui.mode === m"
        class="h-8.5 rounded-md px-3 text-sm capitalize transition-colors"
        :class="ui.mode === m ? 'bg-foreground font-semibold text-background' : 'font-medium text-muted-foreground hover:text-foreground'"
        @click="switchMode(m)">{{ m }}</button>
    </div>

    <div class="flex flex-wrap items-center gap-1 rounded-xl border border-border bg-card p-1">
      <span v-if="network" class="flex h-8.5 items-center gap-1.5 px-2.5 text-sm font-medium text-foreground"
        :title="`Connected node chain: ${network.chainId}`" data-testid="network-chip">
        <GlobeIcon :size="15" class="text-muted-foreground" />{{ network.name }}
        <span class="font-mono text-xs text-muted-foreground">chain {{ network.chainId }}</span>
      </span>
      <span v-if="network" class="mx-1 h-5 w-px bg-border"></span>

      <button type="button" aria-label="Toggle theme"
        class="grid h-8.5 w-8.5 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="ui.toggleTheme()">
        <component :is="ui.theme === 'dark' ? SunIcon : MoonIcon" :size="16" />
      </button>

      <button type="button" aria-label="Plasma"
        :title="plasmaAvailable ? `Plasma: ${plasmaLvl}` : 'Plasma: unavailable'"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="`hover:bg-foreground/[0.06] ${plasmaColor}`"
        @click="router.push('/network/plasma')">
        <ZapIcon :size="16" />
      </button>

      <!-- (The pillar and vote shortcuts further down are operator tools:
           Advanced only. Everyday reports a waiting vote on Home instead.) -->
      <!-- Opens the waiting payments. It never changes auto-receive: that is
           an explicit switch on the Receive screen. -->
      <button type="button"
        :aria-label="unreceived.failed ? `Couldn't check for incoming payments` : unreceived.items.length > 0 ? `${unreceived.items.length} incoming payment(s) waiting` : 'Incoming payments'"
        :title="incomingTitle"
        class="relative grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="`hover:bg-foreground/[0.06] ${unreceived.failed ? 'text-warning' : unreceived.items.length > 0 ? 'text-primary' : 'text-muted-foreground'}`"
        @click="router.push('/receive')">
        <ArrowDownCircleIcon :size="16" />
        <span v-if="unreceived.failed"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-warning px-1 text-[0.625rem] font-semibold text-warning-foreground" aria-hidden="true">?</span>
        <span v-else-if="unreceived.items.length > 0"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[0.625rem] font-semibold text-primary-foreground">
          {{ unreceived.items.length }}
        </span>
      </button>

      <button type="button"
        :aria-label="walletConnect.sessions.length > 0 ? 'WalletConnect connected' : 'WalletConnect'"
        :title="walletConnect.sessions.length > 0
          ? `WalletConnect: ${walletConnect.sessions.length} connected dapp(s)`
          : 'WalletConnect: not connected'"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        :class="walletConnect.sessions.length > 0 ? 'text-success' : 'text-muted-foreground'"
        @click="router.push('/walletconnect')">
        <LinkIcon :size="16" />
      </button>

      <button v-if="pillar.ownsPillar && ui.mode === 'advanced'" type="button" aria-label="Your pillar"
        :title="`Operating pillar: ${pillar.myPillar?.name ?? ''}`"
        class="grid h-8.5 w-8.5 place-items-center rounded-md text-success transition-colors hover:bg-foreground/[0.06]"
        @click="router.push('/network/pillars')">
        <Building2Icon :size="16" />
      </button>

      <button v-if="pillar.ownsPillar && ui.mode === 'advanced'" type="button" aria-label="Accelerator votes"
        :title="accelerator.needsVoteCount > 0 ? `${accelerator.needsVoteCount} AZ item(s) to vote on` : 'Accelerator votes'"
        class="relative grid h-8.5 w-8.5 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="gotoVotes">
        <RocketIcon :size="16" />
        <span v-if="accelerator.needsVoteCount > 0"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[0.625rem] font-semibold text-primary-foreground">
          {{ accelerator.needsVoteCount }}
        </span>
      </button>

      <span class="mx-1 h-5 w-px bg-border"></span>

      <button type="button" aria-label="Lock wallet" title="Lock wallet"
        class="flex h-8.5 items-center gap-2 rounded-md px-3 text-sm font-medium text-foreground transition-colors hover:bg-foreground/[0.06]"
        @click="lockWallet">
        <LockIcon :size="15" />Lock
      </button>
    </div>
  </header>
</template>
