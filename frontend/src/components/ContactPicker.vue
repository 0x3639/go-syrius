<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { storeToRefs } from 'pinia'
import { shortAddress } from '../lib/format'
import { useContactsStore } from '../stores/contacts'

const props = defineProps<{ currentAddress?: string }>()
const emit = defineEmits<{ (e: 'select', address: string): void }>()

const contacts = useContactsStore()
const { items } = storeToRefs(contacts)

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const name = ref('')
const address = ref('')
const err = ref('')

const isValidAddr = (a: string) => /^z1[0-9a-z]{38}$/.test(a)

function toggle() {
  open.value = !open.value
  if (open.value) {
    contacts.load()
    err.value = ''
    name.value = ''
    // Prefill the add form's address with the current recipient if it's valid.
    address.value = props.currentAddress && isValidAddr(props.currentAddress) ? props.currentAddress : ''
  }
}

function pick(a: string) {
  emit('select', a)
  open.value = false
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

function onDoc(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => {
  contacts.load()
  document.addEventListener('click', onDoc)
})
onBeforeUnmount(() => document.removeEventListener('click', onDoc))
</script>

<template>
  <div ref="root" class="relative inline-block">
    <button
      type="button"
      aria-label="address book"
      title="Address book"
      class="grid h-8 w-8 place-items-center rounded-lg border border-border text-muted-foreground transition-colors hover:text-foreground"
      @click="toggle"
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>
    </button>

    <div
      v-if="open"
      class="absolute right-0 top-full z-30 mt-2 w-72 overflow-hidden rounded-lg border border-border bg-card shadow-lg"
    >
      <div class="max-h-56 overflow-y-auto">
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
      <div class="space-y-2 border-t border-border bg-background/40 p-3">
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
  </div>
</template>
