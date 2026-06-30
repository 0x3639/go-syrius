<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ChevronDownIcon, CheckIcon, XIcon, PencilIcon } from '@lucide/vue'
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
      class="flex w-full items-center gap-3 rounded-xl border border-border bg-background px-3 py-3 text-left transition-colors hover:border-muted-foreground/40"
      :class="open ? 'rounded-b-none border-muted-foreground/40' : ''"
      @click="toggle"
    >
      <template v-if="selected">
        <div class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg bg-sidebar-accent text-sm font-bold text-foreground">
          {{ avatarLetter(selected.name) }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-base font-semibold leading-tight text-foreground">{{ selected.name }}</div>
          <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(selected.baseAddress) }}</div>
        </div>
      </template>
      <span v-else class="flex-1 text-base text-muted-foreground">No wallets</span>
      <ChevronDownIcon
        :size="18"
        class="flex-none text-muted-foreground transition-transform"
        :class="open ? 'rotate-180 text-foreground' : ''"
      />
    </button>

    <!-- Expanded panel -->
    <div
      v-if="open"
      class="absolute z-10 w-full overflow-hidden rounded-b-xl border border-t-0 border-muted-foreground/40 bg-muted"
      role="listbox"
    >
      <template v-for="w in wallets" :key="w.id">
        <!-- Inline-rename row -->
        <div
          v-if="renamingId === w.id"
          class="flex items-center gap-3 border-t border-border bg-info/[0.07] px-3 py-2.5 first:border-t-0"
        >
          <div class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg bg-sidebar-accent text-sm font-bold text-foreground">
            {{ avatarLetter(draft || w.name) }}
          </div>
          <input
            v-model="draft"
            :aria-label="`Rename ${w.name}`"
            class="flex-1 rounded-lg border border-info bg-background px-2 py-1.5 text-sm font-semibold text-foreground outline-none"
            @keyup.enter="saveRename(w)"
            @keyup.esc="cancelRename"
            @click.stop
          />
          <button
            type="button"
            :aria-label="`Save ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-primary/40 text-primary"
            @click.stop="saveRename(w)"
          ><CheckIcon :size="14" /></button>
          <button
            type="button"
            :aria-label="`Cancel rename ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-border text-muted-foreground"
            @click.stop="cancelRename"
          ><XIcon :size="14" /></button>
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
          <div class="grid h-7.5 w-7.5 flex-none place-items-center rounded-lg bg-sidebar-accent text-sm font-bold text-foreground">
            {{ avatarLetter(w.name) }}
          </div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-base font-semibold leading-tight text-foreground">{{ w.name }}</div>
            <div class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{{ shortAddress(w.baseAddress) }}</div>
          </div>
          <span class="flex-none text-primary" :class="w.id === modelValue ? 'opacity-100' : 'opacity-0'"><CheckIcon :size="15" /></span>
          <button
            type="button"
            title="Rename"
            :aria-label="`Rename ${w.name}`"
            class="grid h-7 w-7 flex-none place-items-center rounded-lg border border-transparent text-muted-foreground group-hover:border-border group-hover:text-foreground"
            @click.stop="startRename(w)"
          ><PencilIcon :size="13" /></button>
        </div>
      </template>
    </div>
  </div>
</template>
