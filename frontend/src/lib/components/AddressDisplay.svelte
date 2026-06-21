<script lang="ts">
  import QRCode from 'qrcode'
  import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'
  export let address = ''
  let dataUrl = ''
  let reqId = 0
  let copied = false
  $: void renderQR(address)
  async function renderQR(addr: string) {
    const id = ++reqId
    if (!addr) { dataUrl = ''; return }
    try {
      const u = await QRCode.toDataURL(addr, { margin: 1, width: 160 })
      if (id === reqId) dataUrl = u
    } catch {
      if (id === reqId) dataUrl = ''
    }
  }
  async function copy() { await ClipboardSetText(address); copied = true; setTimeout(() => (copied = false), 1200) }
</script>
<div class="flex items-center gap-4 rounded bg-surface p-4">
  {#if dataUrl}<img src={dataUrl} alt="address QR" class="h-32 w-32 rounded bg-white p-1" />{/if}
  <div class="min-w-0">
    <div class="break-all font-mono text-sm text-text">{address}</div>
    <button class="mt-2 rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={copy}>{copied ? 'Copied' : 'Copy'}</button>
  </div>
</div>
