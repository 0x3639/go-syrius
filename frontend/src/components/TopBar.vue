<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'nom-ui'
import {
  WalletIcon, SunIcon, MoonIcon, ZapIcon, ArrowDownCircleIcon, RocketIcon, LockIcon,
  Building2Icon,
} from '@lucide/vue'
import { useWalletStore } from '../stores/wallet'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'
import { useAcceleratorStore } from '../stores/accelerator'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { useUnreceivedStore } from '../stores/unreceived'
import { useUiStore } from '../stores/ui'
import { plasmaLevel, plasmaColorClass } from '../lib/plasma'
import AccountSlotPicker from './AccountSlotPicker.vue'

defineProps<{ title?: string; locked?: boolean }>()

const router = useRouter()
const wallet = useWalletStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()
const accelerator = useAcceleratorStore()
const autoReceive = useAutoReceiveStore()
const unreceived = useUnreceivedStore()
const ui = useUiStore()

const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaColor = computed(() => plasmaColorClass(plasmaLvl.value))

function gotoVotes() {
  router.push({ path: '/network/accelerator', query: { sub: 'Vote' } })
}

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { /* no Toaster in tests */ }
watch(
  () => autoReceive.errorCount,
  () => { if (autoReceive.lastError) toast?.show(autoReceive.lastError, 'error') },
)
</script>

<template>
  <header class="flex h-15 flex-none items-center gap-4 border-b border-border px-7">
    <h1 class="text-lg font-semibold tracking-tight text-foreground">{{ title }}</h1>

    <div class="ml-auto flex items-center gap-2">
      <!-- Account/address pill: opens the account picker dropdown -->
      <AccountSlotPicker v-if="!locked" variant="pill" />
      <div v-else class="flex h-8.5 items-center gap-2 rounded-md border border-border bg-card px-3 text-muted-foreground">
        <WalletIcon :size="15" />
        <span class="font-mono text-xs">Locked</span>
      </div>

      <button type="button" aria-label="Toggle theme"
        class="grid h-8.5 w-8.5 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="ui.toggleTheme()">
        <component :is="ui.theme === 'dark' ? SunIcon : MoonIcon" :size="16" />
      </button>

      <button type="button" :disabled="locked" aria-label="Plasma"
        :title="locked ? 'Plasma — unlock to use' : `Plasma: ${plasmaLvl}`"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${plasmaColor}`"
        @click="router.push('/network/plasma')">
        <ZapIcon :size="16" />
      </button>

      <button type="button" :disabled="locked"
        :aria-label="autoReceive.enabled ? 'Auto-receive on' : 'Auto-receive off'"
        :title="!locked && unreceived.items.length > 0
          ? `${unreceived.items.length} transaction(s) to receive`
          : autoReceive.enabled ? 'Auto-receive: on' : 'Auto-receive: off'"
        :aria-pressed="locked ? undefined : autoReceive.enabled"
        class="relative grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${autoReceive.enabled ? 'text-primary' : 'text-muted-foreground'}`"
        @click="autoReceive.toggle(wallet.activeIndex)">
        <ArrowDownCircleIcon :size="16" />
        <span v-if="!locked && unreceived.items.length > 0"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[0.625rem] font-semibold text-primary-foreground">
          {{ unreceived.items.length }}
        </span>
      </button>

      <button v-if="!locked && pillar.ownsPillar" type="button" aria-label="Your pillar"
        :title="`Operating pillar: ${pillar.myPillar?.name ?? ''}`"
        class="grid h-8.5 w-8.5 place-items-center rounded-md text-success transition-colors hover:bg-foreground/[0.06]"
        @click="router.push('/network/pillars')">
        <Building2Icon :size="16" />
      </button>

      <button v-if="!locked && pillar.ownsPillar" type="button" aria-label="Accelerator votes"
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

      <button type="button" :disabled="locked" aria-label="Lock wallet" title="Lock wallet"
        class="grid h-8.5 w-8.5 place-items-center rounded-md transition-colors"
        :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : 'text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground'"
        @click="wallet.lock()">
        <LockIcon :size="16" />
      </button>
    </div>
  </header>
</template>
