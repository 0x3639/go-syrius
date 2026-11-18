<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePlasmaStore } from '../../stores/plasma'
import { useTxStore } from '../../stores/tx'
import { formatAmount, formatAmountExact, shortAddress, toBase } from '../../lib/format'
import { useBalancesStore } from '../../stores/balances'
import Field from '../Field.vue'
import AmountInput from '../AmountInput.vue'

const plasma = usePlasmaStore()
const tx = useTxStore()

const { info, fusionEntries } = storeToRefs(plasma)

const beneficiary = ref('')
const amount = ref('')
const estimate = ref(0)
const error = ref('')
const balances = useBalancesStore()
const available = computed(() => balances.items.find(b => b.symbol === 'QSR')?.amount ?? '0')
const availableLabel = computed(() => balances.available ? `${formatAmountExact(available.value, 8)} QSR available` : 'Balance unavailable')
const canFuse = computed(() => {
  try {
    const base = BigInt(toBase(amount.value, 8))
    return balances.available && /^z1[0-9a-z]{38}$/.test(beneficiary.value) && base > 0n && base <= BigInt(available.value)
  } catch { return false }
})

onMounted(() => plasma.refresh())

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
    const preview = await Nom.PrepareFuse(beneficiary.value, toBase(amount.value, 8))
    tx.awaitConfirm(preview)
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function cancel(id: string) {
  error.value = ''
  try {
    const preview = await Nom.PrepareCancelFuse(id)
    tx.awaitConfirm(preview)
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
    <p v-if="info" class="text-sm text-muted-foreground">
      Current plasma {{ info.currentPlasma.toLocaleString('en-US') }} / {{ info.maxPlasma.toLocaleString('en-US') }} · QSR fused
      {{ formatAmount(info.qsrFused, 8) }} QSR
    </p>

    <section class="space-y-3 panel-card">
      <h2 class="text-sm font-medium text-foreground">Fuse Plasma</h2>
      <Field label="Beneficiary Address">
        <Input v-model="beneficiary" placeholder="z1…" aria-label="beneficiary" />
      </Field>
      <div class="space-y-1">
        <AmountInput v-model="amount" label="Amount (QSR)" aria-label="qsr amount" :max="balances.available ? formatAmountExact(available, 8) : ''" />
        <p class="text-xs text-muted-foreground">{{ availableLabel }}</p>
      </div>
      <p v-if="estimate > 0" class="text-xs text-muted-foreground">≈ {{ estimate.toLocaleString('en-US') }} plasma</p>
      <Button class="w-full" :disabled="!canFuse" @click="fuse">Fuse Plasma</Button>
    </section>

    <section class="space-y-2 panel-card">
      <h2 class="text-sm font-medium text-foreground">Fusion entries</h2>
      <div
        v-for="e in fusionEntries"
        :key="e.id"
        class="flex items-center justify-between text-sm"
      >
        <span class="font-mono" :title="e.beneficiary">
          {{ formatAmount(e.qsrAmount, 8) }} QSR → {{ shortAddress(e.beneficiary) }}
          <span v-if="!e.isRevocable" class="block text-xs text-muted-foreground">Locked until momentum {{ e.expirationHeight.toLocaleString('en-US') }}</span>
        </span>
        <Button
          variant="outline"
          :disabled="!e.isRevocable"
          aria-label="unfuse QSR"
          @click="cancel(e.id)"
        >Unfuse</Button>
      </div>
      <p v-if="fusionEntries.length === 0" class="text-xs text-muted-foreground">
        No fusion entries.
      </p>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">
      Preparing transaction…
    </p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
