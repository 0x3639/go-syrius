<script setup lang="ts">
import { computed } from 'vue'
import { Button, Dialog, DialogContent, DialogHeader, DialogTitle } from 'nom-ui'
import { LoaderCircleIcon } from '@lucide/vue'
import { formatAmountExact } from '../lib/format'
import { useWalletConnectStore } from '../stores/walletconnect'

const wc = useWalletConnectStore()
const open = computed({
  get: () => wc.request !== null,
  set: (value: boolean) => {
    // A generic dismissal (Escape, backdrop, X) is NON-destructive: it only
    // hides the modal. It must never acknowledge/delete a recovered record's
    // durable duplicate guard — only the labeled "Acknowledge and clear" button
    // does that.
    if (!value && wc.request?.status === 'awaiting') void wc.rejectRequest()
    else if (!value && (wc.request?.status === 'delivery-error' || wc.request?.status === 'unknown' || wc.request?.status === 'recovered')) wc.clearPublishedRequest()
  },
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="w-[42rem] max-w-[95vw]">
      <DialogHeader><DialogTitle>WalletConnect request</DialogTitle></DialogHeader>
      <div v-if="wc.request" class="space-y-3 rounded border border-primary/40 bg-card p-4">
        <p class="text-sm text-primary">{{ wc.request.dapp }} requests {{ wc.request.preview.summary }}</p>
        <p v-if="wc.request.validation === 'VALID'" class="text-xs text-success">
          Verified origin: <span class="break-all font-mono">{{ wc.request.verifiedOrigin }}</span>
        </p>
        <p v-else class="text-xs text-warning" role="alert">
          Dapp origin not verified by WalletConnect{{ wc.request.verifiedOrigin ? ':' : '' }}
          <span v-if="wc.request.verifiedOrigin" class="break-all font-mono">{{ wc.request.verifiedOrigin }}</span>
          — the name above is claimed by the dapp, not proven.
        </p>
        <p class="text-xs text-muted-foreground">Verify the exact Go-decoded effect before approving.</p>
        <div v-if="wc.request.preview.effect" class="space-y-1 rounded border border-border/60 p-3">
          <p class="text-xs font-medium text-muted-foreground">
            {{ wc.request.preview.effect.contract }}.{{ wc.request.preview.effect.method }}
          </p>
          <div v-for="field in wc.request.preview.effect.fields" :key="field.label" class="flex justify-between gap-4 text-sm">
            <span class="shrink-0 text-muted-foreground">{{ field.label }}</span>
            <span class="break-all whitespace-pre-wrap text-right font-mono">{{ field.value }}</span>
          </div>
        </div>
        <div class="flex justify-between gap-4 text-sm"><span class="text-muted-foreground">From</span><span class="break-all text-right font-mono">{{ wc.request.preview.fromAddress }}</span></div>
        <div class="flex justify-between gap-4 text-sm"><span class="text-muted-foreground">To</span><span class="break-all text-right font-mono">{{ wc.request.preview.toAddress }}</span></div>
        <div class="flex justify-between gap-4 text-sm">
          <span class="text-muted-foreground">Contract call value</span>
          <span class="font-mono">{{ formatAmountExact(wc.request.preview.amount, wc.request.preview.decimals ?? 8) }} {{ wc.request.preview.symbol || wc.request.preview.zts }}</span>
        </div>
        <!-- The human rendering above depends on node-reported token decimals;
             the base-unit integer is the held block's authoritative amount. -->
        <div class="flex justify-between gap-4 text-sm">
          <span class="shrink-0 text-muted-foreground">Exact amount (base units)</span>
          <span class="break-all text-right font-mono">{{ wc.request.preview.amount }} {{ wc.request.preview.zts }}</span>
        </div>
        <div class="flex justify-between gap-4 text-sm"><span class="text-muted-foreground">Fee</span><span>{{ wc.request.preview.needsPoW ? 'PoW — plasma generated on confirm' : 'Feeless (plasma)' }}</span></div>

        <div v-if="wc.request.status === 'publishing'" class="flex items-center gap-2 text-info" aria-live="polite">
          <LoaderCircleIcon class="animate-spin" :size="16" /><span>Generating plasma and publishing…</span>
        </div>
        <div v-else-if="wc.request.status === 'notifying'" class="flex items-center gap-2 text-info" aria-live="polite">
          <LoaderCircleIcon class="animate-spin" :size="16" /><span>Published. Notifying the dapp…</span>
        </div>
        <div v-else-if="wc.request.status === 'delivery-error'" class="space-y-2">
          <p class="text-sm text-destructive" role="alert">{{ wc.request.error }}</p>
          <p v-if="wc.request.publishedHash" class="break-all text-xs font-mono text-muted-foreground">
            Published transaction: {{ wc.request.publishedHash }}
          </p>
          <div class="flex gap-2">
            <Button
              v-if="!wc.request.sessionEnded"
              class="flex-1"
              @click="wc.retryPublishedResponse()"
            >Retry dapp notification</Button>
            <Button class="flex-1" variant="outline" @click="wc.clearPublishedRequest()">Close locally</Button>
          </div>
        </div>
        <div v-else-if="wc.request.status === 'unknown'" class="space-y-2">
          <p class="text-sm text-warning" role="alert">
            The broadcast outcome of this transaction is unknown — it may already be on chain.
            Never resubmit it from the dapp.
          </p>
          <p v-if="wc.request.publishedHash" class="break-all text-xs font-mono text-muted-foreground">
            Signed block: {{ wc.request.publishedHash }}
          </p>
          <p v-if="wc.request.error" class="text-xs text-muted-foreground">{{ wc.request.error }}</p>
          <div class="flex gap-2">
            <Button class="flex-1" @click="wc.reconcileRequest()">Check outcome</Button>
            <Button class="flex-1" variant="outline" @click="wc.clearPublishedRequest()">Close locally</Button>
          </div>
        </div>
        <div v-else-if="wc.request.status === 'recovered'" class="space-y-2">
          <p class="text-sm text-success">
            The outcome of this transfer from another WalletConnect session has been resolved.
          </p>
          <p v-if="wc.request.publishedHash" class="break-all text-xs font-mono text-muted-foreground">
            Transaction: {{ wc.request.publishedHash }}
          </p>
          <p v-if="wc.request.error" class="text-sm text-destructive" role="alert">{{ wc.request.error }}</p>
          <Button class="w-full" @click="wc.acknowledgeRecovered()">Acknowledge and clear</Button>
        </div>
        <div v-else-if="wc.request.status === 'error'" class="space-y-2">
          <p class="text-sm text-destructive" role="alert">{{ wc.request.error }}</p>
          <Button class="w-full" @click="wc.clearRequestError()">Close and reject request</Button>
        </div>
        <div v-else-if="wc.request.status === 'awaiting'" class="flex gap-2">
          <Button class="flex-1" @click="wc.approveRequest()">Approve and publish</Button>
          <Button class="flex-1" variant="outline" @click="wc.rejectRequest()">Reject</Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
