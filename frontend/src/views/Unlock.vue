<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'

const wallet = useWalletStore()
const password = ref('')
const error = ref('')

onMounted(() => wallet.loadWallets())

async function submit() {
  error.value = ''
  try {
    await wallet.unlock(wallet.active, password.value)
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="w-80">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-lg text-foreground">Unlock {{ wallet.active || 'wallet' }}</h1>
        <Input
          v-model="password"
          type="password"
          placeholder="Password"
          aria-label="password"
          @keyup.enter="submit"
        />
        <Button class="w-full" @click="submit">Unlock</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
