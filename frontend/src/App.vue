<!-- src/App.vue -->
<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme, Toaster } from 'nom-ui'
import * as N from '../wailsjs/go/app/NodeService'
import { useWalletStore } from './stores/wallet'

const { setTheme } = useTheme()
const router = useRouter()
const wallet = useWalletStore()

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
</template>
