<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePillarStore, PILLAR_PLASMA_REQUIRED, FUSE_RECOMMENDED_QSR } from '../../stores/pillar'
import { useTxStore } from '../../stores/tx'
import { useWalletStore } from '../../stores/wallet'
import { formatAmount, toBase, isValidPillarName } from '../../lib/format'
import StepHeader from './StepHeader.vue'
import Field from '../Field.vue'

const SLOW_AFTER_POLLS = 6
const PILLAR_STEPS = [
  { n: 1, label: 'Fuse plasma' },
  { n: 2, label: 'Deposit QSR' },
  { n: 3, label: 'Configure & register' },
]

const pillarStore = usePillarStore()
const tx = useTxStore()
const wallet = useWalletStore()
const { depositedQsr, qsrCost, plasma, pendingStep, pollCount } = storeToRefs(pillarStore)
const error = ref('')

// Registration form.
const name = ref('')
const producer = ref(wallet.activeAddress())
const reward = ref(wallet.activeAddress())
const momentumPct = ref('100')
const delegatePct = ref('100')
const nameAvailable = ref<boolean | null>(null)

const plasmaCurrent = computed(() => {
  try {
    return BigInt(plasma.value?.currentPlasma ?? 0)
  } catch {
    return 0n
  }
})
const plasmaCleared = computed(() => plasmaCurrent.value >= PILLAR_PLASMA_REQUIRED)
const deposited = computed(() => {
  try {
    return BigInt(depositedQsr.value || '0')
  } catch {
    return 0n
  }
})
const cost = computed(() => {
  try {
    return BigInt(qsrCost.value || '0')
  } catch {
    return 0n
  }
})
const shortfall = computed(() => (cost.value > deposited.value ? cost.value - deposited.value : 0n))
const qsrCleared = computed(() => cost.value > 0n && deposited.value >= cost.value)
const clearing = computed(() => pendingStep.value !== null)
const slow = computed(() => pendingStep.value !== null && pollCount.value >= SLOW_AFTER_POLLS)
const currentStep = computed<1 | 2 | 3>(() => (!plasmaCleared.value ? 1 : !qsrCleared.value ? 2 : 3))

const nameValid = computed(() => isValidPillarName(name.value.trim()))
const pctValid = computed(() => {
  if (momentumPct.value.trim() === '' || delegatePct.value.trim() === '') return false
  const m = Number(momentumPct.value)
  const d = Number(delegatePct.value)
  return Number.isInteger(m) && m >= 0 && m <= 100 && Number.isInteger(d) && d >= 0 && d <= 100
})
const canRegister = computed(
  () =>
    nameValid.value &&
    nameAvailable.value !== false &&
    producer.value.trim() !== '' &&
    reward.value.trim() !== '' &&
    pctValid.value,
)

// Check availability when the name becomes valid (best-effort; backend is final).
watch(name, async (n) => {
  nameAvailable.value = null
  if (!isValidPillarName(n.trim())) return
  try {
    nameAvailable.value = await Nom.CheckPillarName(n.trim())
  } catch {
    nameAvailable.value = null
  }
})

// Remember which action we initiated so the tx-done watcher can begin polling.
let lastAction: 'plasma' | 'deposit' | 'register' | null = null

