<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useSentinelStore, QSR_REQUIRED } from '../../stores/sentinel'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import StepHeader from './StepHeader.vue'
import { CheckIcon, LoaderCircleIcon } from '@lucide/vue'
import { useBalancesStore } from '../../stores/balances'

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
// Progress + a heads-up when the account cannot cover the QSR shortfall. This
// is guidance only: the button stays enabled and the backend decides.
const balances = useBalancesStore()
const depositPercent = computed(() => Number((deposited.value * 100n) / QSR_REQUIRED))
const heldQsr = computed(() => {
  try { return BigInt(balances.items.find((b) => b.symbol === 'QSR')?.amount ?? '0') } catch { return 0n }
})
const missingQsr = computed(() => (balances.available && shortfall.value > heldQsr.value ? shortfall.value - heldQsr.value : 0n))
const currentStep = computed<1 | 2 | 3>(() => (cleared.value ? 2 : 1))

// Remember which action we initiated so the tx-done watcher can begin polling.
let lastAction: 'deposit' | 'register' | null = null

async function depositQsr() {
  error.value = ''
  lastAction = 'deposit'
  try {
    await tx.prepareCall(() => Nom.PrepareDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  lastAction = 'register'
  try {
    await tx.prepareCall(() => Nom.PrepareRegisterSentinel())
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
    await tx.prepareCall(() => Nom.PrepareWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// When the user's deposit/register publishes, poll for it to settle on-chain.
watch(
  () => tx.status,
  (s) => {
    if (s === 'idle' || s === 'error') {
      lastAction = null
      return
    }
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
  <div class="page-split">
  <section class="space-y-5 panel-card">
    <h2 class="section-title">Start a Sentinel in three steps</h2>
    <StepHeader :current="currentStep" />

    <!-- Clearing (transient): waiting for the contract to credit/activate. -->
    <div v-if="clearing" class="space-y-2">
      <div class="flex items-center gap-2 text-sm font-medium text-info">
        <LoaderCircleIcon class="animate-spin" :size="16" />
        <span>{{
          pendingStep === 'deposit'
            ? 'Your QSR deposit is on-chain. Waiting for the Sentinel contract to credit it…'
            : 'Launching your Sentinel — waiting for activation…'
        }}</span>
      </div>
      <p class="hint">This usually takes a few momentums.</p>
      <div v-if="slow" class="flex flex-wrap items-center gap-2">
        <p class="hint">Taking longer than usual — the network may be busy.</p>
        <button
          type="button"
          class="rounded-lg border border-input px-3 py-1.5 text-xs font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]"
          @click="sentinelStore.refresh()"
        >
          Refresh
        </button>
        <button
          type="button"
          aria-label="stop waiting"
          class="rounded-lg border border-input px-3 py-1.5 text-xs font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]"
          @click="sentinelStore.stopPolling()"
        >
          Stop waiting
        </button>
      </div>
    </div>

    <!-- Step 1: deposit the QSR shortfall. -->
    <template v-else-if="!cleared">
      <div class="space-y-2">
        <div class="flex justify-between gap-3 text-sm">
          <span class="text-muted-foreground">Deposited</span>
          <span class="font-mono text-foreground">{{ formatAmount(depositedQsr, 8) }} / 50,000 QSR</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
          <div class="h-full rounded-full bg-primary" :style="{ width: `${depositPercent}%` }"></div>
        </div>
      </div>
      <div v-if="missingQsr > 0n" class="rounded-xl border border-warning/40 bg-warning/10 px-3.5 py-3 text-sm">
        <strong class="text-warning">This account is {{ formatAmount(missingQsr.toString(), 8) }} QSR short.</strong>
        <span class="text-muted-foreground"> Add QSR before depositing.</span>
      </div>
      <Button class="h-12 w-full text-base font-bold" aria-label="deposit qsr" @click="depositQsr"
        >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
      >
    </template>

    <!-- Step 2: register (sends 5,000 ZNN), with a withdraw escape hatch. -->
    <template v-else>
      <p class="flex items-center gap-1.5 text-sm text-foreground"><CheckIcon :size="15" class="text-success" /> 50,000 QSR cleared. Ready to launch.</p>
      <Button class="h-12 w-full text-base font-bold" aria-label="register sentinel" @click="register"
        >Deposit 5,000 ZNN &amp; Launch Sentinel</Button
      >
      <Button variant="outline" class="w-full" aria-label="withdraw qsr" @click="withdrawQsr"
        >Changed your mind? Withdraw your 50,000 QSR</Button
      >
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>

  <aside class="space-y-5">
    <div class="panel-card space-y-3 text-sm">
      <h2 class="section-title">What you need</h2>
      <div class="flex justify-between gap-3"><span class="text-muted-foreground">QSR collateral</span><span class="font-mono text-foreground">50,000</span></div>
      <div class="flex justify-between gap-3"><span class="text-muted-foreground">ZNN collateral</span><span class="font-mono text-foreground">5,000</span></div>
      <div class="flex justify-between gap-3 border-t border-border pt-3"><span class="text-muted-foreground">Collateral returned</span><span class="font-semibold text-success">On revocation</span></div>
    </div>
    <p class="panel-card hint"><strong class="text-foreground">Good to know.</strong> A Sentinel also needs a node that stays online. This screen only handles the on-chain deposit and registration.</p>
  </aside>
  </div>
</template>
