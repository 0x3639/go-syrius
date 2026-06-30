<script setup lang="ts">
import { computed } from 'vue'
import { useWalletStore } from '../stores/wallet'
import AddressDisplay from '../components/AddressDisplay.vue'
import UnreceivedPanel from '../components/UnreceivedPanel.vue'

const wallet = useWalletStore()
const address = computed(() => wallet.activeAddress())
</script>

<template>
  <div class="mx-auto max-w-[34rem] space-y-5">
    <div class="rounded-xl border border-border bg-card p-6">
      <AddressDisplay :address="address" />
    </div>
    <div class="rounded-xl border border-border bg-card p-6">
      <UnreceivedPanel />
    </div>
  </div>
</template>
