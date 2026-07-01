<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import WalletPicker from '../components/WalletPicker.vue'
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
  <!-- Shell-less lock screen with a soft radial plasma halo. -->
  <div
    class="grid min-h-screen place-items-center bg-background p-8"
    style="background-image: radial-gradient(circle at 50% 30%, rgba(0,213,87,.10), transparent 60%);"
  >
    <div class="flex w-[21rem] flex-col items-center gap-5">
      <img :src="logoUrl" alt="go-syrius" class="h-16 w-16 rounded-2xl" />
      <div class="text-center">
        <div class="text-xl font-bold tracking-tight text-foreground">Welcome back</div>
        <div class="mt-1 text-sm text-muted-foreground">Unlock your go-syrius wallet</div>
      </div>

      <template v-if="wallet.wallets.length > 0">
        <WalletPicker v-model="selected" :wallets="wallet.wallets" class="w-full" />
        <Input v-model="password" type="password" placeholder="Password" aria-label="password" class="w-full" @keyup.enter="doUnlock" />
        <Button class="w-full" size="lg" :disabled="busy || !selected" aria-label="Unlock" @click="doUnlock">Unlock</Button>
      </template>
      <p v-else class="text-sm text-muted-foreground">No wallets yet. Import a keystore to begin.</p>

      <div class="flex w-full flex-col gap-2">
        <Button variant="outline" class="w-full" @click="doImport">Import keystore…</Button>
        <Button variant="outline" class="w-full" @click="router.push('/create')">Create new wallet</Button>
        <Button variant="outline" class="w-full" @click="router.push('/import')">Import mnemonic</Button>
      </div>

      <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <p v-if="notice" class="text-sm text-muted-foreground">{{ notice }}</p>
    </div>
  </div>
</template>
