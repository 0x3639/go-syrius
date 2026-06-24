<script lang="ts">
  import { node } from '../stores/node'
  import { balances } from '../stores/balances'
  import { plasmaInfo } from '../stores/plasma'
  import { delegation } from '../stores/pillar'

  // No canonical plasma-level mapping exists in the codebase (Plasma.svelte
  // only renders the raw currentPlasma value), so we use the brief thresholds.
  function plasmaLevel(p: number): string {
    if (p >= 84000) return 'High'
    if (p >= 21000) return 'Medium'
    if (p > 0) return 'Low'
    return 'None'
  }
  $: pillarName = $delegation && $delegation.name ? $delegation.name : 'None'
</script>

<div
  class="flex flex-wrap items-center gap-x-6 gap-y-1 rounded border border-border bg-surface px-4 py-2 text-sm text-muted"
>
  <span>Account Height: <span class="font-medium text-text">{$node.height}</span></span>
  <span>Tokens: <span class="font-medium text-text">{$balances.length}</span></span>
  <span>Plasma: <span class="font-medium text-accent">⚡ {plasmaLevel($plasmaInfo?.currentPlasma ?? 0)}</span></span>
  <span>Pillar: <span class="font-medium text-text">{pillarName}</span></span>
</div>
