<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import QRCode from 'qrcode'
import logoUrl from '../assets/images/syrius-logo.png'

const props = defineProps<{ address: string }>()

const dataUrl = ref('')
const copied = ref(false)
let reqId = 0

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = reject
    img.src = src
  })
}

async function renderQR(addr: string) {
  const id = ++reqId
  if (!addr) {
    dataUrl.value = ''
    return
  }
  try {
    // Render to a canvas so we can composite the logo. Error correction 'H'
    // tolerates the centred logo; brand-green modules on a near-black field stay
    // high-contrast for scanning while looking on-brand.
    const canvas = document.createElement('canvas')
    await QRCode.toCanvas(canvas, addr, {
      errorCorrectionLevel: 'H',
      margin: 1,
      width: 240,
      color: { dark: '#00d659', light: '#0d0d0d' },
    })
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const logo = await loadImage(logoUrl)
      if (id !== reqId) return
      const s = Math.round(canvas.width * 0.26)
      const x = (canvas.width - s) / 2
      const y = (canvas.height - s) / 2
      const pad = Math.round(s * 0.1)
      const r = Math.round(s * 0.22)
      // Dark rounded patch behind the logo so modules don't bleed under it.
      ctx.fillStyle = '#0d0d0d'
      ctx.beginPath()
      ctx.roundRect(x - pad, y - pad, s + pad * 2, s + pad * 2, r)
      ctx.fill()
      ctx.drawImage(logo, x, y, s, s)
    }
    const u = canvas.toDataURL('image/png')
    if (id === reqId) dataUrl.value = u
  } catch {
    if (id === reqId) dataUrl.value = ''
  }
}

async function copy() {
  try {
    await navigator.clipboard?.writeText(props.address)
    copied.value = true
    window.setTimeout(() => (copied.value = false), 1200)
  } catch {
    /* clipboard unavailable; ignore */
  }
}

onMounted(() => renderQR(props.address))
watch(
  () => props.address,
  (a) => renderQR(a),
)
</script>

<template>
  <div class="flex flex-col items-center gap-4 rounded bg-card p-4">
    <img
      v-if="dataUrl"
      :src="dataUrl"
      alt="address QR"
      class="h-52 w-52 rounded-lg border border-border"
    />
    <!-- Full address on a single line (never wrapped) + copy. -->
    <div class="flex w-full items-center justify-center gap-2">
      <code class="whitespace-nowrap font-mono text-sm text-foreground">{{ props.address }}</code>
      <button
        type="button"
        :aria-label="`copy address ${props.address}`"
        title="Copy address"
        class="grid h-7 w-7 shrink-0 place-items-center rounded-md border border-border text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
        @click="copy"
      >
        <svg v-if="copied" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="13" height="13" x="9" y="9" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
      </button>
    </div>
  </div>
</template>
