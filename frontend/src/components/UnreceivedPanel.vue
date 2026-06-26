<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { formatAmount, shortAddress } from '../lib/format'
import { plasmaLevel } from '../lib/plasma'

const unreceived = useUnreceivedStore()
const { items, busy, busyAll, error } = storeToRefs(unreceived)
const plasma = usePlasmaStore()

onMounted(() => unreceived.load())

const anyBusy = computed(() => busyAll.value || Object.values(busy.value).some(Boolean))
// PoW generates plasma when the account has none.
const receivingLabel = computed(() =>
  plasmaLevel(plasma.info?.currentPlasma ?? 0) === 'None' ? 'Generating Plasma…' : 'Receiving…',
)
</script>

<template>
  <div class="rounded border border-border bg-card px-4 py-3">
    <div class="mb-2 flex items-center justify-between">
      <h2 class="text-sm text-muted-foreground">Unreceived ({{ items.length }})</h2>
      <Button v-if="items.length" variant="ghost" size="sm" :disabled="busyAll" @click="unreceived.receiveAll()">
        {{ busyAll ? receivingLabel : 'Receive all' }}
      </Button>
    </div>

    <div
      v-for="u in items"
      :key="u.fromHash"
      class="flex items-center gap-3 border-b border-border/60 py-2 text-sm last:border-b-0"
    >
      <span class="flex-1 truncate font-mono text-muted-foreground">{{ shortAddress(u.fromAddress) }}</span>
      <span class="font-mono text-foreground">{{ formatAmount(u.amount, u.decimals ?? 8) }} {{ u.token }}</span>
      <!-- Fixed-width status cell so the columns don't shift as the label changes. -->
      <div class="flex w-40 flex-none justify-end">
        <span
          v-if="busy[u.fromHash]"
          class="inline-flex animate-pulse items-center rounded-full bg-info/15 px-2.5 py-1 text-xs font-medium text-info"
        >{{ receivingLabel }}</span>
        <button
          v-else
          type="button"
          aria-label="receive"
          :disabled="busyAll"
          class="inline-flex items-center rounded-full bg-primary/15 px-3 py-1 text-xs font-semibold text-primary transition-colors hover:bg-primary/25 disabled:opacity-50"
          @click="unreceived.receive(u.fromHash)"
        >Receive</button>
      </div>
    </div>

    <p v-if="error" class="mt-2 text-xs text-destructive" role="alert">{{ error }}</p>
    <p v-if="anyBusy" class="mt-2 text-xs text-muted-foreground">
      Receiving may take a few seconds (proof-of-work) when plasma is low…
    </p>
  </div>
</template>
