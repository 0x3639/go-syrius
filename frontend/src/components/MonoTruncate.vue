<script setup lang="ts">
// Width-aware middle truncation for monospace data (addresses / hashes). Measures
// the column the value is rendered in and collapses it to `start…end` — keeping
// BOTH ends and shortening them together as the column narrows (full when it
// fits → z1ffjdk…jfkd → z1ff…jfkd → z1…kd). Never truncates only the end.
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'

const props = withDefaults(defineProps<{ value?: string; minEnd?: number }>(), {
  value: '',
  minEnd: 4, // never show fewer than this many leading / trailing chars
})

const root = ref<HTMLElement | null>(null)
const display = ref(props.value ?? '')
let canvas: HTMLCanvasElement | null = null

function recompute() {
  const v = props.value ?? ''
  const el = root.value
  // No element / no layout yet (e.g. jsdom) / too short to bother → show full.
  if (!el || v.length <= props.minEnd * 2 + 1) { display.value = v; return }
  const avail = el.clientWidth
  if (!avail) { display.value = v; return }
  try {
    canvas = canvas || document.createElement('canvas')
    const ctx = canvas.getContext('2d')
    if (!ctx) { display.value = v; return }
    const cs = getComputedStyle(el)
    ctx.font = `${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`
    const full = ctx.measureText(v).width
    if (full <= avail) { display.value = v; return } // it all fits — show full
    const charPx = full / v.length // monospace ⇒ uniform advance
    const maxChars = Math.floor(avail / charPx)
    // Reserve one char for the ellipsis; floor at minEnd*2+1; cap below full length.
    const keep = Math.min(Math.max(props.minEnd * 2 + 1, maxChars - 1), v.length - 1)
    const end = Math.max(props.minEnd, Math.floor(keep / 2))
    const start = Math.max(props.minEnd, keep - end)
    display.value = `${v.slice(0, start)}…${v.slice(v.length - end)}`
  } catch {
    display.value = v
  }
}

let ro: ResizeObserver | null = null
onMounted(() => {
  recompute()
  if (typeof ResizeObserver !== 'undefined' && root.value) {
    ro = new ResizeObserver(() => recompute())
    ro.observe(root.value)
  }
})
onBeforeUnmount(() => ro?.disconnect())
watch(() => props.value, recompute)
</script>

<template>
  <!-- title carries the full value so it's recoverable on hover even when truncated. -->
  <span
    ref="root"
    :title="value"
    class="block min-w-0 overflow-hidden whitespace-nowrap font-mono"
  >{{ display }}</span>
</template>
