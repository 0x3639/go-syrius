<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import { ClipboardSetText, ClipboardGetText } from '../../wailsjs/runtime/runtime'

const wallet = useWalletStore()
const router = useRouter()

const SEED_CLIPBOARD_TTL_MS = 45_000
const step = ref(1)
const copied = ref(false)
const everCopied = ref(false)
async function copySeed() {
  try {
    await ClipboardSetText(mnemonic.value)
    copied.value = true
    everCopied.value = true
    setTimeout(() => (copied.value = false), 1500)
    // Don't let the seed linger in the clipboard for other apps to read: clear it
    // after a short window, but only if it's still ours (never wipe something the
    // user has copied since).
    const seed = mnemonic.value
    setTimeout(async () => {
      try {
        if ((await ClipboardGetText()) === seed) await ClipboardSetText('')
      } catch {
        /* ignore */
      }
    }, SEED_CLIPBOARD_TTL_MS)
  } catch {
    /* clipboard unavailable */
  }
}
const mnemonic = ref('')
const words = ref<string[]>([])
const positions = ref<number[]>([])
const answers = ref<Record<number, string>>({})
const name = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')

onMounted(async () => {
  try {
    mnemonic.value = await wallet.generateMnemonic()
    words.value = mnemonic.value.split(/\s+/)
    const idx = new Set<number>()
    while (idx.size < 3) idx.add(Math.floor(Math.random() * words.value.length))
    positions.value = [...idx].sort((a, b) => a - b)
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  }
})

const verifyOk = computed(
  () => positions.value.length === 3 && positions.value.every((p) => (answers.value[p] ?? '').trim() === words.value[p]),
)
const canCreate = computed(() => name.value.trim() !== '' && password.value.length > 0 && password.value === confirm.value)

async function finish() {
  error.value = ''
  try {
    // `name` is now a display name; the backend assigns a uuid keystore filename.
    // Capture the returned meta and unlock by its real id.
    const meta = await wallet.importMnemonic(name.value.trim(), password.value, mnemonic.value)
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
    <Card class="w-[32rem]">
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Create wallet</h1>

        <template v-if="step === 1">
          <p class="text-sm text-destructive">
            Write these {{ words.length }} words down and store them safely. Anyone with them controls your funds.
            They are shown only once.
          </p>
          <div class="grid grid-cols-3 gap-2 rounded bg-background p-3 font-mono text-sm text-foreground">
            <div v-for="(wd, i) in words" :key="i"><span class="text-muted-foreground">{{ i + 1 }}.</span> {{ wd }}</div>
          </div>
          <Button variant="outline" class="w-full" aria-label="copy seed phrase" @click="copySeed">
            {{ copied ? 'Copied ✓' : 'Copy seed phrase' }}
          </Button>
          <p v-if="everCopied" class="text-xs text-muted-foreground">
            In your clipboard — it auto-clears in ~45s. Clear it sooner if you paste it elsewhere.
          </p>
          <Button class="w-full" @click="step = 2">I've backed it up</Button>
        </template>

        <template v-else-if="step === 2">
          <p class="text-sm text-muted-foreground">Confirm your backup — enter these words:</p>
          <label v-for="p in positions" :key="p" class="block text-sm text-muted-foreground">
            Word #{{ p + 1 }}
            <Input v-model="answers[p]" :aria-label="`word ${p + 1}`" class="mt-1 font-mono" />
          </label>
          <Button class="w-full" :disabled="!verifyOk" @click="step = 3">Continue</Button>
        </template>

        <template v-else>
          <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
          <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
          <Button class="w-full" :disabled="!canCreate" @click="finish">Create wallet</Button>
        </template>

        <button class="text-xs text-muted-foreground" @click="router.push('/unlock')">Cancel</button>
        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
