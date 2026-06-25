<script setup lang="ts">
import { ref } from 'vue'
import { useWalletStore, type AccountInfo } from '../stores/wallet'

const wallet = useWalletStore()
const editing = ref(false)
const draft = ref('')

function labelFor(a: { index: number; label?: string }) {
  return a.label && a.label.trim() ? a.label : `Account ${a.index}`
}

async function onChange(e: Event) {
  await wallet.select(Number((e.target as HTMLSelectElement).value))
}

function startEdit() {
  draft.value =
    wallet.accounts.find((a: AccountInfo) => a.index === wallet.activeIndex)?.label ?? ''
  editing.value = true
}

async function saveEdit() {
  await wallet.setLabel(wallet.activeIndex, draft.value.trim())
  editing.value = false
}
</script>

<template>
  <div class="flex items-center gap-2">
    <select
      class="rounded bg-card px-2 py-1 text-sm"
      :value="wallet.activeIndex"
      @change="onChange"
    >
      <option v-for="a in wallet.accounts" :key="a.index" :value="a.index">{{ labelFor(a) }}</option>
    </select>
    <template v-if="editing">
      <input class="rounded bg-card px-2 py-1 text-sm" v-model="draft" aria-label="account label" />
      <button class="text-xs text-primary" @click="saveEdit">Save</button>
    </template>
    <button v-else class="text-xs text-muted-foreground" @click="startEdit" aria-label="edit label">
      ✎
    </button>
  </div>
</template>
