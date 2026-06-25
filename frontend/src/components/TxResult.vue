<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

const tx = useTxStore()
const { hash } = storeToRefs(tx)
const copied = ref(false)

async function copy() {
  await ClipboardSetText(hash.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 1200)
}
</script>

<template>
  <div class="space-y-2 rounded border border-success/40 bg-card p-4">
    <p class="text-success">Transaction published</p>
    <div class="break-all font-mono text-xs">{{ hash }}</div>
    <Button variant="ghost" size="xs" @click="copy">{{
      copied ? 'Copied' : 'Copy hash'
    }}</Button>
  </div>
</template>
