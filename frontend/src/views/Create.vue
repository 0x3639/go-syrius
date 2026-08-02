<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Card, CardContent, Input, Button } from 'nom-ui'
import { useWalletStore } from '../stores/wallet'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import { ClipboardSetText, ClipboardGetText } from '../../wailsjs/runtime/runtime'
import { XIcon, TriangleAlertIcon, CopyIcon, CheckIcon } from '@lucide/vue'
import {
  InteractionCollector,
  createEntropyRequest,
  pickDistinctIndexes,
  webCryptoAvailable,
  COLLECTION_MIN_DURATION_MS,
  ENTROPY_TARGET_PRESETS,
  ENTROPY_TARGET_DEFAULT,
  type EntropyRequest,
} from '../lib/wallet-entropy'

const wallet = useWalletStore()
const router = useRouter()

// Phase 7f (spec 2026-07-31 §9): no mnemonic may exist before an explicit
// generation action — there is deliberately no onMounted generation here.
type Stage = 'choice' | 'collect' | 'phrase' | 'verify' | 'password'
const stage = ref<Stage>('choice')
const generating = ref(false)
const cryptoUnavailable = ref(false)
const error = ref('')

const SEED_CLIPBOARD_TTL_MS = 45_000
const copied = ref(false)
const everCopied = ref(false)
const mnemonic = ref('')
const words = ref<string[]>([])
const positions = ref<number[]>([])
const answers = ref<Record<number, string>>({})
const name = ref('')
const password = ref('')
const confirm = ref('')

// --- interaction collection (spec §8, §9.3) ---
const collector = new InteractionCollector()
const sampleCount = ref(0)
const elapsedMs = ref(0)
const INDICATOR_BLOCKS = 32
const estimatedBits = ref(0)
const entropyTarget = ref<number>(ENTROPY_TARGET_DEFAULT)
let collectStart = 0
let elapsedTimer: ReturnType<typeof setInterval> | null = null
// A generation call can settle after the route is torn down. Late
// continuations must not repopulate the mnemonic refs onUnmounted cleared or
// restart the elapsed interval on a dead component.
let disposed = false

function startCollection() {
  error.value = ''
  if (!webCryptoAvailable()) {
    cryptoUnavailable.value = true
    error.value = 'Enhanced generation is unavailable: this environment has no Web Crypto support'
    return
  }
  collector.reset()
  sampleCount.value = 0
  estimatedBits.value = 0
  elapsedMs.value = 0
  startElapsedTimer()
  stage.value = 'collect'
}

function startElapsedTimer() {
  stopCollectionTimer()
  collectStart = performance.now()
  elapsedTimer = setInterval(() => {
    elapsedMs.value = performance.now() - collectStart
  }, 250)
}

