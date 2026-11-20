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
// Status pill tone + vote bar widths (share of the votes cast so far).
function pillClass(p: app.ProjectDTO): string {
  if (p.status === 0) return 'bg-warning/15 text-warning'
  if (p.status === 1 || p.status === 4) return 'bg-success/15 text-success'
  return 'bg-foreground/10 text-muted-foreground'
}
function share(n: number, total: number): string {
  return total > 0 ? `${Math.round((n / total) * 100)}%` : '0%'
}
function toggle(id: string) {
  expanded.value = expanded.value === id ? null : id
}
function label(f: Filter): string {
  return f
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap gap-1.5" role="group" aria-label="Filter by status">
      <button
        v-for="f in FILTERS"
        :key="f"
        type="button"
        class="h-8.5 rounded-full px-3.5 text-xs font-semibold transition-colors"
        :class="filter === f ? 'bg-primary/15 text-primary' : 'bg-foreground/[0.07] text-foreground hover:bg-foreground/[0.12]'"
        :aria-label="`filter ${f}`"
        :aria-pressed="filter === f"
        @click="filter = f"
      >{{ label(f) }}</button>
    </div>

    <p v-if="filtered.length === 0" class="text-sm text-muted-foreground">No matching projects.</p>

    <article
      v-for="p in filtered"
      :key="p.id"
      class="panel-card space-y-3 text-sm"
    >
      <div class="flex flex-wrap items-center gap-x-8 gap-y-3">
        <div class="min-w-0 flex-[2] basis-64 space-y-1.5">
          <div class="flex flex-wrap items-center gap-2.5">
            <span class="text-base font-semibold text-foreground">{{ p.name }}</span>
            <span class="status-pill" :class="pillClass(p)"
              >{{ statusLabel(p.status) }}<span v-if="awaitingPayout(p)"> · awaiting payout</span></span
            >
          </div>
          <p class="text-xs text-muted-foreground">
            Funding <span class="font-mono text-foreground">{{ formatAmount(p.znnFundsNeeded, 8) }} ZNN</span> · <span class="font-mono text-foreground">{{ formatAmount(p.qsrFundsNeeded, 8) }} QSR</span>
          </p>
        </div>
        <div class="flex-1 basis-52 space-y-1.5">
          <div class="flex justify-between gap-3 text-xs">
            <span><span class="font-bold text-primary">{{ p.votes.yes }} yes</span> · <span class="text-foreground">{{ p.votes.no }} no</span></span>
            <span class="text-muted-foreground">{{ p.votes.total }} total votes</span>
          </div>
          <div class="flex h-2 overflow-hidden rounded-full bg-foreground/10" aria-hidden="true">
            <div class="bg-primary" :style="{ width: share(p.votes.yes, p.votes.total) }"></div>
            <div class="bg-foreground/70" :style="{ width: share(p.votes.no, p.votes.total) }"></div>
          </div>
        </div>
        <Button variant="outline" :aria-label="`phases ${p.name}`" @click="toggle(p.id)">
          {{ expanded === p.id ? 'Hide phases' : 'Phases' }}
        </Button>
      </div>
      <div v-if="expanded === p.id" class="space-y-1.5 border-t border-border pt-3">
        <div v-for="ph in p.phases" :key="ph.id" class="text-xs text-muted-foreground">
          <span class="font-semibold text-foreground">{{ ph.name }}</span> · {{ statusLabel(ph.status) }} · {{ ph.votes.yes }} yes · {{ ph.votes.no }} no · {{ ph.votes.total }} total votes
          <span v-if="phasePassing(ph)" class="text-primary"> · awaiting payout</span>
        </div>
        <p v-if="!p.phases || p.phases.length === 0" class="text-xs text-muted-foreground">No phases.</p>
      </div>
    </article>

    <div v-if="pageCount > 1" class="flex items-center justify-between gap-2 pt-1">
      <Button
        variant="outline"
        :disabled="!hasPrev"
        aria-label="previous page"
        @click="goPage(projectPage - 1)"
      >Prev</Button>
      <span class="text-xs text-muted-foreground">
        Page {{ projectPage + 1 }} of {{ pageCount }} · {{ projectCount }} projects
      </span>
      <Button
        variant="outline"
        :disabled="!hasNext"
        aria-label="next page"
        @click="goPage(projectPage + 1)"
      >Next</Button>
    </div>
  </div>
</template>
