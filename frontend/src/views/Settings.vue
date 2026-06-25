<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Input, Button } from 'nom-ui'
import Field from '../components/Field.vue'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

const node = useNodeStore()
const wallet = useWalletStore()
const router = useRouter()

const nodeMode = ref('remote')
const remoteUrl = ref('')
const localUrl = ref('')
const nodeMsg = ref('')
const nodeErr = ref('')
let loadedMode = 'remote'
let loadedRemote = ''
let loadedLocal = ''
// Track explicit user edits so the async config load can't clobber them and
// so picking a mode never makes the (still-pristine) URL fields look "edited".
const modeDirty = ref(false)
const remoteDirty = ref(false)
const localDirty = ref(false)
const showEmbeddedConfirm = ref(false)
const embeddedSize = ref(0)

async function refreshEmbedded() {
  try { embeddedSize.value = (await node.getEmbeddedInfo()).sizeBytes } catch {}
}

const chainId = ref(1)
const chainMsg = ref('')
const chainErr = ref('')
const chainMismatch = computed(
  () => node.connected && node.chainId !== 0 && Number(chainId.value) !== node.chainId,
)
async function applyChainId() {
  chainMsg.value = ''; chainErr.value = ''
  try {
    const s = await Cfg.GetSettings()
    s.chainId = Number(chainId.value)
    await Cfg.SetSettings(s)
    chainMsg.value = 'Network configuration applied'
  } catch (e: any) { chainErr.value = e?.message ?? String(e) }
}

onMounted(async () => {
  const c = await node.getConfig()
  loadedMode = c.mode
  loadedRemote = c.remoteUrl
  loadedLocal = c.localUrl
  if (!modeDirty.value) nodeMode.value = c.mode
  if (!remoteDirty.value) remoteUrl.value = c.remoteUrl
  if (!localDirty.value) localUrl.value = c.localUrl
  await refreshEmbedded()
  try { chainId.value = (await Cfg.GetSettings()).chainId || 1 } catch {}
})

async function applyNode() {
  nodeMsg.value = ''; nodeErr.value = ''
  // Embedded mode is gated behind an explicit warning before we ever start it.
  if (nodeMode.value === 'embedded' && loadedMode !== 'embedded') { showEmbeddedConfirm.value = true; return }
  try {
    const remoteEdited = remoteDirty.value && remoteUrl.value !== loadedRemote
    const localEdited = localDirty.value && localUrl.value !== loadedLocal
    if (remoteEdited) { await node.setUrl('remote', remoteUrl.value); loadedRemote = remoteUrl.value }
    if (localEdited) { await node.setUrl('local', localUrl.value); loadedLocal = localUrl.value }
    if (nodeMode.value !== loadedMode) { await node.setMode(nodeMode.value); loadedMode = nodeMode.value }
    else if (nodeMode.value === 'remote' ? remoteEdited : localEdited) { await node.setMode(nodeMode.value) }
    nodeMsg.value = 'Node settings applied'
  } catch (e: any) { nodeErr.value = e?.message ?? String(e) }
}
async function confirmStartEmbedded() {
  nodeMsg.value = ''; nodeErr.value = ''
  try { await node.setMode('embedded'); loadedMode = 'embedded'; modeDirty.value = false; nodeMsg.value = 'Node settings applied' }
  catch (e: any) { nodeErr.value = e?.message ?? String(e) }
  finally { showEmbeddedConfirm.value = false }
}
async function doDeleteEmbedded() {
  nodeErr.value = ''
  try { await node.deleteEmbeddedData(); await refreshEmbedded() } catch (e: any) { nodeErr.value = e?.message ?? String(e) }
}
function fmtEta(s: number): string {
  if (s <= 0) return ''
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}
async function retryNode() {
  nodeMsg.value = ''; nodeErr.value = ''
  try { await node.setMode(nodeMode.value) } catch (e: any) { nodeErr.value = e?.message ?? String(e) }
}

const oldP = ref(''), newP = ref(''), confirmP = ref(''), cpMsg = ref(''), cpErr = ref('')
const canChange = computed(() => oldP.value.length > 0 && newP.value.length > 0 && newP.value === confirmP.value)
async function doChange() {
  cpMsg.value = ''; cpErr.value = ''
  try { await wallet.changePassword(oldP.value, newP.value); cpMsg.value = 'Password changed'; oldP.value = newP.value = confirmP.value = '' }
  catch (e: any) { cpErr.value = e?.message ?? String(e) }
}

const revealP = ref(''), revealed = ref(''), revErr = ref('')
async function doReveal() {
  revErr.value = ''; revealed.value = ''
  try { revealed.value = await wallet.revealMnemonic(revealP.value) } catch (e: any) { revErr.value = e?.message ?? String(e) }
  revealP.value = ''
}
function hide() { revealed.value = '' }
</script>

