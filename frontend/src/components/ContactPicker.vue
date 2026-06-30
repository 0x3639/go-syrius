<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { shortAddress } from '../lib/format'
import { useContactsStore } from '../stores/contacts'
import { XIcon } from '@lucide/vue'

// Inline address-book panel for Send: SEARCH + SELECT only. Adding/editing lives
// on the dedicated /address-book screen so this stays compact as the book grows.
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ (e: 'select', address: string): void; (e: 'close'): void }>()

const router = useRouter()
const contacts = useContactsStore()
const { items } = storeToRefs(contacts)
const search = ref('')

watch(
  () => props.open,
  (o) => {
    if (o) {
      contacts.load()
      search.value = ''
    }
  },
  { immediate: true },
)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((c) => c.name.toLowerCase().includes(q) || c.address.toLowerCase().includes(q))
})

function pick(a: string) {
  emit('select', a)
}
function manage() {
  router.push('/address-book')
}
</script>

<template>
  <div v-if="open" class="overflow-hidden rounded-lg border border-border bg-card">
    <div class="flex items-center justify-between border-b border-border px-3 py-2">
      <span class="text-xs font-medium text-muted-foreground">Address book</span>
      <button type="button" aria-label="close address book" class="text-muted-foreground transition-colors hover:text-foreground" @click="emit('close')"><XIcon :size="16" /></button>
    </div>

    <div class="p-2">
      <input
        v-model="search"
        aria-label="search addresses"
        placeholder="Search…"
        class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm text-foreground outline-none focus:border-primary"
      />
    </div>

    <div class="max-h-48 overflow-y-auto">
      <p v-if="filtered.length === 0" class="px-3 pb-3 text-sm text-muted-foreground">
        {{ items.length === 0 ? 'No saved addresses yet.' : 'No matches.' }}
      </p>
      <div
        v-for="c in filtered"
        :key="c.address"
        role="button"
        :aria-label="`select ${c.name}`"
        class="flex cursor-pointer items-center gap-2 border-t border-border px-3 py-2 hover:bg-foreground/[0.06]"
        @click="pick(c.address)"
      >
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-foreground">{{ c.name }}</div>
          <div class="truncate font-mono text-xs text-muted-foreground">{{ shortAddress(c.address) }}</div>
        </div>
      </div>
    </div>

    <button
      type="button"
      aria-label="manage address book"
      class="flex w-full items-center justify-center gap-1 border-t border-border px-3 py-2 text-xs font-medium text-primary transition-colors hover:bg-foreground/[0.06]"
      @click="manage"
    >
      Manage address book →
    </button>
  </div>
</template>
