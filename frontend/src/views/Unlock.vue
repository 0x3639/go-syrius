<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import logoUrl from '../assets/images/syrius-logo.png'

const wallet = useWalletStore()
const router = useRouter()
const selected = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

onMounted(async () => {
  await wallet.loadWallets()
  if (!selected.value && wallet.wallets[0]) selected.value = wallet.wallets[0]
})

async function doUnlock() {
  error.value = ''
  busy.value = true
  try {
    await wallet.unlock(selected.value, password.value)
    router.push('/home')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    busy.value = false
    password.value = ''
  }
}

async function doImport() {
  error.value = ''
  try {
    const path = await wallet.pickKeystoreFile()
    if (!path) return
    await wallet.importKeystore(path)
    if (!selected.value && wallet.wallets[0]) selected.value = wallet.wallets[0]
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
}
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <div class="flex flex-col items-center gap-6">
      <img :src="logoUrl" alt="syrius" class="h-20 w-20 rounded-2xl" />
      <Card class="w-96">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Unlock wallet</h1>
        <p v-if="wallet.wallets.length === 0" class="text-muted-foreground">
          No wallets yet. Import a keystore to begin.
        </p>
        <template v-else>
          <select
            v-model="selected"
            aria-label="wallet"
            class="w-full rounded border border-border bg-background px-3 py-2 text-foreground">
            <option v-for="w in wallet.wallets" :key="w" :value="w">{{ w }}</option>
          </select>
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" @keyup.enter="doUnlock" />
          <Button class="w-full" :disabled="busy || !selected" aria-label="Unlock" @click="doUnlock">Unlock</Button>
        </template>
        <Button variant="outline" class="w-full" @click="doImport">Import keystore…</Button>
        <Button variant="outline" class="w-full" @click="router.push('/create')">Create new wallet</Button>
        <Button variant="outline" class="w-full" @click="router.push('/import')">Import mnemonic</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
      </Card>
    </div>
  </main>
</template>
