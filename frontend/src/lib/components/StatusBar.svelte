<script lang="ts">
  import { node } from '../stores/node'
</script>
<div class="flex items-center gap-3 text-xs text-muted">
  <span class="inline-block h-2 w-2 rounded-full {$node.connected ? 'bg-success' : 'bg-error'}"></span>
  <span>{$node.connected ? 'Connected' : 'Disconnected'}</span>
  <span>height {$node.height}</span>
  {#if $node.peers}<span>{$node.peers} peers</span>{/if}
</div>
