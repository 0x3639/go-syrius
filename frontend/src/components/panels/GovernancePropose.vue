<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { proposeKinds } = storeToRefs(gov)
const error = ref('')

// governance metadata
const name = ref('')
const description = ref('')
const url = ref('')

// selected kind + its per-field values
const selectedKind = ref('')
const params = reactive<Record<string, string>>({})

watch(
  proposeKinds,
  (list) => {
    if (list.length && !list.some((k) => k.kind === selectedKind.value)) {
      selectedKind.value = list[0].kind
    }
  },
  { immediate: true },
)

const currentKind = computed<app.ProposeKindDTO | undefined>(() =>
  (proposeKinds.value ?? []).find((k) => k.kind === selectedKind.value),
)

// reset params when the kind changes so stale fields don't leak across kinds
watch(selectedKind, () => {
  for (const key of Object.keys(params)) delete params[key]
  for (const f of currentKind.value?.fields ?? []) params[f.key] = f.type === 'bool' ? 'false' : ''
})

function inputType(t: string): string {
  return t === 'number' ? 'number' : 'text'
}

// Inline length hint from the field's catalog min/max byte bounds (0 = unset).
function lengthHint(f: app.ProposeFieldDTO): string {
  if (f.min > 0 && f.max > 0) return `${f.min}–${f.max} characters`
  if (f.max > 0) return `up to ${f.max} characters`
  if (f.min > 0) return `at least ${f.min} characters`
  return ''
}

async function submit() {
  error.value = ''
  if (!currentKind.value) {
    error.value = 'Select an action kind.'
    return
  }
  const payload: Record<string, string> = {}
  for (const f of currentKind.value.fields) payload[f.key] = params[f.key] ?? ''
  try {
    tx.awaitConfirm(await Nom.PrepareProposeAction(name.value, description.value, url.value, selectedKind.value, payload))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <p class="text-xs text-muted-foreground">Proposing an action costs 1 ZNN (non-refundable).</p>

    <label class="block text-sm">
      <span class="text-muted-foreground">Action name</span>
      <input v-model="name" aria-label="action name" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>
    <label class="block text-sm">
      <span class="text-muted-foreground">Description</span>
      <input v-model="description" aria-label="action description" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>
    <label class="block text-sm">
      <span class="text-muted-foreground">URL</span>
      <input v-model="url" aria-label="action url" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary" />
    </label>

    <label class="block text-sm">
      <span class="text-muted-foreground">Action kind</span>
      <select v-model="selectedKind" aria-label="propose kind" class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary">
        <option v-for="k in proposeKinds" :key="k.kind" :value="k.kind">{{ k.group }} · {{ k.label }}</option>
      </select>
    </label>

    <template v-for="f in currentKind?.fields ?? []" :key="f.key">
      <label class="block text-sm">
        <span class="text-muted-foreground">{{ f.label }}<span v-if="f.required" class="text-destructive"> *</span></span>
        <label v-if="f.type === 'bool'" class="mt-1 flex items-center gap-2">
          <input
            type="checkbox"
            :aria-label="`field ${f.key}`"
            :checked="params[f.key] === 'true'"
            @change="params[f.key] = ($event.target as HTMLInputElement).checked ? 'true' : 'false'"
          />
          <span class="text-xs text-muted-foreground">{{ f.placeholder || 'enabled' }}</span>
        </label>
        <input
          v-else
          v-model="params[f.key]"
          :type="inputType(f.type)"
          :aria-label="`field ${f.key}`"
          :placeholder="f.placeholder"
          :maxlength="f.max > 0 ? f.max : undefined"
          class="mt-1 w-full rounded border border-border bg-muted px-2 py-1 text-foreground outline-none focus:ring-2 focus:ring-primary"
        />
        <span v-if="lengthHint(f)" class="text-[10px] text-muted-foreground">{{ lengthHint(f) }}</span>
        <span v-if="f.type === 'list'" class="text-[10px] text-muted-foreground">comma-separated</span>
      </label>
    </template>

    <Button aria-label="submit proposal" @click="submit">Propose (1 ZNN)</Button>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
