<script setup lang="ts">
import { useRouter } from 'vue-router'
import { CheckIcon } from '@lucide/vue'
import { useAttention } from '../lib/useAttention'
import { useNodeStore } from '../stores/node'

// Renders the shared attention list. `dense` is the Advanced table-like form;
// the default is the roomier Everyday card form.
defineProps<{ dense?: boolean }>()
const router = useRouter()
const node = useNodeStore()
const { items } = useAttention()

const TONES = {
  primary: 'bg-primary/15 text-primary',
  info: 'bg-info/15 text-info',
  warning: 'bg-warning/15 text-warning',
  danger: 'bg-destructive/15 text-destructive',
} as const
</script>

<template>
  <ul v-if="items.length" class="flex flex-col" data-testid="attention-list">
    <li v-for="item in items" :key="item.key"
      class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-border/70"
      :class="dense ? 'px-6 py-2.5' : 'py-3'">
      <span class="grid h-8 w-8 flex-none place-items-center rounded-full" :class="TONES[item.tone]" aria-hidden="true">
        <component :is="item.icon" :size="16" />
      </span>
      <span class="min-w-0 flex-1 basis-40">
        <span class="block text-sm font-semibold text-foreground">{{ item.title }}</span>
        <span class="block text-xs text-muted-foreground">{{ item.detail }}</span>
      </span>
      <button type="button"
        class="inline-flex h-8.5 items-center rounded-lg border border-input px-3 text-sm font-semibold text-foreground transition-colors hover:bg-foreground/[0.06]"
        :data-testid="`attention-${item.key}`"
        @click="router.push(item.to)">{{ item.action }}</button>
    </li>
  </ul>
  <p v-else class="flex items-center gap-2.5 text-sm text-muted-foreground" :class="dense ? 'border-t border-border/70 px-6 py-4' : 'pt-3'">
    <span class="grid h-8 w-8 flex-none place-items-center rounded-full bg-primary/15 text-primary" aria-hidden="true"><CheckIcon :size="16" /></span>
    {{ node.connected ? 'Nothing needs you right now.' : 'Shown once the wallet is connected to a node.' }}
  </p>
</template>
