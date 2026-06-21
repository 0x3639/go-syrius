<script lang="ts">
  import { tx } from '../stores/tx'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'
  let copied = false
  async function copy() { await ClipboardSetText($tx.hash); copied = true; setTimeout(() => (copied = false), 1200) }
</script>
<div class="rounded border border-success/40 bg-surface p-4 space-y-2">
  <p class="text-success">Transaction published</p>
  <div class="font-mono text-xs break-all">{$tx.hash}</div>
  <button class="rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={copy}>{copied ? 'Copied' : 'Copy hash'}</button>
</div>
