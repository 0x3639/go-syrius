<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useToast } from 'nom-ui'
import { watch } from 'vue'
import { useBalancesStore } from '../stores/balances'
import { useTxStore } from '../stores/tx'
import { toBase } from '../lib/format'
import SendForm from '../components/SendForm.vue'
import TxModal from '../components/TxModal.vue'
import TxResult from '../components/TxResult.vue'
import BalanceNotice from '../components/BalanceNotice.vue'

const balances = useBalancesStore()
const { items } = storeToRefs(balances)
const tx = useTxStore()
const { status, error } = storeToRefs(tx)

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { toast = undefined }

async function onSend(intent: { recipient: string; zts: string; amountDecimal: string }) {
  // A retryable confirm failure may still own its backend hold. The transfer
  // page renders a new form in error state, so release that exact old hold and
  // await the local binding before preparing the replacement transaction.
  if (tx.status === 'error') await tx.discard()
  const tok = items.value.find((b) => b.zts === intent.zts)
  // Recheck the amount at the view boundary and surface conversion failures.
  let amount: string
  try {
    amount = toBase(intent.amountDecimal, tok?.decimals ?? 8)
  } catch (e: unknown) {
    toast?.show(e instanceof Error ? e.message : String(e), 'error')
    return
  }
  await tx.prepare(intent.recipient, intent.zts, amount)
}

watch(status, (s) => { if (s === 'done') toast?.show('Transaction published', 'success') })
</script>

<template>
  <div class="page-content">
    <BalanceNotice />
    <div class="panel-card">
      <SendForm v-if="status === 'idle' || status === 'error'" @send="onSend" />
      <p v-if="status === 'preparing'" class="text-sm text-muted-foreground">Preparing transaction…</p>
      <p v-if="status === 'error'" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <TxModal v-if="status === 'awaiting' || status === 'publishing'" />
      <TxResult v-if="status === 'done'" @close="tx.reset()" />
    </div>
  </div>
</template>
