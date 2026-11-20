<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import { shortAddress } from '../lib/format'
import { useContactsStore, type Contact } from '../stores/contacts'
import { CheckIcon, XIcon, PencilIcon, SearchIcon } from '@lucide/vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const contacts = useContactsStore()

// Jump to Send with this contact pre-filled as the recipient.
function sendTo(a: string) {
  router?.push({ path: '/transfer', query: { to: a } })
}
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
  <div class="page-content">
    <div class="page-split">
      <!-- List -->
      <section class="panel-list">
        <div class="flex flex-wrap items-center gap-3 px-6 py-4">
          <h2 class="section-title flex-1">Contacts <span class="font-normal text-muted-foreground">· {{ items.length }}</span></h2>
          <div class="relative w-full max-w-72">
            <SearchIcon :size="15" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="search" aria-label="search addresses" placeholder="Search by name or address" class="pl-9" />
          </div>
        </div>
        <div class="max-h-[34rem] overflow-y-auto">
          <p v-if="filtered.length === 0" class="border-t border-border/70 px-6 py-8 text-center text-sm text-muted-foreground">
            {{ items.length === 0 ? 'No saved addresses yet.' : 'No matches.' }}
          </p>
          <div v-for="c in filtered" :key="c.address" class="list-row group">
            <template v-if="renamingAddr === c.address">
              <Input v-model="renameDraft" :aria-label="`rename ${c.name}`" class="flex-1" @keyup.enter="saveRename(c)" @keyup.esc="cancelRename" />
              <button class="grid h-9 w-9 flex-none place-items-center rounded-lg border border-primary/40 text-primary" :aria-label="`save rename ${c.name}`" @click="saveRename(c)"><CheckIcon :size="16" /></button>
              <button class="grid h-9 w-9 flex-none place-items-center rounded-lg border border-border text-muted-foreground" aria-label="cancel rename" @click="cancelRename"><XIcon :size="16" /></button>
            </template>
            <template v-else>
              <span class="avatar-dot text-sm" aria-hidden="true">{{ c.name.charAt(0).toUpperCase() }}</span>
              <div class="min-w-0 flex-1">
                <div class="truncate font-semibold text-foreground">{{ c.name }}</div>
                <div class="truncate font-mono text-xs text-muted-foreground" :title="c.address">{{ shortAddress(c.address) }}</div>
              </div>
              <button class="h-9 flex-none rounded-lg bg-primary/15 px-3.5 text-sm font-bold text-primary transition-colors hover:bg-primary/25" :aria-label="`send to ${c.name}`" @click="sendTo(c.address)">Send</button>
              <button class="grid h-9 w-9 flex-none place-items-center rounded-lg border border-input text-muted-foreground transition-colors hover:text-foreground" title="Rename" :aria-label="`rename ${c.name}`" @click="startRename(c)"><PencilIcon :size="15" /></button>
              <button class="grid h-9 w-9 flex-none place-items-center rounded-lg border border-input text-muted-foreground transition-colors hover:text-destructive" title="Delete" :aria-label="`delete ${c.name}`" @click="remove(c.address)"><XIcon :size="15" /></button>
            </template>
          </div>
        </div>
      </section>

      <!-- Add -->
      <section class="panel-card flex flex-col gap-3.5">
        <h2 class="section-title">Add a contact</h2>
        <label class="field-label flex flex-col gap-1.5">Name
          <Input v-model="name" aria-label="contact name" placeholder="e.g. Exchange deposit" />
        </label>
        <label class="field-label flex flex-col gap-1.5">Address
          <Input v-model="address" aria-label="contact address" placeholder="z1…" class="font-mono" />
        </label>
        <p class="hint">The address is checked before it is saved.</p>
        <Button class="h-11" :disabled="!name.trim() || !isValidAddr(address.trim())" aria-label="save contact" @click="save">Save contact</Button>
        <p v-if="err" class="text-xs text-destructive">{{ err }}</p>
        <p class="hint border-t border-border pt-3">Contacts are stored on this device only.</p>
      </section>
    </div>
  </div>
</template>
