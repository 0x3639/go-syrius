<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

const emit = defineEmits<{ (e: 'close'): void }>()

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
  <div class="space-y-3 rounded border border-success/40 bg-card p-4">
    <p class="font-medium text-success">Transaction published</p>
    <div>
      <span class="text-xs text-muted-foreground">Hash</span>
      <div class="mt-1 flex items-start gap-2">
        <div class="min-w-0 flex-1 break-all font-mono text-xs text-foreground">{{ hash }}</div>
        <button
          type="button"
          :aria-label="copied ? 'hash copied' : 'copy hash'"
          :title="copied ? 'Copied' : 'Copy hash'"
          class="grid h-7 w-7 flex-none place-items-center rounded text-muted-foreground transition-colors hover:bg-foreground/[0.08] hover:text-foreground"
          @click="copy"
        >
          <svg v-if="!copied" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="13" height="13" x="9" y="9" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <svg v-else class="text-primary" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        </button>
      </div>
    </div>
    <Button class="w-full" aria-label="close" @click="emit('close')">Close</Button>
  </div>
</template>
