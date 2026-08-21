<script setup lang="ts">
import { computed } from 'vue'
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

// ZNN and QSR have protocol-fixed decimals (8). Any other token's decimals are
// metadata reported by the connected node, which the wallet cannot verify: a
// malicious node could understate/overstate them so the human rendering looks
// small while the signed base-unit amount is large. For those tokens the exact
// base-unit integer — the amount the held block actually transfers — is shown
// alongside, and the human form is labelled as node-reported.
const ZNN_ZTS = 'zts1znnxxxxxxxxxxxxx9z4ulx'
const QSR_ZTS = 'zts1qsrxxxxxxxxxxxxxmrhjll'
const isCustomToken = computed(() => !!p.value && p.value.zts !== ZNN_ZTS && p.value.zts !== QSR_ZTS)
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
    <!-- Decoded effect: every material ABI parameter of the held call, decoded
         backend-side from the exact payload bytes (confirm-what-you-sign).
         Values wrap in full — a truncated address or amount cannot be verified. -->
    <div v-if="p.effect" class="space-y-1 rounded border border-border/60 p-3" data-testid="tx-effect">
      <p class="text-xs font-medium text-muted-foreground">
        Decoded effect — {{ p.effect.contract }}.{{ p.effect.method }}
      </p>
      <div v-for="f in p.effect.fields" :key="f.label" class="flex justify-between gap-4 text-sm">
        <span class="shrink-0 text-muted-foreground">{{ f.label }}</span>
        <span class="break-all whitespace-pre-wrap text-right font-mono">{{ f.value }}</span>
      </div>
    </div>
    <div v-if="p.fromAddress" class="flex justify-between gap-4">
      <span class="shrink-0 text-muted-foreground">From</span>
      <!-- The source account matters: an account switch invalidates the hold,
           and the user must see WHICH account signs (confirm-what-you-sign). -->
      <span class="break-all text-right font-mono">{{ p.fromAddress }}</span>
    </div>
    <div class="flex justify-between gap-4">
      <span class="shrink-0 text-muted-foreground">To</span>
      <!-- Full address (wraps) — confirm-what-you-sign means the user verifies
           the exact recipient, not a truncation. -->
      <span class="break-all text-right font-mono">{{ p.toAddress }}</span>
    </div>
    <div
      v-if="p.effect?.contract === 'Stake' && p.effect?.method === 'CollectReward'"
      class="flex justify-between gap-4"
      data-testid="staking-reward-asset"
    >
      <span class="text-muted-foreground">Reward asset</span>
      <span class="font-mono">QSR only</span>
    </div>
    <div class="flex justify-between gap-4">
      <span class="text-muted-foreground">{{ p.summary ? 'Contract call value' : 'Amount' }}</span>
      <span class="text-right font-mono">
        {{ formatAmountExact(p.amount, p.decimals ?? 8) }} {{ p.symbol || p.zts }}
        <span v-if="p.summary && p.amount === '0'" class="font-sans text-muted-foreground">
          (no tokens sent)
        </span>
      </span>
    </div>
    <div v-if="isCustomToken" class="space-y-1 rounded border border-border/60 p-2 text-sm" data-testid="exact-base-units">
      <div class="flex justify-between gap-4">
        <span class="shrink-0 text-muted-foreground">Exact amount (base units)</span>
        <span class="break-all text-right font-mono">{{ p.amount }}</span>
      </div>
      <div class="flex justify-between gap-4">
        <span class="shrink-0 text-muted-foreground">Token (ZTS)</span>
        <span class="break-all text-right font-mono">{{ p.zts }}</span>
      </div>
      <p class="text-xs text-muted-foreground">
        The amount above is formatted with node-reported decimals ({{ p.decimals ?? 8 }}) — verify the base-unit value.
      </p>
    </div>
    <div class="flex justify-between">
      <span class="text-muted-foreground">Fee</span>
      <span>{{ p.needsPoW ? 'PoW — plasma generated on confirm' : 'Feeless (plasma)' }}</span>
    </div>

    <!-- After Confirm, PoW (the slow part) runs here. -->
    <div v-if="status === 'publishing'" class="flex items-center gap-2 pt-2 text-sm font-medium text-info">
      <LoaderCircleIcon class="animate-spin" :size="16" />
      <!-- tx:pow-progress drives the transition: "Generating" until the PoW
           callback reports Done, then the block is being published. -->
      <span class="animate-pulse">{{ p.needsPoW && tx.powState !== 'Done' ? 'Generating Plasma…' : 'Publishing…' }}</span>
    </div>
    <div v-else class="flex gap-2 pt-2">
      <Button class="flex-1" @click="tx.confirm()">Confirm</Button>
      <Button class="flex-1" variant="outline" @click="tx.discard()">Cancel</Button>
    </div>
  </div>
</template>
