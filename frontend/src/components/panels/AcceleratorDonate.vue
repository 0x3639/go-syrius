<script setup lang="ts">
import { ref, watch } from 'vue'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()
const amount = ref('')
const token = ref('QSR')
const error = ref('')

async function donate() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareDonate(amount.value, token.value))
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
  <div class="space-y-3 p-4">
    <p class="text-xs text-muted-foreground">Donate ZNN or QSR to the Accelerator-Z funding pool.</p>
    <div class="flex items-end gap-2">
      <div class="flex-1">
        <Field label="Amount (base units)">
          <Input v-model="amount" placeholder="amount (base units)" aria-label="donate amount" />
        </Field>
      </div>
      <select
        v-model="token"
        class="rounded border border-border bg-muted px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-primary"
        aria-label="donate token"
      >
        <option value="ZNN">ZNN</option>
        <option value="QSR">QSR</option>
      </select>
      <Button aria-label="donate" @click="donate">Donate</Button>
    </div>
    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
