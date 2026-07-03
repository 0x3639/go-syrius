<script setup lang="ts">
import { computed } from 'vue'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from 'nom-ui'
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
  // between Confirm and the published result.
  get: () => tx.status === 'awaiting' || tx.status === 'publishing' || tx.status === 'done',
  set: (v: boolean) => {
    if (!v) {
      // Closing while awaiting cancels the held block; after a publish (done)
      // it just clears the result.
      tx.status === 'awaiting' ? tx.cancel() : tx.reset()
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
    </DialogContent>
  </Dialog>
</template>
