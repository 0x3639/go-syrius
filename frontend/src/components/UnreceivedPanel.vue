<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useUnreceivedStore } from '../stores/unreceived'
import { formatAmount, shortAddress } from '../lib/format'

const unreceived = useUnreceivedStore()
const { items, busy, busyAll, error } = storeToRefs(unreceived)

onMounted(() => unreceived.load())

const anyBusy = computed(
  () => busyAll.value || Object.values(busy.value).some(Boolean),
)
</script>

<template>
  <div class="rounded border border-border bg-card px-4 py-3">
    <div class="mb-2 flex items-center justify-between">
      <h2 class="text-sm text-muted-foreground">Unreceived ({{ items.length }})</h2>
      <Button
        v-if="items.length"
        variant="ghost"
        size="sm"
        :disabled="busyAll"
        @click="unreceived.receiveAll()"
      >
        {{ busyAll ? 'Receiving…' : 'Receive all' }}
      </Button>
    </div>

    <div
      v-for="u in items"
      :key="u.fromHash"
      class="flex items-center justify-between gap-2 border-b border-border/60 py-1.5 text-sm last:border-b-0"
    >
      <span class="font-mono text-muted-foreground">{{ shortAddress(u.fromAddress) }}</span>
      <span class="font-mono text-foreground">{{ formatAmount(u.amount, 8) }} {{ u.token }}</span>
      <Button
        variant="secondary"
        size="sm"
        :disabled="busy[u.fromHash] || busyAll"
        @click="unreceived.receive(u.fromHash)"
      >
        {{ busy[u.fromHash] ? 'Receiving…' : 'Receive' }}
      </Button>
    </div>

    <p v-if="error" class="mt-2 text-xs text-destructive" role="alert">{{ error }}</p>
    <p v-if="anyBusy" class="mt-2 text-xs text-muted-foreground">
      Receiving may take a few seconds (proof-of-work) when plasma is low…
    </p>
  </div>
</template>
