<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useUnreceivedStore } from '../stores/unreceived'
import { usePlasmaStore } from '../stores/plasma'
import { formatAmount } from '../lib/format'
import { plasmaLevel } from '../lib/plasma'
import MonoTruncate from './MonoTruncate.vue'
import { ArrowDownIcon } from '@lucide/vue'

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
  <div>
    <div class="flex items-center justify-between gap-3 px-6 py-4">
      <h2 class="section-title">Waiting to be received <span class="font-normal text-muted-foreground">· {{ items.length }}</span></h2>
      <Button v-if="items.length" size="sm" :disabled="busyAll" @click="unreceived.receiveAll()">
        {{ busyAll ? receivingLabel : 'Receive all' }}
      </Button>
    </div>

    <p v-if="!items.length" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">Nothing is waiting. Incoming payments show up here.</p>

    <div v-for="u in items" :key="u.fromHash" class="list-row text-sm">
      <span class="avatar-dot" aria-hidden="true"><ArrowDownIcon :size="16" /></span>
      <!-- Address takes the full remaining width (truncates only if it must); the
           amount sits with the Receive action on the right. -->
      <MonoTruncate :value="u.fromAddress" class="min-w-0 flex-1 text-muted-foreground" />
      <div class="flex flex-none items-center gap-3">
        <span class="whitespace-nowrap font-mono text-primary">{{ formatAmount(u.amount, u.decimals ?? 8) }} {{ u.token }}</span>
        <span
          v-if="busy[u.fromHash]"
          class="inline-flex animate-pulse items-center rounded-full bg-info/15 px-2.5 py-1 text-xs font-medium text-info"
        >{{ receivingLabel }}</span>
        <button
          v-else
          type="button"
          aria-label="receive"
          :disabled="busyAll"
          class="inline-flex h-9 items-center rounded-lg border border-input px-3.5 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06] disabled:opacity-50"
          @click="unreceived.receive(u.fromHash)"
        >Receive</button>
      </div>
    </div>

    <p v-if="error" class="px-6 pb-4 text-xs text-destructive" role="alert">{{ error }}</p>
    <p v-if="anyBusy" class="hint px-6 pb-4">
      Receiving may take a few seconds (proof-of-work) when plasma is low…
    </p>
  </div>
</template>
