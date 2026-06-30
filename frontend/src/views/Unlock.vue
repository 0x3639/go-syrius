<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import WalletPicker from '../components/WalletPicker.vue'
import TopBar from '../components/TopBar.vue'
import logoUrl from '../assets/images/syrius-logo.png'

const wallet = useWalletStore()
const router = useRouter()
const selected = ref('')
const password = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)

onMounted(async () => {
  await wallet.loadWallets()
  if (!selected.value && wallet.wallets[0]) selected.value = wallet.wallets[0].id
})

async function doUnlock() {
  error.value = ''
  busy.value = true
  try {
    await wallet.unlock(selected.value, password.value)
    router.push('/dashboard')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    busy.value = false
    password.value = ''
  }
}

async function doImport() {
  error.value = ''
  notice.value = ''
  try {
    const path = await wallet.pickKeystoreFile()
    if (!path) return
    const existing = new Set(wallet.wallets.map((w) => w.baseAddress))
    const meta = await wallet.importKeystore(path, '')
    selected.value = meta.id
    if (existing.has(meta.baseAddress)) {
      const other = wallet.wallets.find((w) => w.baseAddress === meta.baseAddress && w.id !== meta.id)
      notice.value = `Imported. Note: this wallet has the same address as ${other?.name ?? 'an existing wallet'}.`
    }
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
}
</script>

<template>
  <div class="flex min-h-screen flex-col bg-background">
    <!-- Locked chrome so the window matches the in-app shell instead of a bare
         centered card (mirrors how syrius keeps its top bar on first load). -->
    <TopBar locked />
    <main class="grid flex-1 place-items-center p-8">
    <div class="flex flex-col items-center gap-6">
      <img :src="logoUrl" alt="syrius" class="h-20 w-20 rounded-2xl" />
      <Card class="w-96">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Unlock wallet</h1>
        <p v-if="wallet.wallets.length === 0" class="text-muted-foreground">
          No wallets yet. Import a keystore to begin.
        </p>
        <template v-else>
          <WalletPicker v-model="selected" :wallets="wallet.wallets" />
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" @keyup.enter="doUnlock" />
          <Button class="w-full" :disabled="busy || !selected" aria-label="Unlock" @click="doUnlock">Unlock</Button>
        </template>
        <Button variant="outline" class="w-full" @click="doImport">Import keystore…</Button>
        <Button variant="outline" class="w-full" @click="router.push('/create')">Create new wallet</Button>
        <Button variant="outline" class="w-full" @click="router.push('/import')">Import mnemonic</Button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
        <p v-if="notice" class="text-sm text-muted-foreground">{{ notice }}</p>
      </CardContent>
      </Card>
    </div>
    </main>
  </div>
</template>
