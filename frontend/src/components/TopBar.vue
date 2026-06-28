<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import { usePlasmaStore } from '../stores/plasma'
import { useAutoReceiveStore } from '../stores/autoReceive'
import { plasmaLevel, plasmaColorClass } from '../lib/plasma'
import AccountSlotPicker from './AccountSlotPicker.vue'

// Shared top navigation, rendered on every in-app screen so sub-pages (Address
// book, Settings, …) keep the wallet context instead of feeling like modals.
// `locked` renders a stripped variant for the Unlock screen: the same chrome,
// but with the wallet-context controls (account picker, plasma, auto-receive,
// lock) removed — they're meaningless while locked, and Settings stays gated
// by the router, so its gear is shown inert rather than as a dead link.
defineProps<{ locked?: boolean }>()

const router = useRouter()
const wallet = useWalletStore()
const plasma = usePlasmaStore()
const autoReceive = useAutoReceiveStore()

const plasmaLvl = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
const plasmaColor = computed(() => plasmaColorClass(plasmaLvl.value))

// Surface background auto-receive failures (no Confirm dialog drives them) as a
// toast. useToast may be unavailable (e.g. no Toaster mounted in tests); guard.
let toast: ReturnType<typeof useToast> | undefined
try {
  toast = useToast()
} catch {
  /* no Toaster (tests/offline) */
}
watch(
  () => autoReceive.errorCount,
  () => {
    if (autoReceive.lastError) toast?.show(autoReceive.lastError, 'error')
  },
)
</script>

<template>
  <header class="w-full border-b border-border bg-card px-6 py-3">
    <div class="flex items-center justify-between">
      <AccountSlotPicker v-if="!locked" />
      <div v-else class="flex items-center gap-2 text-muted-foreground">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
        <span class="text-sm font-medium">Locked</span>
      </div>
      <!-- One icon row for both states. While locked every control is disabled
           (a disabled <button> won't fire its @click, so the handlers stay
           inert) and rendered muted — the full bar shows, but nothing acts
           until unlock, matching syrius. -->
      <div class="flex items-center gap-1">
        <button
          type="button"
          :disabled="locked"
          :title="locked ? 'Plasma — unlock to use' : `Plasma: ${plasmaLvl}`"
          :aria-label="locked ? 'Plasma' : `Plasma: ${plasmaLvl}`"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors"
          :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${plasmaColor}`"
          @click="router.push('/home')"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/></svg>
        </button>
        <button
          type="button"
          :disabled="locked"
          :title="locked ? 'Auto-receive — unlock to use' : autoReceive.enabled ? 'Auto-receive: on' : 'Auto-receive: off'"
          :aria-label="locked ? 'Auto-receive' : autoReceive.enabled ? 'Auto-receive on' : 'Auto-receive off'"
          :aria-pressed="locked ? undefined : autoReceive.enabled"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors"
          :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : `hover:bg-foreground/[0.06] ${autoReceive.enabled ? 'text-primary' : 'text-muted-foreground'}`"
          @click="autoReceive.toggle(wallet.activeIndex)"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 8v8M8 12l4 4 4-4"/></svg>
        </button>
        <span class="mx-1 h-5 w-px bg-border"></span>
        <button
          type="button"
          :disabled="locked"
          :title="locked ? 'Wallet locked' : 'Lock wallet'"
          aria-label="Lock wallet"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors"
          :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : 'text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground'"
          @click="wallet.lock()"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/></svg>
        </button>
        <button
          type="button"
          :disabled="locked"
          :title="locked ? 'Address book — unlock to use' : 'Address book'"
          aria-label="Address book"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors"
          :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : 'text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground'"
          @click="router.push('/address-book')"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>
        </button>
        <button
          type="button"
          :disabled="locked"
          :title="locked ? 'Settings — unlock to use' : 'Settings'"
          aria-label="Settings"
          class="grid h-9 w-9 place-items-center rounded-lg transition-colors"
          :class="locked ? 'cursor-not-allowed text-muted-foreground/40' : 'text-muted-foreground hover:bg-foreground/[0.06] hover:text-foreground'"
          @click="router.push('/settings')"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
        </button>
      </div>
    </div>
  </header>
</template>
