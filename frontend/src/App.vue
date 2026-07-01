<!-- src/App.vue -->
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme, Toaster } from 'nom-ui'
import * as N from '../wailsjs/go/app/NodeService'
import { useWalletStore } from './stores/wallet'
import IntroSplash from './components/IntroSplash.vue'

const { setTheme } = useTheme()
const router = useRouter()
const wallet = useWalletStore()

// Show the logo intro on every launch, unless the user disabled it in Settings
// (persisted in localStorage; read directly here since this runs at root mount,
// before any store init). lottie-web + the asset are dynamically imported by
// IntroSplash, so they only load while the splash is on screen.
const showIntro = ref(localStorage.getItem('syrius.splash') !== '0')
function dismissIntro() {
  showIntro.value = false
}

// When the wallet locks (from anywhere — the Lock button, or a backend-driven
// lock), leave the protected UI immediately. The router guard only runs on
// navigation, so locking while staying on a gated route would otherwise keep the
// loaded balances/history on screen.
watch(
  () => wallet.locked,
  (locked) => {
    if (locked && router.currentRoute.value.name !== 'unlock') router.push('/unlock')
  },
)

onMounted(async () => {
  setTheme?.('dark')
  try {
    await N.Connect()
  } catch {
    /* best-effort; screens work offline */
  }
})
</script>

<template>
  <RouterView />
  <Toaster />
  <IntroSplash v-if="showIntro" @done="dismissIntro" />
</template>
