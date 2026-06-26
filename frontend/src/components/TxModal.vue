<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import { formatAmountExact, shortAddress } from '../lib/format'

// FUNDS-CRITICAL — confirm-what-you-sign. Every field rendered here derives from
// `tx.preview` (the BUILT BLOCK returned by PrepareSend), NEVER from the form
// inputs. The amount uses formatAmountExact so the user confirms the exact value
// being signed. Do not introduce any path where the confirmed amount differs
// from preview.amount.
const tx = useTxStore()
const { preview: p, status } = storeToRefs(tx)
</script>

<template>
  <div
    v-if="p"
    class="space-y-2 rounded border border-primary/40 bg-card p-4"
    role="dialog"
    aria-label="Confirm transaction"
  >
    <h2 class="text-sm text-muted-foreground">
      Confirm — you are signing this exact transaction
    </h2>
    <p v-if="p.summary" class="text-sm text-primary">{{ p.summary }}</p>
    <div class="flex justify-between">
      <span class="text-muted-foreground">To</span>
      <span class="font-mono">{{ shortAddress(p.toAddress) }}</span>
    </div>
    <div class="flex justify-between">
      <span class="text-muted-foreground">Amount</span>
      <span class="font-mono"
        >{{ formatAmountExact(p.amount, p.decimals ?? 8) }} {{ p.symbol || p.zts }}</span
      >
    </div>
    <div class="flex justify-between">
      <span class="text-muted-foreground">Fee</span>
      <span>{{
        p.needsPoW ? `PoW (difficulty ${p.difficulty})` : 'Feeless (plasma)'
      }}</span>
    </div>
    <div class="space-y-1">
      <span class="text-muted-foreground">Hash</span>
      <div class="break-all font-mono text-xs text-foreground">{{ p.hash }}</div>
    </div>
    <div class="flex gap-2 pt-2">
      <Button
        class="flex-1"
        :disabled="status === 'publishing'"
        @click="tx.confirm()"
      >
        Confirm
      </Button>
      <Button
        class="flex-1"
        variant="outline"
        :disabled="status === 'publishing'"
        @click="tx.cancel()"
      >
        Cancel
      </Button>
    </div>
  </div>
</template>
