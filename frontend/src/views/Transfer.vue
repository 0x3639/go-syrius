<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useToast } from 'nom-ui'
import { watch } from 'vue'
import { useRoute } from 'vue-router'
import { useBalancesStore } from '../stores/balances'
import { useTxStore } from '../stores/tx'
import { toBase } from '../lib/format'
import { currentRequestEpoch } from '../lib/requestEpoch'
import SendForm from '../components/SendForm.vue'
import TxModal from '../components/TxModal.vue'
import TxResult from '../components/TxResult.vue'
import BalanceNotice from '../components/BalanceNotice.vue'

// Address book → Send hands the recipient over in ?to= (display intent only;
// the form and the backend both re-validate it).
const route = useRoute()
const initialRecipient = typeof route?.query?.to === 'string' ? route.query.to : ''
const initialToken = typeof route?.query?.token === 'string' ? route.query.token : ''

const balances = useBalancesStore()
const { items } = storeToRefs(balances)
const tx = useTxStore()
const { status, error } = storeToRefs(tx)

let toast: ReturnType<typeof useToast> | undefined
try { toast = useToast() } catch { toast = undefined }

async function onSend(intent: { recipient: string; zts: string; amountDecimal: string }) {
  const epoch = currentRequestEpoch()
  // A retryable confirm failure may still own its backend hold. The transfer
  // page renders a new form in error state, so release that exact old hold and
  // await the local binding before preparing the replacement transaction.
  if (tx.status === 'error') await tx.discard()
  const tok = items.value.find((b) => b.zts === intent.zts)
  // Recheck the amount at the view boundary and surface conversion failures.
  let amount: string
  try {
    if (epoch !== currentRequestEpoch() || !balances.available || !tok) {
      throw new Error('Balances changed. Review the amount and try again.')
    }
    amount = toBase(intent.amountDecimal, tok.decimals)
    if (BigInt(amount) <= 0n || BigInt(amount) > BigInt(tok.amount)) {
      throw new Error('Amount must be positive and no greater than the available balance.')
    }
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
    <!-- The form brings its own two-column layout (form + summary). -->
    <SendForm v-if="status === 'idle' || status === 'error'" :initial-recipient="initialRecipient" :initial-token="initialToken" @send="onSend" />
    <p v-if="status === 'error'" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <div v-if="status === 'preparing' || status === 'awaiting' || status === 'publishing' || status === 'done'" class="panel-card mx-auto max-w-[40rem]">
      <p v-if="status === 'preparing'" class="text-sm text-muted-foreground">Preparing transaction…</p>
      <TxModal v-if="status === 'awaiting' || status === 'publishing'" />
      <TxResult v-if="status === 'done'" @close="tx.reset()" />
    </div>
  </div>
</template>