function stopCollectionTimer() {
  if (elapsedTimer !== null) {
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
}

// A failed generation has already frozen and wiped the collector, so the
// retained sampleCount/elapsedMs are stale: leaving them would keep the
// "Enough interaction collected" banner up and let the next click submit the
// empty-transcript digest while the user believes their interaction
// contributed. Force a visible re-collect instead.
function restartCollectionAfterFailure() {
  if (disposed || stage.value !== 'collect') return
  sampleCount.value = 0
  estimatedBits.value = 0
  elapsedMs.value = 0
  startElapsedTimer()
}

function onPointer(e: PointerEvent) {
  if (collector.addPointerSample(e.clientX, e.clientY, performance.now(), e.pointerType)) {
    sampleCount.value = collector.sampleCount
    estimatedBits.value = collector.estimatedBits
  }
}

// Only the timing delta contributes; e.key/e.code/modifiers are never read
// (spec §8.1).
function onKey(e: KeyboardEvent) {
  if (collector.addKeySample(performance.now(), e.repeat)) {
    sampleCount.value = collector.sampleCount
    estimatedBits.value = collector.estimatedBits
  }
}

// Hard participation gate (spec 2026-08-02 §7): the conservatively credited
// bit estimate must reach the user-selected target, and the minimum-duration
// floor still applies. Still not a security estimate of the final seed.
const collectionReady = computed(
  () => elapsedMs.value >= COLLECTION_MIN_DURATION_MS && estimatedBits.value >= entropyTarget.value,
)
const shownBits = computed(() => Math.min(estimatedBits.value, entropyTarget.value))
const filledBlocks = computed(() =>
  Math.min(INDICATOR_BLOCKS, Math.floor((estimatedBits.value * INDICATOR_BLOCKS) / entropyTarget.value)),
)
const blocksLabel = computed(
  () => `${filledBlocks.value} of ${INDICATOR_BLOCKS} blocks · ${shownBits.value} / ${entropyTarget.value} estimated bits`,
)

function showPhrase(phrase: string) {
  if (disposed) return
  mnemonic.value = phrase
  words.value = phrase.split(/\s+/)
  positions.value = pickDistinctIndexes(3, words.value.length)
  answers.value = {}
  stage.value = 'phrase'
}

// Enhanced generation. `useTranscript: false` is the skip path — it still
// includes backend crypto/rand AND renderer getRandomValues; only the
// transcript contribution becomes the empty digest (spec §6.1).
async function generate(useTranscript: boolean) {
  if (generating.value) return
  generating.value = true
  error.value = ''
  stopCollectionTimer()
  const transcript = useTranscript ? collector.freeze() : new Uint8Array(0)
  try {
    let request: EntropyRequest
    try {
      request = await createEntropyRequest(transcript)
    } catch (e) {
      // Web Crypto is unavailable *or failing* (spec §8.4) — either way retries
      // will keep failing, so expose the explicit backend-only action. Scoped
      // to this step: a backend RPC failure below must not set the flag.
      cryptoUnavailable.value = true
      throw e
    }
    showPhrase(await wallet.generateMnemonicWithEntropy(request))
  } catch (e: any) {
    // Never silently downgrade (spec §8.4): show the failure; when Web Crypto
    // is the cause, additionally expose the explicit backend-only action.
    if (!webCryptoAvailable()) cryptoUnavailable.value = true
    error.value = e?.message ?? String(e)
    restartCollectionAfterFailure()
  } finally {
    collector.reset()
    generating.value = false
  }
}

// Deliberate, user-chosen fallback only — nothing calls this automatically.
async function generateBackendOnly() {
  if (generating.value) return
  generating.value = true
  error.value = ''
  stopCollectionTimer()
  try {
    showPhrase(await wallet.generateMnemonic())
  } catch (e: any) {
    error.value = e?.message ?? String(e)
    restartCollectionAfterFailure()
  } finally {
    collector.reset()
    generating.value = false
  }
}

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
    // A freshly created wallet starts with auto-receive OFF, so it never inherits
    // a previously-enabled global toggle and surprise-sweeps the new account.
    try {
      const s = await Cfg.GetSettings()
      if (s.autoReceive) {
        await Cfg.SetAutoReceive(false)
      }
    } catch {
      /* best-effort */
    }
    await wallet.unlock(meta.id, password.value)
    router.push('/dashboard')
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    password.value = ''
    confirm.value = ''
  }
}

// Best-effort teardown on any route exit (spec §9.4): stop timers, wipe the
// sample buffer, and drop mnemonic material from component state.
onUnmounted(() => {
  disposed = true
  stopCollectionTimer()
  collector.reset()
  mnemonic.value = ''
  words.value = []
  positions.value = []
  answers.value = {}
})
</script>

