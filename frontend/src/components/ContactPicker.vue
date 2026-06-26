<script setup lang="ts">
import { ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { shortAddress } from '../lib/format'
import { useContactsStore } from '../stores/contacts'

// Inline address-book panel. Rendered in the Send form's normal flow (not an
// overlay) so it never stacks behind other fields or feels like a modal-on-modal.
const props = defineProps<{ open: boolean; currentAddress?: string }>()
const emit = defineEmits<{ (e: 'select', address: string): void; (e: 'close'): void }>()

const contacts = useContactsStore()
const { items } = storeToRefs(contacts)

const name = ref('')
const address = ref('')
const err = ref('')

const isValidAddr = (a: string) => /^z1[0-9a-z]{38}$/.test(a)

// On open: refresh the list and prefill the add-form address from the current
// recipient (if it's a valid z1 address).
watch(
  () => props.open,
  (o) => {
    if (o) {
      contacts.load()
      err.value = ''
      name.value = ''
      address.value = props.currentAddress && isValidAddr(props.currentAddress) ? props.currentAddress : ''
    }
  },
  { immediate: true },
)

function pick(a: string) {
  emit('select', a)
}

async function save() {
  err.value = ''
  try {
    await contacts.add(name.value.trim(), address.value.trim())
    name.value = ''
  } catch (e: any) {
    err.value = e?.message ?? String(e)
  }
}

async function remove(a: string) {
  try {
    await contacts.remove(a)
  } catch {
    /* ignore */
  }
}
</script>

<template>
  <div v-if="open" class="overflow-hidden rounded-lg border border-border bg-card">
    <div class="flex items-center justify-between border-b border-border px-3 py-2">
      <span class="text-xs font-medium text-muted-foreground">Address book</span>
      <button type="button" aria-label="close address book" class="text-muted-foreground transition-colors hover:text-foreground" @click="emit('close')">✕</button>
    </div>

    <div class="max-h-44 overflow-y-auto">
      <p v-if="items.length === 0" class="px-3 py-3 text-sm text-muted-foreground">No saved addresses yet.</p>
      <div
        v-for="ct in items"
        :key="ct.address"
        role="button"
        :aria-label="`select ${ct.name}`"
        class="group flex cursor-pointer items-center gap-2 border-t border-border px-3 py-2 first:border-t-0 hover:bg-foreground/[0.06]"
        @click="pick(ct.address)"
      >
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-foreground">{{ ct.name }}</div>
          <div class="truncate font-mono text-xs text-muted-foreground">{{ shortAddress(ct.address) }}</div>
        </div>
        <button
          type="button"
          :aria-label="`delete ${ct.name}`"
          class="grid h-6 w-6 flex-none place-items-center rounded text-muted-foreground opacity-0 transition-opacity hover:text-destructive group-hover:opacity-100"
          @click.stop="remove(ct.address)"
        >✕</button>
      </div>
    </div>

    <!-- Add a contact -->
    <div class="space-y-2 border-t border-border p-3">
      <p class="text-xs font-medium text-muted-foreground">Add address</p>
      <input
        v-model="name"
        aria-label="contact name"
        placeholder="Name"
        class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm text-foreground outline-none focus:border-primary"
      />
      <input
        v-model="address"
        aria-label="contact address"
        placeholder="z1…"
        class="w-full rounded border border-border bg-background px-2 py-1.5 font-mono text-xs text-foreground outline-none focus:border-primary"
      />
      <p v-if="err" class="text-xs text-destructive">{{ err }}</p>
      <button
        type="button"
        aria-label="save contact"
        :disabled="!name.trim() || !isValidAddr(address.trim())"
        class="w-full rounded bg-primary/15 py-1.5 text-sm font-semibold text-primary transition-colors hover:bg-primary/25 disabled:opacity-40"
        @click="save"
      >
        Save
      </button>
    </div>
  </div>
</template>
