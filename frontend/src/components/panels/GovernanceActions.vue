<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useGovernanceStore } from '../../stores/governance'
import { useTxStore } from '../../stores/tx'
import { actionStatusLabel, actionTypeLabel, isActionApproved } from '../../lib/governance'
import type { app } from '../../../wailsjs/go/models'

const gov = useGovernanceStore()
const tx = useTxStore()
const { actions, actionCount, actionPage, numActivePillars } = storeToRefs(gov)
const error = ref('')

const PAGE_SIZE = 20
const pageCount = computed(() => Math.max(1, Math.ceil(actionCount.value / PAGE_SIZE)))
const hasPrev = computed(() => actionPage.value > 0)
const hasNext = computed(() => actionPage.value + 1 < pageCount.value)

const FILTERS = ['All', 'Voting', 'Approved', 'Rejected', 'NoDecision'] as const
type Filter = (typeof FILTERS)[number]
const filter = ref<Filter>('All')
const expanded = ref<string | null>(null)

const filtered = computed(() =>
  (actions.value ?? []).filter((a) => {
    switch (filter.value) {
      case 'Voting': return a.status === 0
      case 'Approved': return a.status === 1
      case 'Rejected': return a.status === 2
      case 'NoDecision': return a.status === 3
      default: return true
    }
  }),
)

function executable(a: app.ActionDTO): boolean {
  return a.status === 1 && !a.executed
}
function passing(a: app.ActionDTO): boolean {
  return a.status === 0 && isActionApproved(a.votes, a, numActivePillars.value)
}
function toggle(id: string) {
  expanded.value = expanded.value === id ? null : id
}
function goPage(page: number) {
  expanded.value = null
  gov.loadActions(page)
}
async function execute(id: string) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareExecuteAction(id))
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="space-y-3 p-4">
    <div class="flex flex-wrap gap-1">
      <button
        v-for="f in FILTERS"
        :key="f"
        type="button"
        class="rounded-full border px-3 py-1 text-xs transition-colors"
        :class="filter === f ? 'border-primary bg-primary/15 text-primary' : 'border-border text-muted-foreground hover:text-foreground'"
        :aria-label="`filter ${f}`"
        :aria-pressed="filter === f"
        @click="filter = f"
      >{{ f }}</button>
    </div>

    <p v-if="filtered.length === 0" class="text-sm text-muted-foreground">No matching actions.</p>

    <div
      v-for="a in filtered"
      :key="a.id"
      class="space-y-1 rounded-lg border border-border bg-card p-3 text-sm"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium text-foreground">{{ a.name }}</span>
        <span class="text-xs text-muted-foreground">
          <span class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase">{{ actionTypeLabel(a.type) }}</span>
          {{ actionStatusLabel(a.status) }}
          <span v-if="passing(a)" class="text-primary"> · passing</span>
        </span>
      </div>
      <p class="text-xs text-muted-foreground">
        {{ a.votes.yes }} yes · {{ a.votes.no }} no · {{ a.votes.total }} votes (round {{ a.round + 1 }})
      </p>
      <Button
        variant="outline"
        class="px-2 py-1 text-xs"
        :aria-label="`details ${a.id}`"
        :aria-expanded="expanded === a.id"
        @click="toggle(a.id)"
      >{{ expanded === a.id ? 'Hide' : 'Details' }}</Button>
      <template v-if="expanded === a.id">
        <p class="ml-1 mt-1 break-all text-xs text-muted-foreground">{{ a.description }}</p>
        <p class="ml-1 text-xs text-muted-foreground">Calls: {{ a.destination }}</p>
        <p class="ml-1 break-all text-xs text-muted-foreground">Data (base64): {{ a.data || '—' }}</p>
        <p class="ml-1 text-xs text-muted-foreground">
          Thresholds: {{ a.activePillarThreshold }}% quorum · {{ a.directionalThreshold }}% directional
        </p>
        <Button
          v-if="executable(a)"
          class="mt-1 px-2 py-1 text-xs"
          :aria-label="`execute ${a.id}`"
          @click="execute(a.id)"
        >Execute</Button>
      </template>
    </div>

    <div v-if="pageCount > 1" class="flex items-center justify-between gap-2 pt-1">
      <Button variant="outline" class="px-2 py-1 text-xs" :disabled="!hasPrev" aria-label="previous page" @click="goPage(actionPage - 1)">Prev</Button>
      <span class="text-xs text-muted-foreground">Page {{ actionPage + 1 }} of {{ pageCount }} · {{ actionCount }} actions</span>
      <Button variant="outline" class="px-2 py-1 text-xs" :disabled="!hasNext" aria-label="next page" @click="goPage(actionPage + 1)">Next</Button>
    </div>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
