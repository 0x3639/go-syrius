<script setup lang="ts">
import { computed } from 'vue'
import { useNodeStore } from '../stores/node'
import { useBalancesStore } from '../stores/balances'
import { usePlasmaStore } from '../stores/plasma'
import { usePillarStore } from '../stores/pillar'

const node = useNodeStore()
const balances = useBalancesStore()
const plasma = usePlasmaStore()
const pillar = usePillarStore()

// No canonical plasma-level mapping exists in the codebase, so we use the brief
// thresholds (verbatim from the merged Svelte StatusStrip).
function plasmaLevel(p: number): string {
  if (p >= 84000) return 'High'
  if (p >= 21000) return 'Medium'
  if (p > 0) return 'Low'
  return 'None'
}

const pillarName = computed(() =>
  pillar.delegation && pillar.delegation.name ? pillar.delegation.name : 'None',
)
const level = computed(() => plasmaLevel(plasma.info?.currentPlasma ?? 0))
</script>

<template>
  <div
    class="flex flex-wrap items-center gap-x-6 gap-y-1 rounded border border-border bg-card px-4 py-2 text-sm text-muted-foreground"
  >
    <span>Account Height: <span class="font-medium text-foreground">{{ node.height }}</span></span>
    <span>Tokens: <span class="font-medium text-foreground">{{ balances.items.length }}</span></span>
    <span>Plasma: <span class="font-medium text-primary">⚡ {{ level }}</span></span>
    <span>Pillar: <span class="font-medium text-foreground">{{ pillarName }}</span></span>
  </div>
</template>
