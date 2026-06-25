<!-- src/App.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useTheme } from 'nom-ui'
import * as N from '../wailsjs/go/app/NodeService'
const { setTheme } = useTheme()
onMounted(async () => {
  setTheme?.('dark')
  try { await N.Connect() } catch { /* best-effort; screens work offline */ }
})
</script>

<template>
  <RouterView />
</template>
