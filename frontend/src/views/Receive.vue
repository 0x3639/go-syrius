<script setup lang="ts">
import { computed } from 'vue'
import { useWalletStore } from '../stores/wallet'
import { useAutoReceiveStore } from '../stores/autoReceive'
import AddressDisplay from '../components/AddressDisplay.vue'
import UnreceivedPanel from '../components/UnreceivedPanel.vue'

const wallet = useWalletStore()
const autoReceive = useAutoReceiveStore()
const address = computed(() => wallet.activeAddress())
</script>

<template>
  <div class="page-content">
    <div class="grid grid-cols-1 items-start gap-5 @3xl:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
      <section class="panel-card space-y-4">
        <h2 class="section-title">Your address</h2>
        <AddressDisplay :address="address" />
      </section>

      <div class="min-w-0 space-y-5">
        <section class="panel-list">
          <UnreceivedPanel />
        </section>

        <section class="panel-card flex items-center gap-4">
          <div class="min-w-0 flex-1">
            <h2 class="section-title">Receive automatically</h2>
            <p class="hint mt-1">Accept incoming payments as they arrive while the wallet is unlocked. Each one uses a little plasma or a short proof-of-work.</p>
          </div>
          <button
            type="button"
            role="switch"
            :aria-checked="autoReceive.enabled"
            aria-label="Receive automatically"
            class="flex h-7.5 w-13 flex-none items-center rounded-full p-0.5 transition-colors"
            :class="autoReceive.enabled ? 'justify-end bg-primary' : 'justify-start bg-foreground/20'"
            @click="autoReceive.toggle(wallet.activeIndex)"
          >
            <span class="h-6.5 w-6.5 rounded-full bg-background"></span>
          </button>
        </section>

        <section class="panel-card hint">
          <strong class="text-foreground">Why do payments wait?</strong>
          On Zenon the receiver publishes a small block to accept each payment. Until then the funds are safely on their way and show up here.
        </section>
      </div>
    </div>
  </div>
</template>
