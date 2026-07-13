<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Input, Button } from 'nom-ui'
import Field from '../components/Field.vue'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
import { useUiStore } from '../stores/ui'
import * as Cfg from '../../wailsjs/go/app/ConfigService'

const node = useNodeStore()
const wallet = useWalletStore()
const ui = useUiStore()

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
    await Cfg.SetChainID(Number(chainId.value))
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
  await ui.init()
  if (!wallet.wallets.length) await wallet.loadWallets()
  walletName.value = wallet.wallets.find((w) => w.id === wallet.active)?.name ?? ''
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

// Rename the currently-unlocked wallet (no password). Seeded from the active
// wallet's current display name once the wallet list has loaded.
const walletName = ref('')
const renameMsg = ref(''), renameErr = ref('')
const canRename = computed(() => walletName.value.trim() !== '' && wallet.active !== '')
async function doRename() {
  renameMsg.value = ''; renameErr.value = ''
  try {
    await wallet.rename(wallet.active, walletName.value.trim())
    renameMsg.value = 'Wallet name updated'
  } catch (e: any) { renameErr.value = e?.message ?? String(e) }
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
  <div class="mx-auto max-w-[48rem] space-y-6">
    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Appearance</h2>
      <label class="flex items-center gap-2 text-foreground">
        <input
          type="checkbox"
          aria-label="show startup animation"
          :checked="ui.splashEnabled"
          @change="ui.setSplashEnabled(($event.target as HTMLInputElement).checked)"
        />
        Show startup animation
      </label>
      <p class="text-xs text-muted-foreground">
        Plays the go-syrius logo intro each time the wallet opens. Takes effect on the next launch.
      </p>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Wallet name</h2>
      <input class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground" placeholder="Wallet name" v-model="walletName" aria-label="wallet name" />
      <Button :disabled="!canRename" @click="doRename">Rename</Button>
      <p v-if="renameMsg" class="text-primary text-sm">{{ renameMsg }}</p>
      <p v-if="renameErr" class="text-destructive text-sm" role="alert">{{ renameErr }}</p>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Change password</h2>
      <input type="password" class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground" placeholder="Current password" v-model="oldP" aria-label="current password" />
      <input type="password" class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground" placeholder="New password" v-model="newP" aria-label="new password" />
      <input type="password" class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground" placeholder="Confirm new password" v-model="confirmP" aria-label="confirm new password" />
      <Button :disabled="!canChange" @click="doChange">Change</Button>
      <p v-if="cpMsg" class="text-primary text-sm">{{ cpMsg }}</p>
      <p v-if="cpErr" class="text-destructive text-sm" role="alert">{{ cpErr }}</p>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Reveal mnemonic</h2>
      <p class="text-destructive text-xs">Anyone who sees these words controls your funds. Reveal only in private.</p>
      <template v-if="revealed">
        <div class="rounded bg-background p-3 font-mono text-sm break-words text-foreground">{{ revealed }}</div>
        <Button variant="outline" @click="hide">Hide</Button>
      </template>
      <template v-else>
        <input type="password" class="w-full rounded-md border border-input bg-transparent px-3 py-2 text-foreground" placeholder="Password" v-model="revealP" aria-label="reveal password" />
        <Button @click="doReveal">Reveal</Button>
      </template>
      <p v-if="revErr" class="text-destructive text-sm" role="alert">{{ revErr }}</p>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Node</h2>
      <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="remote" @change="modeDirty = true" /> Remote</label>
      <input class="w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-sm text-foreground" v-model="remoteUrl" @input="remoteDirty = true" aria-label="wss endpoint url" />
      <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="local" @change="modeDirty = true" /> Local</label>
      <input class="w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-sm text-foreground" v-model="localUrl" @input="localDirty = true" aria-label="ws endpoint url" />
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

      <Button v-if="node.mode !== 'embedded'" variant="outline" @click="doDeleteEmbedded">Delete embedded data ({{ (embeddedSize / 1e9).toFixed(2) }} GB)</Button>

      <div v-if="showEmbeddedConfirm" class="rounded border border-destructive/40 bg-background p-3 space-y-2">
        <p class="text-destructive text-sm">Embedded mode runs a full Zenon node in-app: it needs several GB of disk and can take hours to fully sync. Continue?</p>
        <div class="flex gap-2">
          <Button @click="confirmStartEmbedded">Start embedded</Button>
          <Button variant="outline" @click="showEmbeddedConfirm = false">Cancel</Button>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <Button @click="applyNode" aria-label="Apply node">Apply node</Button>
        <Button v-if="!node.connected" variant="outline" @click="retryNode">Retry</Button>
      </div>
      <p class="text-xs text-muted-foreground">{{ node.connected ? `Connected (${node.mode}) · height ${node.height}` : `Disconnected (${node.mode})` }}</p>
      <p v-if="nodeMsg" class="text-primary text-sm">{{ nodeMsg }}</p>
      <p v-if="nodeErr" class="text-destructive text-sm" role="alert">{{ nodeErr }}</p>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
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

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Testnet features</h2>
      <label class="flex items-center gap-2 text-foreground">
        <input
          type="checkbox"
          aria-label="show governance"
          :checked="ui.showGovernance"
          @change="ui.setShowGovernance(($event.target as HTMLInputElement).checked)"
        />
        Show Governance
      </label>
      <p class="text-xs text-destructive">
        Governance is experimental and only functional on testnet. Enabling it adds a Governance tab to the navigation.
      </p>
    </section>
  </div>
</template>
