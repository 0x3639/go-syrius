<script setup lang="ts">
import { watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Dialog, DialogContent, DialogHeader, DialogTitle, useToast } from 'nom-ui'
import { useBalancesStore } from '../stores/balances'
import { useTxStore } from '../stores/tx'
import { toBase } from '../lib/format'
import SendForm from './SendForm.vue'
import TxModal from './TxModal.vue'
import TxResult from './TxResult.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const balances = useBalancesStore()
const { items } = storeToRefs(balances)
const tx = useTxStore()
const { status, error } = storeToRefs(tx)

// useToast may be unavailable (e.g. no Toaster mounted in tests); guard it.
let toast: ReturnType<typeof useToast> | undefined
try {
  toast = useToast()
} catch {
  toast = undefined
}

// On send INTENT: look up the token's decimals in balances, convert the decimal
// amount to base units via toBase, then ask the tx store to PREPARE (build the
// block). The amount confirmed later (TxModal) comes from preview, not here.
async function onSend(intent: { recipient: string; zts: string; amountDecimal: string }) {
  const { recipient, zts, amountDecimal } = intent
  const tok = items.value.find((b) => b.zts === zts)
  await tx.prepare(recipient, zts, toBase(amountDecimal, tok?.decimals ?? 8))
}

// Closing the Dialog resets the tx flow and notifies the parent.
function onOpenChange(value: boolean) {
  if (!value) {
    tx.reset()
    emit('update:open', false)
  }
}

// On a successful publish, surface a toast (guarded if unavailable).
watch(status, (s) => {
  if (s === 'done') toast?.show('Transaction published', 'success')
})
</script>

<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Send</DialogTitle>
      </DialogHeader>
      <SendForm @send="onSend" />
      <p v-if="status === 'preparing'" class="text-sm text-muted-foreground">
        Preparing… (PoW if required)
      </p>
      <p v-if="status === 'error'" class="text-sm text-destructive" role="alert">
        {{ error }}
      </p>
      <TxModal v-if="status === 'awaiting'" />
      <TxResult v-if="status === 'done'" />
    </DialogContent>
  </Dialog>
</template>
