<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { shortAddress } from '../lib/format'
import { useWalletStore, type WalletMeta } from '../stores/wallet'

const props = defineProps<{ modelValue: string; wallets: WalletMeta[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', id: string): void }>()

const wallet = useWalletStore()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

// Inline-rename state: id of the wallet being renamed (or null) + its draft text.
const renamingId = ref<string | null>(null)
const draft = ref('')

const selected = computed(() => props.wallets.find((w) => w.id === props.modelValue) ?? props.wallets[0])

function avatarLetter(name: string): string {
  return (name?.trim()?.[0] ?? '?').toUpperCase()
}

function toggle() {
  open.value = !open.value
}

function pick(id: string) {
  emit('update:modelValue', id)
  open.value = false
}

function startRename(w: WalletMeta) {
  renamingId.value = w.id
  draft.value = w.name
}

function cancelRename() {
  renamingId.value = null
  draft.value = ''
}

async function saveRename(w: WalletMeta) {
  const name = draft.value.trim()
  if (!name || name === w.name) {
    cancelRename()
    return
  }
  await wallet.rename(w.id, name)
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
  <div ref="root" class="relative" :class="{ 'is-open': open }">
    <!-- Collapsed trigger -->
    <button
      type="button"
      aria-label="Select wallet"
      :aria-expanded="open"
      class="flex w-full items-center gap-3 rounded-[10px] border border-border bg-background px-3 py-[11px] text-left transition-colors hover:border-muted-foreground/40"
      :class="open ? 'rounded-b-none border-muted-foreground/40' : ''"
      @click="toggle"
    >
      <template v-if="selected">
        <div class="flex h-[30px] w-[30px] flex-none place-items-center rounded-lg bg-gradient-to-br from-primary to-info text-[13px] font-bold text-primary-foreground grid">
          {{ avatarLetter(selected.name) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-[15px] font-semibold leading-tight text-foreground">{{ selected.name }}</div>
          <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(selected.baseAddress) }}</div>
        </div>
      </template>
      <span v-else class="flex-1 text-[15px] text-muted-foreground">No wallets</span>
      <svg
        class="h-[18px] w-[18px] flex-none text-muted-foreground transition-transform"
        :class="open ? 'rotate-180 text-foreground' : ''"
        viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
      >
        <path d="M6 9l6 6 6-6" />
      </svg>
    </button>

    <!-- Expanded panel -->
    <div
      v-if="open"
      class="absolute z-10 w-full overflow-hidden rounded-b-[10px] border border-t-0 border-muted-foreground/40 bg-muted"
      role="listbox"
    >
      <template v-for="w in wallets" :key="w.id">
        <!-- Inline-rename row -->
        <div
          v-if="renamingId === w.id"
          class="flex items-center gap-3 border-t border-border bg-info/[0.07] px-3 py-2.5 first:border-t-0"
        >
          <div class="flex h-[30px] w-[30px] flex-none place-items-center rounded-lg bg-gradient-to-br from-primary to-info text-[13px] font-bold text-primary-foreground grid">
            {{ avatarLetter(draft || w.name) }}
          </div>
          <input
            v-model="draft"
            :aria-label="`Rename ${w.name}`"
            class="flex-1 rounded-[7px] border border-info bg-background px-2 py-[5px] text-sm font-semibold text-foreground outline-none"
            @keyup.enter="saveRename(w)"
            @keyup.esc="cancelRename"
            @click.stop
          />
          <button
            type="button"
            :aria-label="`Save ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-primary/40 text-[13px] text-primary"
            @click.stop="saveRename(w)"
          >✓</button>
          <button
            type="button"
            :aria-label="`Cancel rename ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-border text-[13px] text-muted-foreground"
            @click.stop="cancelRename"
          >✕</button>
        </div>

        <!-- Selectable row -->
        <div
          v-else
          role="option"
          :aria-selected="w.id === modelValue"
          class="group flex cursor-pointer items-center gap-3 border-t border-border px-3 py-2.5 first:border-t-0 hover:bg-foreground/[0.06]"
          :class="w.id === modelValue ? 'bg-primary/[0.08]' : ''"
          @click="pick(w.id)"
        >
          <div class="flex h-[30px] w-[30px] flex-none place-items-center rounded-lg bg-gradient-to-br from-primary to-info text-[13px] font-bold text-primary-foreground grid">
            {{ avatarLetter(w.name) }}
          </div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-[15px] font-semibold leading-tight text-foreground">{{ w.name }}</div>
            <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(w.baseAddress) }}</div>
          </div>
          <span class="flex-none text-[15px] text-primary" :class="w.id === modelValue ? 'opacity-100' : 'opacity-0'">✓</span>
          <button
            type="button"
            title="Rename"
            :aria-label="`Rename ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-[7px] border border-transparent text-[13px] text-muted-foreground group-hover:border-border group-hover:text-foreground"
            @click.stop="startRename(w)"
          >✎</button>
        </div>
      </template>
    </div>
  </div>
</template>
