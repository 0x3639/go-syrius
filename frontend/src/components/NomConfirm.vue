<script setup lang="ts">
import { computed } from 'vue'
import { Button, Dialog, DialogContent, DialogHeader, DialogTitle } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import TxModal from './TxModal.vue'
import TxResult from './TxResult.vue'

// Global confirm modal for PANEL-triggered transactions. The NoM panels call
// tx.awaitConfirm(preview) (status -> 'awaiting') but don't own a modal, so this
// renders the confirm/result UI for them. The Transfer page drives its own
// TxModal/TxResult, so AppShell renders this dialog on every route EXCEPT
// 'transfer' to avoid a double modal on the same tx status.
const tx = useTxStore()
const open = computed({
  // Stay open through 'publishing' too, so the modal doesn't flicker closed
  // between Confirm and the published result — and through 'error', so a
  // failed publish shows its failure instead of the dialog snapping shut.
  get: () => tx.status === 'awaiting' || tx.status === 'publishing' || tx.status === 'done' || tx.status === 'error',
  set: (v: boolean) => {
    if (!v) {
      // Closing while awaiting discards the held block (synchronous, then a
      // background identity-checked CancelPending); after a publish
      // (done/error) it just clears the result.
      tx.status === 'awaiting' ? tx.discard() : tx.reset()
    }
  },
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="w-[40rem] max-w-[95vw]">
      <DialogHeader><DialogTitle>Confirm</DialogTitle></DialogHeader>
      <TxModal v-if="tx.status === 'awaiting' || tx.status === 'publishing'" />
      <TxResult v-else-if="tx.status === 'done'" @close="open = false" />
      <div v-else-if="tx.status === 'error'" class="space-y-3">
        <p class="text-sm text-destructive" role="alert">{{ tx.error || 'Transaction failed.' }}</p>
        <Button class="w-full" aria-label="close" @click="open = false">Close</Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
