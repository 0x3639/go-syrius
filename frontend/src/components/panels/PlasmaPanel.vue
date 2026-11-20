<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePlasmaStore } from '../../stores/plasma'
import { useTxStore } from '../../stores/tx'
import { formatAmount, formatAmountExact, shortAddress, toBase } from '../../lib/format'
import { useBalancesStore } from '../../stores/balances'
import Field from '../Field.vue'
import AmountInput from '../AmountInput.vue'
import { useWalletStore } from '../../stores/wallet'
import { plasmaLevel } from '../../lib/plasma'

const plasma = usePlasmaStore()
const wallet = useWalletStore()
const tx = useTxStore()

const { info, fusionEntries } = storeToRefs(plasma)

const beneficiary = ref('')
const amount = ref('')
const estimate = ref(0)
const error = ref('')
const balances = useBalancesStore()
const available = computed(() => balances.items.find(b => b.symbol === 'QSR')?.amount ?? '0')
const availableLabel = computed(() => balances.available ? `${formatAmountExact(available.value, 8)} QSR available` : 'Balance unavailable')
const maxFusion = computed(() => {
  const whole = BigInt(available.value) / 100000000n
  return balances.available && whole >= 10n ? whole.toString() : ''
})
const canFuse = computed(() => {
  try {
    const base = BigInt(toBase(amount.value, 8))
    const address = beneficiary.value
    const uniformCase = address === address.toLowerCase() || address === address.toUpperCase()
    return balances.available && uniformCase && /^z1[0-9a-z]{38}$/i.test(address)
      && base >= 1000000000n && base % 100000000n === 0n && base <= BigInt(available.value)
  } catch { return false }
})

// Level meter: the same thresholds as the top-bar bolt, worded for people.
const level = computed(() => plasmaLevel(info.value?.currentPlasma ?? 0))
const levelOk = computed(() => level.value === 'High' || level.value === 'Medium')
const levelPercent = computed(() => {
  const max = info.value?.maxPlasma ?? 0
  return max > 0 ? Math.max(0, Math.min(100, ((info.value?.currentPlasma ?? 0) / max) * 100)) : 0
})
const ownAddress = computed(() => wallet.activeAddress())
const forSelf = computed(() => !!ownAddress.value && beneficiary.value === ownAddress.value)

useNodeRefresh(() => plasma.refresh())

// Live plasma estimate: mirrors the Svelte `$: if (amount) estimate(...)`. Only
// the whole-QSR part drives the estimate, matching the original.
watch(amount, async (a) => {
  if (a) {
    estimate.value = await plasma.estimate(a.split('.')[0] || '0')
  } else {
    estimate.value = 0
  }
})

async function fuse() {
  if (!canFuse.value) return
  // NoM-confirm pattern: prepare the call, hand the preview to the global
  // NomConfirm dialog via tx.awaitConfirm. The panel renders no modal itself.
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareFuse(beneficiary.value, toBase(amount.value, 8)))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function cancel(id: string) {
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareCancelFuse(id))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

// After a publish completes, refresh the plasma snapshot + fusion list.
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') plasma.refresh()
  },
)
</script>

<template>
  <div class="space-y-5">
    <section v-if="info" class="panel-card flex flex-wrap items-center gap-x-8 gap-y-4">
      <div class="min-w-0 flex-[2] basis-72 space-y-2.5">
        <div class="flex items-center gap-2.5">
          <h2 class="section-title">This account's plasma</h2>
          <span class="status-pill" :class="levelOk ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'">{{ level }}</span>
        </div>
        <div class="h-2.5 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
          <div class="h-full rounded-full" :class="levelOk ? 'bg-success' : 'bg-warning'" :style="{ width: `${levelPercent}%` }"></div>
        </div>
        <p class="hint">{{ levelOk ? 'Enough for instant transactions.' : 'Not enough for an instant send. The wallet falls back to a short proof-of-work.' }}</p>
      </div>
      <div class="flex-1 basis-36">
        <div class="hint">Current plasma</div>
        <div class="font-mono text-xl text-foreground">{{ info.currentPlasma.toLocaleString('en-US') }} <span class="text-xs text-muted-foreground">/ {{ info.maxPlasma.toLocaleString('en-US') }}</span></div>
      </div>
      <div class="flex-1 basis-36">
        <div class="hint">QSR fused</div>
        <div class="font-mono text-xl text-foreground">{{ formatAmount(info.qsrFused, 8) }} QSR</div>
      </div>
    </section>

    <div class="page-halves">
      <section class="panel-card space-y-5">
        <h2 class="section-title">Fuse QSR into plasma</h2>
        <div class="space-y-2">
          <Field label="Who gets the plasma?">
            <Input v-model="beneficiary" placeholder="z1…" aria-label="beneficiary" class="font-mono" />
          </Field>
          <button
            v-if="ownAddress"
            type="button"
            class="choice py-1.5 text-xs font-semibold"
            :class="forSelf ? 'choice-on' : ''"
            :aria-pressed="forSelf"
            @click="beneficiary = ownAddress"
          >This account</button>
        </div>
        <div class="space-y-1.5">
          <AmountInput v-model="amount" label="Amount (QSR)" symbol="QSR" :max="maxFusion" />
          <p class="hint">{{ availableLabel }}</p>
          <p class="hint">Minimum 10 QSR, in whole QSR increments.</p>
        </div>
        <p v-if="estimate > 0" class="hint">≈ {{ estimate.toLocaleString('en-US') }} plasma</p>
        <Button class="h-12 w-full text-base font-bold" :disabled="!canFuse" @click="fuse">Fuse Plasma</Button>
        <p class="hint text-center">Your QSR stays yours. Unfuse an entry to get it back.</p>
      </section>

      <section class="panel-list">
        <h2 class="section-title px-6 py-4">Your fusions</h2>
        <div v-for="e in fusionEntries" :key="e.id" class="list-row justify-between text-sm">
          <span class="min-w-0 font-mono" :title="e.beneficiary">
            {{ formatAmount(e.qsrAmount, 8) }} QSR → {{ shortAddress(e.beneficiary) }}
            <span v-if="!e.isRevocable" class="block font-sans text-xs text-muted-foreground">Locked until momentum {{ e.expirationHeight.toLocaleString('en-US') }}</span>
          </span>
          <Button
            variant="outline"
            :disabled="!e.isRevocable"
            aria-label="unfuse QSR"
            @click="cancel(e.id)"
          >Unfuse</Button>
        </div>
        <p v-if="fusionEntries.length === 0" class="border-t border-border/70 px-6 py-6 text-sm text-muted-foreground">
          No fusion entries.
        </p>
      </section>
    </div>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing transaction…</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
