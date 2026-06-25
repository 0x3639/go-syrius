<script setup lang="ts">
import { computed } from 'vue'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import AddressDisplay from './AddressDisplay.vue'
import UnreceivedPanel from './UnreceivedPanel.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const wallet = useWalletStore()
const address = computed(() => wallet.activeAddress())

function onOpenChange(value: boolean) {
  emit('update:open', value)
}
</script>

<template>
  <Dialog :open="props.open" @update:open="onOpenChange">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Receive</DialogTitle>
      </DialogHeader>
      <div class="space-y-5">
        <AddressDisplay :address="address" />
        <UnreceivedPanel />
      </div>
    </DialogContent>
  </Dialog>
</template>
