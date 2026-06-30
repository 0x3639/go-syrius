<script setup lang="ts">
import { ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useTxStore } from '../stores/tx'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
import { CheckIcon, CopyIcon } from '@lucide/vue'

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
          <CopyIcon v-if="!copied" :size="14" />
          <CheckIcon v-else :size="14" class="text-primary" />
        </button>
      </div>
    </div>
    <Button class="w-full" aria-label="close" @click="emit('close')">Close</Button>
  </div>
</template>