async function fuse() {
  error.value = ''
  lastAction = 'plasma'
  try {
    tx.awaitConfirm(await Nom.PrepareFuse(wallet.activeAddress(), toBase(FUSE_RECOMMENDED_QSR, 8)))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function deposit() {
  error.value = ''
  lastAction = 'deposit'
  try {
    tx.awaitConfirm(await Nom.PreparePillarDepositQsr(shortfall.value.toString()))
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function withdraw() {
  // Recovers the escrowed QSR — no clearing wait, just refresh.
  error.value = ''
  lastAction = null
  try {
    tx.awaitConfirm(await Nom.PreparePillarWithdrawQsr())
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
async function register() {
  error.value = ''
  lastAction = 'register'
  try {
    tx.awaitConfirm(
      await Nom.PrepareRegisterPillar(
        name.value.trim(),
        producer.value.trim(),
        reward.value.trim(),
        Number(momentumPct.value),
        Number(delegatePct.value),
      ),
    )
  } catch (e: unknown) {
    lastAction = null
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// When a step publishes, poll for it to settle on-chain, then advance.
watch(
  () => tx.status,
  (s) => {
    if (s === 'idle' || s === 'error') {
      lastAction = null
      return
    }
    if (s !== 'done') return
    if (lastAction === 'plasma' || lastAction === 'deposit' || lastAction === 'register') {
      pillarStore.beginPending(lastAction)
    } else {
      pillarStore.refreshRegistration()
    }
    lastAction = null
  },
)
</script>

<template>
  <section class="space-y-4 rounded-lg border border-border bg-card p-4">
    <StepHeader :steps="PILLAR_STEPS" :current="currentStep" ariaLabel="Pillar registration progress" />

    <!-- Clearing (transient): waiting for the contract / fusion to settle. -->
    <div v-if="clearing" class="space-y-2">
      <div class="flex items-center gap-2 text-sm font-medium text-info">
        <svg class="animate-spin" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        <span>{{
          pendingStep === 'plasma'
            ? 'Fusing plasma — waiting for it to land on-chain…'
            : pendingStep === 'deposit'
              ? 'Your QSR deposit is on-chain. Waiting for the pillar contract to credit it…'
              : 'Registering your pillar — waiting for activation…'
        }}</span>
      </div>
      <p class="text-xs text-muted-foreground">This usually takes a few momentums.</p>
      <div v-if="slow" class="flex items-center gap-2">
        <p class="text-xs text-muted-foreground">Taking longer than usual — the network may be busy.</p>
        <button
          type="button"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="pillarStore.refreshRegistration()"
        >
          Refresh
        </button>
        <button
          type="button"
          aria-label="stop waiting"
          class="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          @click="pillarStore.stopPolling()"
        >
          Stop waiting
        </button>
      </div>
    </div>

    <!-- Step 1: ensure enough fused plasma. -->
    <template v-else-if="!plasmaCleared">
      <p class="text-xs text-muted-foreground">
        Registering a pillar needs fused plasma. We recommend fusing 500 QSR (you can cancel the
        fusion later from the Plasma tab to reclaim it).
      </p>
      <p class="text-sm text-muted-foreground">
        Current plasma <span class="font-mono text-foreground">{{ plasma?.currentPlasma ?? 0 }}</span>
      </p>
      <Button class="w-full" aria-label="fuse plasma" @click="fuse">Fuse 500 QSR for plasma</Button>
    </template>

    <!-- Step 2: deposit the (dynamic) QSR registration cost. -->
    <template v-else-if="!qsrCleared">
      <p class="rounded border border-destructive/40 bg-destructive/10 p-2 text-xs text-destructive">
        ⚠ Deposited QSR is <strong>burned and unrecoverable</strong> once the pillar is registered.
        You can withdraw it before registering if you change your mind.
      </p>
      <p class="text-sm text-muted-foreground">
        Deposited
        <span class="font-mono text-foreground"
          >{{ formatAmount(depositedQsr, 8) }} / {{ formatAmount(qsrCost, 8) }} QSR</span
        >
      </p>
      <Button class="w-full" :disabled="shortfall === 0n" aria-label="deposit pillar qsr" @click="deposit"
        >Deposit {{ formatAmount(shortfall.toString(), 8) }} QSR</Button
      >
      <Button variant="outline" class="w-full" aria-label="withdraw pillar qsr" @click="withdraw"
        >Changed your mind? Withdraw deposited QSR</Button
      >
    </template>

    <!-- Step 3: configure + register (sends the 15,000 ZNN collateral). -->
    <template v-else>
      <p class="text-sm text-foreground">✓ QSR cleared. Configure and register your pillar.</p>
      <Field
        label="Pillar name"
        :error="name.length > 0 && !nameValid ? 'Letters, digits, and single - . _ between them (max 40).' : ''"
        :hint="nameValid && nameAvailable === false ? 'Name is already taken.' : nameValid && nameAvailable ? 'Available.' : 'Choose a unique name.'"
      >
        <Input v-model="name" placeholder="my-pillar" aria-label="pillar name" />
      </Field>
      <Field label="Producer address" hint="Your pillar node's block-producing address.">
        <Input v-model="producer" placeholder="z1…" aria-label="producer address" />
      </Field>
      <Field label="Reward address" hint="Where pillar rewards are collected.">
        <Input v-model="reward" placeholder="z1…" aria-label="reward address" />
      </Field>
      <Field label="Momentum reward % (to delegators)">
        <Input v-model="momentumPct" placeholder="0–100" aria-label="momentum percent" />
      </Field>
      <Field label="Delegate reward % (to delegators)">
        <Input v-model="delegatePct" placeholder="0–100" aria-label="delegate percent" />
      </Field>
      <Button class="w-full" :disabled="!canRegister" aria-label="register pillar" @click="register"
        >Deposit 15,000 ZNN &amp; Register Pillar</Button
      >
    </template>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>
</template>
