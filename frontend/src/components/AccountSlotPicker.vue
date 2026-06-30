<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ChevronDownIcon, PlusIcon, CheckIcon, XIcon, PencilIcon, CopyIcon, WalletIcon } from '@lucide/vue'
import { shortAddress } from '../lib/format'
import { useWalletStore, type AccountInfo } from '../stores/wallet'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

const props = defineProps<{ variant?: 'pill' }>()

const wallet = useWalletStore()

const open = ref(false)
const copied = ref(false)
const adding = ref(false)
const root = ref<HTMLElement | null>(null)

// Inline-rename state: index of the slot being renamed (or null) + draft label.
const renamingIndex = ref<number | null>(null)
const draft = ref('')

function labelFor(a: { index: number; label?: string }) {
  return a.label && a.label.trim() ? a.label : `Account ${a.index}`
}

const activeAccount = computed(() => wallet.accounts.find((a) => a.index === wallet.activeIndex))
const activeLabel = computed(() => (activeAccount.value ? labelFor(activeAccount.value) : 'Account'))
const activeAddr = computed(() => activeAccount.value?.address ?? '')

function toggle() {
  open.value = !open.value
}

function pick(index: number) {
  wallet.select(index)
  open.value = false
}

async function addAccount() {
  if (adding.value) return
  adding.value = true
  try {
    await wallet.addAccount() // keeps the dropdown open; the new slot appears at the bottom
  } catch {
    /* at the cap or not connected */
  } finally {
    adding.value = false
  }
}

async function copyAddr() {
  if (!activeAddr.value) return
  try {
    await ClipboardSetText(activeAddr.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    /* clipboard unavailable */
  }
}

function startRename(a: AccountInfo) {
  renamingIndex.value = a.index
  draft.value = a.label ?? ''
}

function cancelRename() {
  renamingIndex.value = null
  draft.value = ''
}

async function saveRename(a: AccountInfo) {
  await wallet.setLabel(a.index, draft.value.trim())
  cancelRename()
}

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) {
    open.value = false
    cancelRename()
  }
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div ref="root" class="relative">
    <!-- Pill entry point (TopBar): compact address pill that toggles the same dropdown -->
    <template v-if="props.variant === 'pill'">
      <button
        type="button"
        class="flex h-8.5 items-center gap-2 rounded-md border border-border bg-card px-3 transition-colors hover:border-muted-foreground/40"
        aria-label="Select account"
        :aria-expanded="open"
        @click="toggle"
      >
        <WalletIcon :size="15" class="text-muted-foreground" />
        <span class="font-mono text-xs">{{ shortAddress(activeAddr) }}</span>
        <ChevronDownIcon :size="14" class="text-muted-foreground" :class="open ? 'rotate-180' : ''" />
      </button>
    </template>

    <template v-else>
      <!-- Name + chevron (expands the slot list) -->
      <button
        type="button"
        class="flex items-center gap-1.5"
        aria-label="Select account"
        :aria-expanded="open"
        @click="toggle"
      >
        <span class="text-lg font-semibold leading-tight text-foreground">{{ activeLabel }}</span>
        <ChevronDownIcon
          :size="16"
          class="flex-none text-muted-foreground transition-transform"
          :class="open ? 'rotate-180 text-foreground' : ''"
        />
      </button>

      <!-- Address + copy -->
      <div class="mt-0.5 flex items-center gap-1.5">
        <span class="font-mono text-xs text-muted-foreground">{{ shortAddress(activeAddr) }}</span>
        <button
          type="button"
          class="text-muted-foreground transition-colors hover:text-foreground"
          :aria-label="copied ? 'address copied' : 'copy address'"
          :title="copied ? 'Copied' : 'Copy address'"
          @click="copyAddr"
        >
          <CopyIcon v-if="!copied" :size="13" />
          <CheckIcon v-else :size="13" class="text-primary" />
        </button>
      </div>
    </template>

    <!-- Slot dropdown -->
    <div
      v-if="open"
      class="absolute left-0 top-full z-20 mt-2 w-72 overflow-hidden rounded-lg border border-border bg-card shadow-lg"
    >
      <div class="max-h-80 overflow-y-auto" role="listbox">
      <template v-for="a in wallet.accounts" :key="a.index">
        <!-- Inline-rename row -->
        <div
          v-if="renamingIndex === a.index"
          class="flex items-center gap-3 border-t border-border bg-info/[0.07] px-3 py-2.5 first:border-t-0"
        >
          <div class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg bg-sidebar-accent text-sm font-bold text-foreground">{{ a.index }}</div>
          <input
            v-model="draft"
            :aria-label="`Rename account ${a.index}`"
            class="flex-1 rounded-lg border border-info bg-background px-2 py-1.5 text-sm font-semibold text-foreground outline-none"
            @keyup.enter="saveRename(a)"
            @keyup.esc="cancelRename"
            @click.stop
          />
          <button type="button" class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-primary/40 text-primary" :aria-label="`Save account ${a.index}`" @click.stop="saveRename(a)"><CheckIcon :size="14" /></button>
          <button type="button" class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-border text-muted-foreground" :aria-label="`Cancel rename account ${a.index}`" @click.stop="cancelRename"><XIcon :size="14" /></button>
        </div>

        <!-- Selectable row -->
        <div
          v-else
          role="option"
          :aria-selected="a.index === wallet.activeIndex"
          class="group flex cursor-pointer items-center gap-3 border-t border-border px-3 py-2.5 first:border-t-0 hover:bg-foreground/[0.06]"
          :class="a.index === wallet.activeIndex ? 'bg-primary/[0.08]' : ''"
          @click="pick(a.index)"
        >
          <div class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg bg-sidebar-accent text-sm font-bold text-foreground">{{ a.index }}</div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-semibold leading-tight text-foreground">{{ labelFor(a) }}</div>
            <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(a.address) }}</div>
          </div>
          <span class="flex-none text-primary" :class="a.index === wallet.activeIndex ? 'opacity-100' : 'opacity-0'"><CheckIcon :size="15" /></span>
          <button type="button" title="Rename" :aria-label="`Rename account ${a.index}`" class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-transparent text-muted-foreground group-hover:border-border group-hover:text-foreground" @click.stop="startRename(a)"><PencilIcon :size="13" /></button>
        </div>
      </template>
      </div>

      <!-- Add another account (derivation index) -->
      <button
        type="button"
        :disabled="adding"
        aria-label="add account"
        class="flex w-full items-center gap-3 border-t border-border px-3 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-foreground/[0.06] disabled:opacity-50"
        @click.stop="addAccount"
      >
        <span class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg border border-dashed border-primary/50 text-primary">
          <PlusIcon :size="15" />
        </span>
        {{ adding ? 'Adding…' : 'Add account' }}
      </button>
    </div>
  </div>
</template>