<template>
  <div class="mx-auto mt-8 w-[32rem] space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-xl text-foreground">Settings</h1>
      <button class="text-xs text-muted-foreground" @click="router.push('/home')">Back</button>
    </div>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Change password</h2>
      <input type="password" class="w-full rounded bg-background px-3 py-2 text-foreground" placeholder="Current password" v-model="oldP" aria-label="current password" />
      <input type="password" class="w-full rounded bg-background px-3 py-2 text-foreground" placeholder="New password" v-model="newP" aria-label="new password" />
      <input type="password" class="w-full rounded bg-background px-3 py-2 text-foreground" placeholder="Confirm new password" v-model="confirmP" aria-label="confirm new password" />
      <button class="rounded bg-primary px-3 py-1 text-background disabled:opacity-50" :disabled="!canChange" @click="doChange">Change</button>
      <p v-if="cpMsg" class="text-primary text-sm">{{ cpMsg }}</p>
      <p v-if="cpErr" class="text-destructive text-sm" role="alert">{{ cpErr }}</p>
    </section>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Reveal mnemonic</h2>
      <p class="text-destructive text-xs">Anyone who sees these words controls your funds. Reveal only in private.</p>
      <template v-if="revealed">
        <div class="rounded bg-background p-3 font-mono text-sm break-words text-foreground">{{ revealed }}</div>
        <button class="rounded border border-muted-foreground/40 px-3 py-1 text-muted-foreground" @click="hide">Hide</button>
      </template>
      <template v-else>
        <input type="password" class="w-full rounded bg-background px-3 py-2 text-foreground" placeholder="Password" v-model="revealP" aria-label="reveal password" />
        <button class="rounded bg-primary px-3 py-1 text-background" @click="doReveal">Reveal</button>
      </template>
      <p v-if="revErr" class="text-destructive text-sm" role="alert">{{ revErr }}</p>
    </section>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Node</h2>
      <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="remote" @change="modeDirty = true" /> Remote</label>
      <input class="w-full rounded bg-background px-3 py-2 font-mono text-sm text-foreground" v-model="remoteUrl" @input="remoteDirty = true" aria-label="wss endpoint url" />
      <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="local" @change="modeDirty = true" /> Local</label>
      <input class="w-full rounded bg-background px-3 py-2 font-mono text-sm text-foreground" v-model="localUrl" @input="localDirty = true" aria-label="ws endpoint url" />
      <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="embedded" @change="modeDirty = true" /> Embedded</label>
      <p class="text-xs text-muted-foreground">Runs a full node in-app at ws://127.0.0.1:35998</p>

      <div v-if="node.mode === 'embedded' && node.sync" class="rounded bg-background p-3 space-y-1 text-sm text-foreground">
        <template v-if="node.sync.targetHeight === 0">
          <p class="text-muted-foreground">connecting to peers…</p>
        </template>
        <template v-else>
          <div class="h-2 w-full rounded bg-card"><div class="h-2 rounded bg-primary" :style="`width:${node.sync.percent}%`"></div></div>
          <p>{{ node.sync.state }} · {{ node.sync.currentHeight }} / {{ node.sync.targetHeight }} ({{ node.sync.percent.toFixed(1) }}%)<template v-if="node.sync.etaSeconds > 0"> · ETA {{ fmtEta(node.sync.etaSeconds) }}</template></p>
        </template>
        <p class="text-muted-foreground">{{ node.sync.peers }} peers · {{ (embeddedSize / 1e9).toFixed(2) }} GB on disk</p>
      </div>

      <button v-if="node.mode !== 'embedded'" class="rounded border border-muted-foreground/40 px-3 py-1 text-muted-foreground" @click="doDeleteEmbedded">Delete embedded data ({{ (embeddedSize / 1e9).toFixed(2) }} GB)</button>

      <div v-if="showEmbeddedConfirm" class="rounded border border-destructive/40 bg-background p-3 space-y-2">
        <p class="text-destructive text-sm">Embedded mode runs a full Zenon node in-app: it needs several GB of disk and can take hours to fully sync. Continue?</p>
        <div class="flex gap-2">
          <button class="rounded bg-primary px-3 py-1 text-background" @click="confirmStartEmbedded">Start embedded</button>
          <button class="rounded border border-muted-foreground/40 px-3 py-1 text-muted-foreground" @click="showEmbeddedConfirm = false">Cancel</button>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <button class="rounded bg-primary px-3 py-1 text-background" @click="applyNode" aria-label="Apply node">Apply node</button>
        <button v-if="!node.connected" class="rounded border border-muted-foreground/40 px-3 py-1 text-muted-foreground" @click="retryNode">Retry</button>
      </div>
      <p class="text-xs text-muted-foreground">{{ node.connected ? `Connected (${node.mode}) · height ${node.height}` : `Disconnected (${node.mode})` }}</p>
      <p v-if="nodeMsg" class="text-primary text-sm">{{ nodeMsg }}</p>
      <p v-if="nodeErr" class="text-destructive text-sm" role="alert">{{ nodeErr }}</p>
    </section>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Network Configuration</h2>
      <p class="text-xs text-muted-foreground">The chain the wallet builds transactions for (1 = mainnet, 73404 = testnet).</p>
      <Field label="Chain ID">
        <Input type="number" v-model="chainId" aria-label="chain id" />
      </Field>
      <p class="text-xs text-muted-foreground">
        Connected node chain: {{ node.connected && node.chainId !== 0 ? node.chainId : '—' }}
      </p>
      <p v-if="chainMismatch" class="text-destructive text-sm" role="alert">
        Configured Chain ID {{ chainId }} differs from the connected node's chain {{ node.chainId }} — sends will be rejected until they match.
      </p>
      <Button @click="applyChainId">Apply network</Button>
      <p v-if="chainMsg" class="text-primary text-sm">{{ chainMsg }}</p>
      <p v-if="chainErr" class="text-destructive text-sm" role="alert">{{ chainErr }}</p>
    </section>
  </div>
</template>
