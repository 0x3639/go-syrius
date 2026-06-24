<!-- src/App.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme } from 'nom-ui'
import { useWalletStore } from './stores/wallet'
import Unlock from './views/Unlock.vue'
import Home from './views/Home.vue'

const wallet = useWalletStore()
const { setTheme } = useTheme()

onMounted(() => setTheme?.('dark'))
</script>

<template>
  <Unlock v-if="wallet.locked" />
  <Home v-else />
</template>
