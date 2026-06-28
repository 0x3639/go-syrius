<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { Button } from 'nom-ui'
import { useAcceleratorStore } from '../../stores/accelerator'
import { formatAmount } from '../../lib/format'
import { statusLabel, isPassing } from '../../lib/accelerator'
import type { app } from '../../../wailsjs/go/models'

const acc = useAcceleratorStore()
const { projects, numActivePillars, projectCount, projectPage } = storeToRefs(acc)

const PAGE_SIZE = 20
const pageCount = computed(() => Math.max(1, Math.ceil(projectCount.value / PAGE_SIZE)))
const hasPrev = computed(() => projectPage.value > 0)
const hasNext = computed(() => projectPage.value + 1 < pageCount.value)
function goPage(page: number) {
  expanded.value = null
  acc.loadProjects(page)
}

const FILTERS = ['All', 'Voting', 'Active', 'Awaiting payout', 'Completed', 'Closed'] as const
type Filter = (typeof FILTERS)[number]
const filter = ref<Filter>('All')
const expanded = ref<string | null>(null)

function currentPhase(p: app.ProjectDTO): app.PhaseDTO | null {
  return p.phases && p.phases.length ? p.phases[p.phases.length - 1] : null
}
function phasePassing(ph: app.PhaseDTO): boolean {
  return ph.status === 0 && isPassing(ph.votes.yes, ph.votes.no, ph.votes.total, numActivePillars.value)
}
function awaitingPayout(p: app.ProjectDTO): boolean {
  const ph = currentPhase(p)
  return !!ph && phasePassing(ph)
}
const filtered = computed(() =>
  (projects.value ?? []).filter((p) => {
    switch (filter.value) {
      case 'Voting': return p.status === 0
      case 'Active': return p.status === 1
      case 'Completed': return p.status === 4
      case 'Closed': return p.status === 3
      case 'Awaiting payout': return awaitingPayout(p)
      default: return true
    }
  }),
)
function toggle(id: string) {
  expanded.value = expanded.value === id ? null : id
}
function label(f: Filter): string {
  return f === 'Voting' ? 'Active AZs' : f
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
      >{{ label(f) }}</button>
    </div>

    <p v-if="filtered.length === 0" class="text-sm text-muted-foreground">No matching projects.</p>

    <div
      v-for="p in filtered"
      :key="p.id"
      class="space-y-1 rounded-lg border border-border bg-card p-3 text-sm"
    >
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium text-foreground">{{ p.name }}</span>
        <span class="text-xs text-muted-foreground"
          >{{ statusLabel(p.status) }}<span v-if="awaitingPayout(p)" class="text-primary"> · awaiting payout</span></span
        >
      </div>
      <p class="text-xs text-muted-foreground">
        {{ formatAmount(p.znnFundsNeeded, 8) }} ZNN / {{ formatAmount(p.qsrFundsNeeded, 8) }} QSR ·
        {{ p.votes.yes }}/{{ p.votes.no }}/{{ p.votes.total }}
      </p>
      <Button variant="outline" class="px-2 py-1 text-xs" :aria-label="`phases ${p.name}`" @click="toggle(p.id)">
        {{ expanded === p.id ? 'Hide phases' : 'Phases' }}
      </Button>
      <template v-if="expanded === p.id">
        <div v-for="ph in p.phases" :key="ph.id" class="ml-3 mt-1 text-xs text-muted-foreground">
          {{ ph.name }} · {{ statusLabel(ph.status) }} · {{ ph.votes.yes }}/{{ ph.votes.no }}/{{ ph.votes.total }}
          <span v-if="phasePassing(ph)" class="text-primary"> · awaiting payout</span>
        </div>
        <p v-if="!p.phases || p.phases.length === 0" class="ml-3 mt-1 text-xs text-muted-foreground">No phases.</p>
      </template>
    </div>

    <div v-if="pageCount > 1" class="flex items-center justify-between gap-2 pt-1">
      <Button
        variant="outline"
        class="px-2 py-1 text-xs"
        :disabled="!hasPrev"
        aria-label="previous page"
        @click="goPage(projectPage - 1)"
      >Prev</Button>
      <span class="text-xs text-muted-foreground">
        Page {{ projectPage + 1 }} of {{ pageCount }} · {{ projectCount }} projects
      </span>
      <Button
        variant="outline"
        class="px-2 py-1 text-xs"
        :disabled="!hasNext"
        aria-label="next page"
        @click="goPage(projectPage + 1)"
      >Next</Button>
    </div>
  </div>
</template>
