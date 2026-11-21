<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useBalancesStore } from '../../stores/balances'
import { useTxStore } from '../../stores/tx'
import { formatAmountExact, toBase } from '../../lib/format'
import AmountInput from '../AmountInput.vue'

const TOKENS = ['ZNN', 'QSR'] as const
// ZNN and QSR have protocol-fixed 8 decimals.
const DECIMALS = 8

const acc = useAcceleratorStore()
const balances = useBalancesStore()
const tx = useTxStore()
const amount = ref('')
const token = ref<(typeof TOKENS)[number]>('ZNN')
const error = ref('')

function held(symbol: string): string {
  return balances.items.find((b) => b.symbol === symbol)?.amount ?? '0'
}
const maxDecimal = computed(() => (balances.available ? formatAmountExact(held(token.value), DECIMALS) : ''))

// The amount is typed in whole ZNN/QSR and converted to base units here; the
// confirm dialog then shows the exact value from the block the backend built.
const baseAmount = computed(() => {
  try {
    const base = BigInt(toBase(amount.value, DECIMALS))
    return base > 0n ? base : null
  } catch { return null }
})
const overBalance = computed(() => balances.available && baseAmount.value !== null && baseAmount.value > BigInt(held(token.value)))
// Like Send, Staking and Plasma: no submit until the balance is known, so the
// amount can actually be checked against it.
const canDonate = computed(() => balances.available && baseAmount.value !== null && !overBalance.value)

async function donate() {
  if (!canDonate.value || baseAmount.value === null) return
  error.value = ''
  const base = baseAmount.value.toString()
  const symbol = token.value
  try {
    await tx.prepareCall(() => Nom.PrepareDonate(base, symbol))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.loadProjects()
  },
)
</script>

<template>
  <section class="panel-card page-narrow space-y-5">
    <div>
      <h2 class="section-title">Donate to the funding pool</h2>
      <p class="hint mt-1">Donate ZNN or QSR to the Accelerator-Z funding pool.</p>
    </div>

    <div class="space-y-2">
      <span class="field-label block">Which asset?</span>
      <div class="grid max-w-md grid-cols-2 gap-2.5" role="group" aria-label="donate token">
        <button
          v-for="t in TOKENS"
          :key="t"
          type="button"
          class="choice flex flex-col gap-0.5"
          :class="token === t ? 'choice-on' : ''"
          :aria-label="`donate ${t}`"
          :aria-pressed="token === t"
          @click="token = t"
        >
          <span class="font-semibold">{{ t }}</span>
          <span class="truncate font-mono text-xs text-muted-foreground">{{ balances.available ? formatAmountExact(held(t), DECIMALS) : '—' }}</span>
        </button>
      </div>
    </div>

    <div class="space-y-1.5">
      <AmountInput v-model="amount" :label="`Amount (${token})`" aria-label="donate amount" :symbol="token" :max="maxDecimal" />
      <p v-if="amount && baseAmount === null" class="text-xs text-destructive">Enter an amount greater than zero, with at most 8 decimal places.</p>
      <p v-else-if="overBalance" class="text-xs text-destructive">That is more than this account holds.</p>
      <p v-else-if="!balances.available" class="hint">Your balance is not available yet, so donating is paused.</p>
    </div>

    <Button class="h-12 w-full text-base font-bold" aria-label="donate" :disabled="!canDonate" @click="donate">Donate</Button>
    <p class="hint text-center">Donations go to the shared pool and cannot be taken back.</p>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </section>
</template>
