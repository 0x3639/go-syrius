<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import QRCode from 'qrcode'
import logoUrl from '../assets/images/syrius-logo.png'
import { CheckIcon, CopyIcon } from '@lucide/vue'

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
      color: { dark: '#00d557', light: '#0d0d0d' },
    })
    const ctx = canvas.getContext('2d')
    if (ctx) {
      const logo = await loadImage(logoUrl)
      if (id !== reqId) return
      const s = Math.round(canvas.width * 0.28)
      const x = (canvas.width - s) / 2
      const y = (canvas.height - s) / 2
      const r = Math.round(s * 0.22)
      // Dark rounded patch behind the logo so modules don't bleed under it.
      ctx.fillStyle = '#0d0d0d'
      ctx.beginPath()
      ctx.roundRect(x, y, s, s, r)
      ctx.fill()
      // The icon PNG bakes generous dark padding around the star; draw it
      // zoomed and clipped to the patch so that padding is cropped rather than
      // stacked as an extra black ring. 1.25 keeps the star tips (~70% span)
      // safely inside the crop.
      const zs = s * 1.25
      ctx.save()
      ctx.beginPath()
      ctx.roundRect(x, y, s, s, r)
      ctx.clip()
      ctx.drawImage(logo, x - (zs - s) / 2, y - (zs - s) / 2, zs, zs)
      ctx.restore()
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
        <CheckIcon v-if="copied" :size="14" />
        <CopyIcon v-else :size="14" />
      </button>
    </div>
  </div>
</template>
