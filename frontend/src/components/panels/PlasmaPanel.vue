<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { usePlasmaStore } from '../../stores/plasma'
import { useTxStore } from '../../stores/tx'
import { formatAmount, toBase } from '../../lib/format'
import Field from '../Field.vue'

const plasma = usePlasmaStore()
const tx = useTxStore()

const { info, fusionEntries } = storeToRefs(plasma)

const beneficiary = ref('')
const amount = ref('')
const estimate = ref(0)
const error = ref('')

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
  <div class="space-y-4 p-4">
    <p v-if="info" class="text-sm text-muted-foreground">
      Current plasma {{ info.currentPlasma }} / {{ info.maxPlasma }} · QSR fused
      {{ formatAmount(info.qsrFused, 8) }} QSR
    </p>

    <section class="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Fuse Plasma</h2>
      <Field label="Beneficiary Address">
        <Input v-model="beneficiary" placeholder="z1…" aria-label="beneficiary" />
      </Field>
      <Field
        label="Amount (QSR)"
        :hint="estimate > 0 ? `≈ ${estimate} plasma` : 'Available / Minimum'"
      >
        <Input v-model="amount" placeholder="QSR amount" aria-label="qsr amount" />
      </Field>
      <Button class="w-full" @click="fuse">Fuse Plasma</Button>
    </section>

    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Fusion entries</h2>
      <div
        v-for="e in fusionEntries"
        :key="e.id"
        class="flex items-center justify-between text-sm"
      >
        <span class="font-mono">
          {{ formatAmount(e.qsrAmount, 8) }} QSR → {{ e.beneficiary.slice(0, 10) }}…
        </span>
        <Button
          variant="outline"
          :disabled="!e.isRevocable"
          aria-label="cancel fusion"
          @click="cancel(e.id)"
        >Cancel</Button>
      </div>
      <p v-if="fusionEntries.length === 0" class="text-xs text-muted-foreground">
        No fusion entries.
      </p>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">
      Preparing… (PoW if required)
    </p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
