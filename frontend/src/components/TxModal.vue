<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import { formatAmountExact } from '../lib/format'
import { LoaderCircleIcon } from '@lucide/vue'

// FUNDS-CRITICAL — confirm-what-you-approve. Every field rendered here derives
// from `tx.preview` — the EFFECT of the held, un-PoW'd template from PrepareSend,
// NEVER from the form inputs. PoW (hashing) and signing happen on Confirm
// (ConfirmPublish); what the user approves here is the exact effect: recipient,
// amount, token. The amount uses formatAmountExact so the approved value is
// exact. Do not introduce any path where the confirmed amount differs from
// preview.amount.
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
      Confirm — you are approving this exact transaction (hashed &amp; signed on confirm)
    </h2>
    <p v-if="p.summary" class="text-sm text-primary">{{ p.summary }}</p>
    <div class="flex justify-between gap-4">
      <span class="shrink-0 text-muted-foreground">To</span>
      <!-- Full address (wraps) — confirm-what-you-sign means the user verifies
           the exact recipient, not a truncation. -->
      <span class="break-all text-right font-mono">{{ p.toAddress }}</span>
    </div>
    <div class="flex justify-between">
      <span class="text-muted-foreground">Amount</span>
      <span class="font-mono"
        >{{ formatAmountExact(p.amount, p.decimals ?? 8) }} {{ p.symbol || p.zts }}</span
      >
    </div>
    <div class="flex justify-between">
      <span class="text-muted-foreground">Fee</span>
      <span>{{ p.needsPoW ? 'PoW — plasma generated on confirm' : 'Feeless (plasma)' }}</span>
    </div>

    <!-- After Confirm, PoW (the slow part) runs here. -->
    <div v-if="status === 'publishing'" class="flex items-center gap-2 pt-2 text-sm font-medium text-info">
      <LoaderCircleIcon class="animate-spin" :size="16" />
      <span class="animate-pulse">{{ p.needsPoW ? 'Generating Plasma…' : 'Publishing…' }}</span>
    </div>
    <div v-else class="flex gap-2 pt-2">
      <Button class="flex-1" @click="tx.confirm()">Confirm</Button>
      <Button class="flex-1" variant="outline" @click="tx.discard()">Cancel</Button>
    </div>
  </div>
</template>
