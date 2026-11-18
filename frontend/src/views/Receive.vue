<script setup lang="ts">
import { computed } from 'vue'
import { useWalletStore } from '../stores/wallet'
import AddressDisplay from '../components/AddressDisplay.vue'
import UnreceivedPanel from '../components/UnreceivedPanel.vue'

const wallet = useWalletStore()
const address = computed(() => wallet.activeAddress())
</script>

<template>
  <div class="page-content">
    <div class="panel-card">
      <AddressDisplay :address="address" />
    </div>
    <div class="panel-card">
      <UnreceivedPanel />
    </div>
  </div>
</template>