<template>
  <main class="grid min-h-screen place-items-center bg-background p-8">
    <Card class="relative w-[32rem]">
      <button
        class="absolute right-4 top-4 text-muted-foreground transition-colors hover:text-foreground"
        aria-label="close"
        @click="router.push('/unlock')">
        <XIcon :size="20" />
      </button>
      <CardContent class="space-y-4 p-6">
        <h1 class="text-xl text-foreground">Create wallet</h1>

        <template v-if="stage === 'choice'">
          <h2 class="text-lg text-foreground">Generate your recovery phrase</h2>
          <p class="text-sm text-muted-foreground">
            Syrius always uses operating-system cryptographic randomness. You can also
            contribute unpredictable pointer movement or key timing before the recovery
            phrase is created.
          </p>
          <Button class="w-full" :disabled="generating" @click="startCollection">Add interaction randomness</Button>
          <Button variant="outline" class="w-full" :disabled="generating" @click="generate(false)">Generate without interaction</Button>
          <Button v-if="cryptoUnavailable" variant="outline" class="w-full" :disabled="generating" @click="generateBackendOnly">
            Generate using backend randomness
          </Button>
        </template>

        <template v-else-if="stage === 'collect'">
          <h2 class="text-lg text-foreground">Add interaction randomness</h2>
          <p class="text-sm text-muted-foreground">
            Move your pointer, swipe, or focus this area and press arbitrary keys. Syrius
            records only movement, timing, and position—not the keys you press.
          </p>
          <p class="text-sm text-muted-foreground">
            Do not type a password or recovery phrase. Your final recovery phrase is the
            only backup you need; these interactions are not saved.
          </p>
          <fieldset :disabled="generating">
            <legend class="text-sm text-muted-foreground">Entropy target</legend>
            <div class="mt-1 flex gap-4">
              <label
                v-for="p in ENTROPY_TARGET_PRESETS"
                :key="p"
                class="flex items-center gap-1.5 text-sm text-foreground">
                <input v-model="entropyTarget" type="radio" name="entropy-target" :value="p" />
                {{ p }} estimated bits
              </label>
            </div>
          </fieldset>
          <p class="text-sm text-muted-foreground">
            Bits shown are a conservative estimate of your added interaction. Your wallet
            always uses operating-system cryptographic randomness as its primary source.
          </p>
          <div
            tabindex="0"
            aria-label="interaction collection area"
            class="grid h-40 place-items-center rounded-lg border border-dashed border-border bg-background text-sm text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            @pointermove="onPointer"
            @keydown="onKey">
            <span aria-hidden="true">Move your pointer here, or focus and press keys</span>
          </div>
          <!-- 32 = INDICATOR_BLOCKS; Tailwind arbitrary values cannot interpolate the constant -->
          <div
            role="progressbar"
            aria-label="Estimated entropy collected"
            :aria-valuemin="0"
            :aria-valuemax="entropyTarget"
            :aria-valuenow="shownBits"
            :aria-valuetext="blocksLabel"
            class="grid grid-cols-[repeat(32,minmax(0,1fr))] gap-0.5">
            <div
              v-for="i in INDICATOR_BLOCKS"
              :key="i"
              :data-filled="i <= filledBlocks"
              class="h-2 rounded-sm"
              :class="i <= filledBlocks ? 'bg-primary' : 'border border-border'"></div>
          </div>
          <p class="text-sm text-muted-foreground" aria-live="polite">
            <template v-if="collectionReady">Enough interaction collected</template>
            <template v-else>
              {{ blocksLabel }} — {{ sampleCount }} samples, {{ Math.floor(elapsedMs / 1000) }}s
            </template>
          </p>
          <Button class="w-full" :disabled="!collectionReady || generating" @click="generate(true)">Generate recovery phrase</Button>
          <Button variant="outline" class="w-full" :disabled="generating" @click="generate(false)">Skip interaction</Button>
          <Button v-if="cryptoUnavailable" variant="outline" class="w-full" :disabled="generating" @click="generateBackendOnly">
            Generate using backend randomness
          </Button>
        </template>

        <template v-else-if="stage === 'phrase'">
          <div class="flex gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
            <TriangleAlertIcon class="mt-0.5 shrink-0" :size="20" />
            <div>
              <p class="font-semibold">Important: save your recovery phrase</p>
              <p class="mt-0.5">Write these {{ words.length }} words down in order and store them safely. This is the only way to recover your wallet — anyone with them controls your funds. They are shown only once.</p>
            </div>
          </div>
          <div class="grid grid-cols-3 gap-2">
            <div v-for="(wd, i) in words" :key="i" class="rounded border border-border bg-background px-3 py-2 font-mono text-sm text-foreground">
              <span class="text-muted-foreground">{{ i + 1 }}.</span> {{ wd }}
            </div>
          </div>
          <Button variant="outline" class="w-full" aria-label="copy recovery phrase" @click="copySeed">
            <span class="inline-flex items-center justify-center gap-2">
              <CheckIcon v-if="copied" :size="15" />
              <CopyIcon v-else :size="15" />
              {{ copied ? 'Copied' : 'Copy recovery phrase' }}
            </span>
          </Button>
          <p v-if="everCopied" class="text-xs text-muted-foreground">
            In your clipboard — it auto-clears in ~45s. Clear it sooner if you paste it elsewhere.
          </p>
          <Button class="w-full" @click="stage = 'verify'">I've backed it up</Button>
        </template>

        <template v-else-if="stage === 'verify'">
          <p class="text-sm text-muted-foreground">Confirm your backup — enter these words:</p>
          <label v-for="p in positions" :key="p" class="block text-sm text-muted-foreground">
            Word #{{ p + 1 }}
            <Input v-model="answers[p]" :aria-label="`word ${p + 1}`" class="mt-1 font-mono" />
          </label>
          <Button class="w-full" :disabled="!verifyOk" @click="stage = 'password'">Continue</Button>
        </template>

        <template v-else>
          <Input v-model="name" placeholder="Wallet name" aria-label="wallet name" />
          <Input v-model="password" type="password" placeholder="Password" aria-label="password" />
          <Input v-model="confirm" type="password" placeholder="Confirm password" aria-label="confirm password" />
          <Button class="w-full" :disabled="!canCreate" @click="finish">Create wallet</Button>
        </template>

        <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      </CardContent>
    </Card>
  </main>
</template>
