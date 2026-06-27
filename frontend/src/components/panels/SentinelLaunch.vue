<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore, QSR_REQUIRED } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import StepHeader from './StepHeader.vue'

const SLOW_AFTER_POLLS = 6

const sentinelStore = useSentinelStore()
const tx = useTxStore()
const { depositedQsr, pendingStep, pollCount } = storeToRefs(sentinelStore)
const error = ref('')

const deposited = computed(() => {
  try {
    return BigInt(depositedQsr.value || '0')
  } catch {
    return 0n
  }
})
const shortfall = computed(() => (QSR_REQUIRED > deposited.value ? QSR_REQUIRED - deposited.value : 0n))
const cleared = computed(() => deposited.value >= QSR_REQUIRED)
const clearing = computed(() => pendingStep.value !== null)
const slow = computed(() => pendingStep.value !== null && pollCount.value >= SLOW_AFTER_POLLS)
const currentStep = computed<1 | 2 | 3>(() => (cleared.value ? 2 : 1))

// Remember which action we initiated so the tx-done watcher can begin polling.
let lastAction: 'deposit' | 'register' | null = null

async function depositQsr() {
  error.value = ''
  lastAction = 'deposit'
  try {
    tx.awaitConfirm(await Nom.PrepareDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  lastAction = 'register'
  try {
    tx.awaitConfirm(await Nom.PrepareRegisterSentinel())
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function withdrawQsr() {
  // Withdraw returns to step 1 — no clearing wait, just refresh.
  error.value = ''
  lastAction = null
  try {
    tx.awaitConfirm(await Nom.PrepareWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// When the user's deposit/register publishes, poll for it to settle on-chain.
watch(
  () => tx.status,
  (s) => {
    if (s !== 'done') return
    if (lastAction === 'deposit' || lastAction === 'register') {
      sentinelStore.beginPending(lastAction)
    } else {
      sentinelStore.refresh()
    }
    lastAction = null
  },
)
</script>

<template>
  <section class="space-y-4 rounded-lg border border-border bg-card p-4">
    <StepHeader :current="currentStep" />

    <!-- Clearing (transient): waiting for the contract to credit/activate. -->
    <div v-if="clearing" class="space-y-2">
      <div class="flex items-center gap-2 text-sm font-medium text-info">
        <svg class="animate-spin" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        <span>{{
          pendingStep === 'deposit'
            ? 'Your QSR deposit is on-chain. Waiting for the Sentinel contract to credit it…'
            : 'Launching your Sentinel — waiting for activation…'
        }}</span>
      </div>
      <p class="text-xs text-muted-foreground">This usually takes a few momentums.</p>
      <div v-if="slow" class="flex items-center gap-2">
        <p class="text-xs text-muted-foreground">Taking longer than usual — the network may be busy.</p>
        <button
          type="button"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="sentinelStore.refresh()"
        >
          Refresh
        </button>
      </div>
    </div>

    <!-- Step 1: deposit the QSR shortfall. -->
    <template v-else-if="!cleared">
      <p class="text-xs text-muted-foreground">
        A Sentinel needs 50,000 QSR + 5,000 ZNN collateral (returned on revocation).
      </p>
      <p class="text-sm text-muted-foreground">
        Deposited
        <span class="font-mono text-foreground">{{ formatAmount(depositedQsr, 8) }} / 50,000 QSR</span>
      </p>
      <Button class="w-full" aria-label="deposit qsr" @click="depositQsr"
        >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
      >
    </template>

    <!-- Step 2: register (sends 5,000 ZNN), with a withdraw escape hatch. -->
    <template v-else>
      <p class="text-sm text-foreground">✓ 50,000 QSR cleared. Ready to launch.</p>
      <Button class="w-full" aria-label="register sentinel" @click="register"
        >Deposit 5,000 ZNN &amp; Launch Sentinel</Button
      >
      <Button variant="outline" class="w-full" aria-label="withdraw qsr" @click="withdrawQsr"
        >Changed your mind? Withdraw your 50,000 QSR</Button
      >
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>
</template>
