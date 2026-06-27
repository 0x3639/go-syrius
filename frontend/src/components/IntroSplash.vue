<!-- src/components/IntroSplash.vue -->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// Single job: play the Zenon logo Lottie once on first launch, then tell the
// parent we're done — whether the animation completed or the user skipped.
const emit = defineEmits<{ done: [] }>()

// Freeze the final frame briefly so it reads, then cross-fade into the app.
// Skips (click/Esc) fade immediately with no hold.
const FREEZE_MS = 1000
const FADE_MS = 600

const container = ref<HTMLDivElement | null>(null)
const leaving = ref(false)
let anim: { destroy: () => void } | null = null
let finished = false

// Start the fade-out. The login screen is already mounted under the overlay, so
// fading the overlay's opacity to 0 cross-fades it in. We emit `done` (which
// unmounts us) only after the fade completes, so the unmount is invisible.
function startFade() {
  if (finished) return
  finished = true
  leaving.value = true
  window.setTimeout(() => emit('done'), FADE_MS)
}

// Natural end of the animation: hold the last frame, then fade.
function onComplete() {
  window.setTimeout(startFade, FREEZE_MS)
}

// User-initiated skip: fade now, no hold.
function skip() {
  startFade()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') skip()
}

onMounted(async () => {
  // Dynamic imports: lottie-web + the asset are code-split, so launches that
  // skip the splash never fetch them.
  const [{ default: lottie }, { default: animationData }] = await Promise.all([
    import('lottie-web'),
    import('../assets/zn-logo.json'),
  ])
  if (!container.value) return
  anim = lottie.loadAnimation({
    container: container.value,
    renderer: 'svg',
    loop: false,
    autoplay: true,
    animationData,
  }) as unknown as {
    addEventListener: (n: string, cb: () => void) => void
    destroy: () => void
  }
  ;(anim as unknown as { addEventListener: (n: string, cb: () => void) => void }).addEventListener(
    'complete',
    onComplete,
  )
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeydown)
  anim?.destroy()
})
</script>

<template>
  <div
    class="fixed inset-0 z-[9999] flex items-center justify-center bg-background transition-opacity"
    :class="leaving ? 'opacity-0' : 'opacity-100'"
    :style="{ transitionDuration: FADE_MS + 'ms' }"
    role="presentation"
    @click="skip"
  >
    <div ref="container" class="h-full max-h-[80vh] w-full max-w-[80vw]" />
  </div>
</template>
