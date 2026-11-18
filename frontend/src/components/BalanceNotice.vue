<script setup lang="ts">
import { ref } from 'vue'
import { Button } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { useNodeStore } from '../stores/node'

const balances = useBalancesStore()
const node = useNodeStore()
const retrying = ref(false)
async function retry() {
  retrying.value = true
  try {
    if (!node.connected) await node.connect()
    if (node.connected) await balances.load()
  } finally { retrying.value = false }
}
</script>

<template>
  <div v-if="!balances.available" class="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/50 p-3 text-sm text-muted-foreground" role="status">
    <span v-if="!node.connected">Disconnected. Connect to a node to view your balances.</span>
    <span v-else-if="balances.status === 'error'">Balances could not be loaded. Please try again.</span>
    <span v-else>Loading balances…</span>
    <Button v-if="!node.connected || balances.status === 'error'" variant="outline" size="sm" :disabled="retrying" @click="retry">
      {{ retrying ? 'Retrying…' : !node.connected ? 'Reconnect' : 'Retry' }}
    </Button>
  </div>
</template>
