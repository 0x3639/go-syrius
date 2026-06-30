<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import { shortAddress } from '../lib/format'
import { useContactsStore, type Contact } from '../stores/contacts'
import TopBar from '../components/TopBar.vue'
import { ArrowLeftIcon, CheckIcon, XIcon, PencilIcon } from '@lucide/vue'

const router = useRouter()
const contacts = useContactsStore()
const { items } = storeToRefs(contacts)

const search = ref('')
const name = ref('')
const address = ref('')
const err = ref('')
const renamingAddr = ref<string | null>(null)
const renameDraft = ref('')

const isValidAddr = (a: string) => /^z1[0-9a-z]{38}$/.test(a)

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((c) => c.name.toLowerCase().includes(q) || c.address.toLowerCase().includes(q))
})

async function save() {
  err.value = ''
  try {
    await contacts.add(name.value.trim(), address.value.trim())
    name.value = ''
    address.value = ''
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

function startRename(c: Contact) {
  renamingAddr.value = c.address
  renameDraft.value = c.name
}
function cancelRename() {
  renamingAddr.value = null
  renameDraft.value = ''
}
async function saveRename(c: Contact) {
  const n = renameDraft.value.trim()
  if (n && n !== c.name) {
    try {
      await contacts.add(n, c.address) // AddContact upserts by address → renames
    } catch {
      /* ignore */
    }
  }
  cancelRename()
}

onMounted(() => contacts.load())
</script>

<template>
  <TopBar />
  <div class="mx-auto mt-6 w-[42rem] max-w-full space-y-4 px-4 pb-10">
    <div class="flex items-center gap-2">
      <button
        class="grid h-8 w-8 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        aria-label="back to wallet"
        @click="router.push('/dashboard')"
      >
        <ArrowLeftIcon :size="20" />
      </button>
      <h1 class="text-xl text-foreground">Address book</h1>
    </div>

        <!-- Add -->
        <div class="space-y-2 rounded-lg border border-border bg-background/40 p-3">
          <p class="text-xs font-medium text-muted-foreground">Add address</p>
          <div class="flex flex-col gap-2 sm:flex-row">
            <Input v-model="name" aria-label="contact name" placeholder="Name" class="sm:w-48" />
            <Input v-model="address" aria-label="contact address" placeholder="z1…" class="flex-1 font-mono" />
            <Button :disabled="!name.trim() || !isValidAddr(address.trim())" aria-label="save contact" @click="save">Save</Button>
          </div>
          <p v-if="err" class="text-xs text-destructive">{{ err }}</p>
        </div>

        <!-- Search -->
        <Input v-model="search" aria-label="search addresses" placeholder="Search by name or address…" />

        <!-- List -->
        <div class="max-h-[26rem] divide-y divide-border overflow-y-auto rounded-lg border border-border">
          <p v-if="filtered.length === 0" class="px-4 py-6 text-center text-sm text-muted-foreground">
            {{ items.length === 0 ? 'No saved addresses yet.' : 'No matches.' }}
          </p>
          <div v-for="c in filtered" :key="c.address" class="group flex items-center gap-3 px-4 py-3">
            <template v-if="renamingAddr === c.address">
              <Input v-model="renameDraft" :aria-label="`rename ${c.name}`" class="flex-1" @keyup.enter="saveRename(c)" @keyup.esc="cancelRename" />
              <button class="grid h-8 w-8 flex-none place-items-center rounded border border-primary/40 text-primary" :aria-label="`save rename ${c.name}`" @click="saveRename(c)"><CheckIcon :size="16" /></button>
              <button class="grid h-8 w-8 flex-none place-items-center rounded border border-border text-muted-foreground" aria-label="cancel rename" @click="cancelRename"><XIcon :size="16" /></button>
            </template>
            <template v-else>
              <div class="min-w-0 flex-1">
                <div class="truncate font-semibold text-foreground">{{ c.name }}</div>
                <div class="truncate font-mono text-xs text-muted-foreground">{{ shortAddress(c.address) }}</div>
              </div>
              <button class="grid h-8 w-8 flex-none place-items-center rounded border border-transparent text-muted-foreground group-hover:border-border group-hover:text-foreground" title="Rename" :aria-label="`rename ${c.name}`" @click="startRename(c)"><PencilIcon :size="16" /></button>
              <button class="grid h-8 w-8 flex-none place-items-center rounded border border-transparent text-muted-foreground hover:text-destructive group-hover:border-border" title="Delete" :aria-label="`delete ${c.name}`" @click="remove(c.address)"><XIcon :size="16" /></button>
            </template>
          </div>
        </div>
  </div>
</template>
