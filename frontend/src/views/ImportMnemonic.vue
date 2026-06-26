<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'

const wallet = useWalletStore()
const router = useRouter()
const mnemonic = ref('')
const name = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')

const wordCount = computed(() => mnemonic.value.trim().split(/\s+/).filter(Boolean).length)
const looksValid = computed(() => wordCount.value === 12 || wordCount.value === 24)
const canImport = computed(
  () => looksValid.value && name.value.trim() !== '' && password.value.length > 0 && password.value === confirm.value,
)

async function doImport() {
  error.value = ''
  try {
    // `name` is now a display name; the backend assigns a uuid keystore filename.
    // Capture the returned meta and unlock by its real id.
    const meta = await wallet.importMnemonic(name.value.trim(), password.value, mnemonic.value.trim())
    await wallet.unlock(meta.id, password.value)
    router.push('/home')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    password.value = ''
    confirm.value = ''
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="relative w-[32rem]">
      <button
        class="absolute right-4 top-4 text-muted-foreground transition-colors hover:text-foreground"
        aria-label="close"
        @click="router.push('/unlock')">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
      </button>
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Import from mnemonic</h1>
        <textarea
          v-model="mnemonic"
          rows="3"
          placeholder="word1 word2 …"
          aria-label="mnemonic"
          class="w-full rounded border border-border bg-background p-3 font-mono text-sm text-foreground"></textarea>
        <p v-if="mnemonic && !looksValid" class="text-xs text-destructive">Expected 12 or 24 words ({{ wordCount }})</p>
        <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
        <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
        <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
        <Button class="w-full" :disabled="!canImport" aria-label="Import" @click="doImport">Import</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
