<script lang="ts">
  import { node, sync } from '../stores/node'
</script>
<div class="flex items-center gap-3 text-xs text-muted">
  <span class="inline-block h-2 w-2 rounded-full {$node.connected ? 'bg-success' : 'bg-error'}"></span>
  <span>{$node.connected ? 'Connected' : 'Disconnected'}</span>
  <span>height {$node.height}</span>
  {#if $node.peers}<span>{$node.peers} peers</span>{/if}
  {#if $node.mode === 'embedded' && $sync}
    <span>Embedded · {$sync.state === 'synced' ? 'synced' : `syncing ${$sync.percent.toFixed(1)}%`}</span>
  {/if}
</div>
