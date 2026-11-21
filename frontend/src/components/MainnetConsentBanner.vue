<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { LockIcon } from '@lucide/vue'
import { useConsentStore } from '../stores/consent'

// Shown on every screen while the wallet is on mainnet with mainnet
// transactions off. The backend refuses such a transaction as soon as it is
// prepared, before any review can appear, so without this the user only ever
// sees "mainnet sending is disabled" with no way to act on it. This explains
// the state up front and leads to the setting; Settings offers the way back.
const consent = useConsentStore()
const route = useRoute()
const router = useRouter()

function review() {
  consent.leaveFrom(route.fullPath)
  router.push({ path: '/settings', query: { section: 'mainnet' } })
}
</script>

<template>
  <div v-if="consent.blocked && route.name !== 'settings'"
    class="mb-4 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border border-warning/40 bg-warning/[0.08] px-4 py-3 text-sm"
    role="status" data-testid="mainnet-consent-banner">
    <LockIcon :size="16" class="flex-none text-warning" aria-hidden="true" />
    <p class="min-w-0 flex-1 basis-72 text-foreground">
      <span class="font-semibold">Mainnet transactions are off.</span>
      You can view balances, but nothing can be sent, received or signed with real funds until you allow it.
    </p>
    <button type="button"
      class="inline-flex h-8.5 items-center rounded-lg border border-input bg-background px-3 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]"
      @click="review">Review setting</button>
  </div>
</template>
