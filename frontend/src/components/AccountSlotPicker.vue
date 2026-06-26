<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { shortAddress } from '../lib/format'
import { useWalletStore, type AccountInfo } from '../stores/wallet'
import { ClipboardSetText } from '../../wailsjs/runtime/runtime'

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
    <!-- Name + chevron (expands the slot list) -->
    <button
      type="button"
      class="flex items-center gap-1.5"
      aria-label="Select account"
      :aria-expanded="open"
      @click="toggle"
    >
      <span class="text-lg font-semibold leading-tight text-foreground">{{ activeLabel }}</span>
      <svg
        class="h-4 w-4 flex-none text-muted-foreground transition-transform"
        :class="open ? 'rotate-180 text-foreground' : ''"
        viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
      ><path d="M6 9l6 6 6-6" /></svg>
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
        <svg v-if="!copied" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="13" height="13" x="9" y="9" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
        <svg v-else class="text-primary" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
      </button>
    </div>

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
          <div class="grid h-[30px] w-[30px] flex-none place-items-center rounded-lg bg-gradient-to-br from-primary to-info text-[13px] font-bold text-primary-foreground">{{ a.index }}</div>
          <input
            v-model="draft"
            :aria-label="`Rename account ${a.index}`"
            class="flex-1 rounded-[7px] border border-info bg-background px-2 py-[5px] text-sm font-semibold text-foreground outline-none"
            @keyup.enter="saveRename(a)"
            @keyup.esc="cancelRename"
            @click.stop
          />
          <button type="button" class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-primary/40 text-[13px] text-primary" :aria-label="`Save account ${a.index}`" @click.stop="saveRename(a)">✓</button>
          <button type="button" class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-border text-[13px] text-muted-foreground" :aria-label="`Cancel rename account ${a.index}`" @click.stop="cancelRename">✕</button>
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
          <div class="grid h-[30px] w-[30px] flex-none place-items-center rounded-lg bg-gradient-to-br from-primary to-info text-[13px] font-bold text-primary-foreground">{{ a.index }}</div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-[15px] font-semibold leading-tight text-foreground">{{ labelFor(a) }}</div>
            <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(a.address) }}</div>
          </div>
          <span class="flex-none text-[15px] text-primary" :class="a.index === wallet.activeIndex ? 'opacity-100' : 'opacity-0'">✓</span>
          <button type="button" title="Rename" :aria-label="`Rename account ${a.index}`" class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-transparent text-[13px] text-muted-foreground group-hover:border-border group-hover:text-foreground" @click.stop="startRename(a)">✎</button>
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
        <span class="grid h-[30px] w-[30px] flex-none place-items-center rounded-lg border border-dashed border-primary/50 text-primary">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M12 5v14M5 12h14"/></svg>
        </span>
        {{ adding ? 'Adding…' : 'Add account' }}
      </button>
    </div>
  </div>
</template>
