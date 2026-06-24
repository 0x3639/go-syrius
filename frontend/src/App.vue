<!-- src/App.vue -->
<script setup lang="ts">
import { onMounted } from 'vue'
import { Button, Card, CardContent, useTheme } from 'nom-ui'

const { setTheme } = useTheme()
onMounted(() => setTheme('dark'))
</script>

<template>
  <main class="min-h-screen bg-background p-8">
    <Card>
      <CardContent class="space-y-3 p-6">
        <p class="text-foreground">nom-ui + Tailwind 4 OK</p>
        <Button>Primary</Button>
      </CardContent>
    </Card>
  </main>
</template>
