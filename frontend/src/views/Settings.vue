<script setup lang="ts">
import { TriangleAlertIcon } from '@lucide/vue'
import { ref, computed, onMounted } from 'vue'
import { Input, Button } from 'nom-ui'
import Field from '../components/Field.vue'
import { useNodeStore } from '../stores/node'
import { useWalletStore } from '../stores/wallet'
import { useUiStore } from '../stores/ui'
import * as Cfg from '../../wailsjs/go/app/ConfigService'
import { SetAutoLockMinutes } from '../../wailsjs/go/app/WalletService'

const node = useNodeStore()
const wallet = useWalletStore()
const ui = useUiStore()

const nodeMode = ref('remote')
const remoteUrl = ref('')
const nodeMsg = ref('')
const nodeErr = ref('')
let loadedMode = 'remote'
let loadedRemote = ''
// Track explicit user edits so the async config load can't clobber them and
// so picking a mode never makes the (still-pristine) URL field look "edited".
const modeDirty = ref(false)
const remoteDirty = ref(false)
const showEmbeddedConfirm = ref(false)
const embeddedSize = ref(0)

async function refreshEmbedded() {
  try { embeddedSize.value = (await node.getEmbeddedInfo()).sizeBytes } catch {}
}

const autoLockMinutes = ref(5)
const autoLockErr = ref('')
async function applyAutoLock(v: number) {
  autoLockErr.value = ''
  autoLockMinutes.value = v
  try {
    await SetAutoLockMinutes(v)
  } catch (e: any) { autoLockErr.value = e?.message ?? String(e) }
}

const chainId = ref(1)
const allowMainnetSend = ref(false)
const showMainnetConfirm = ref(false)
const mainnetMsg = ref('')
const mainnetErr = ref('')
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

async function requestMainnetToggle() {
  if (!allowMainnetSend.value) {
    // click.prevent keeps the controlled checkbox at its persisted value until
    // the warning is explicitly confirmed.
    showMainnetConfirm.value = true
    return
  }
  mainnetMsg.value = ''; mainnetErr.value = ''
  try {
    await Cfg.SetAllowMainnetSend(false)
    allowMainnetSend.value = false
    mainnetMsg.value = 'Mainnet transactions disabled'
  } catch (e: any) { mainnetErr.value = e?.message ?? String(e) }
}

function cancelMainnetTransactions() {
  showMainnetConfirm.value = false
}

async function confirmMainnetTransactions() {
  mainnetMsg.value = ''; mainnetErr.value = ''
  try {
    await Cfg.SetAllowMainnetSend(true)
    allowMainnetSend.value = true
    mainnetMsg.value = 'Mainnet transactions enabled'
  } catch (e: any) { mainnetErr.value = e?.message ?? String(e) }
  finally { showMainnetConfirm.value = false }
}

const buildVersion = ref('')
const buildCommit = ref('')

onMounted(async () => {
  try {
    const build = await Cfg.GetBuildInfo()
    buildVersion.value = build.version
    buildCommit.value = build.commit
  } catch {
    buildVersion.value = 'dev'
    buildCommit.value = 'unknown'
  }
  const c = await node.getConfig()
  loadedMode = c.mode
  loadedRemote = c.remoteUrl
  if (!modeDirty.value) nodeMode.value = c.mode
  if (!remoteDirty.value) remoteUrl.value = c.remoteUrl
  await refreshEmbedded()
  try {
    const settings = await Cfg.GetSettings()
    chainId.value = settings.chainId || 1
    allowMainnetSend.value = settings.allowMainnetSend ?? false
    autoLockMinutes.value = settings.autoLockMinutes ?? 5
  } catch {}
  await ui.init()
  if (!wallet.wallets.length) await wallet.loadWallets()
  walletName.value = wallet.wallets.find((w) => w.id === wallet.active)?.name ?? ''
})

// One node transition at a time (defense in depth — the backend serializes
// transitions authoritatively): the Apply/Start buttons disable while a
// transition is in flight, so a success message can only belong to the
// request that actually ran.
const applyingNode = ref(false)

async function applyNode() {
  if (applyingNode.value) return
  nodeMsg.value = ''; nodeErr.value = ''
  // Embedded mode is gated behind an explicit warning before we ever start it.
  if (nodeMode.value === 'embedded' && loadedMode !== 'embedded') { showEmbeddedConfirm.value = true; return }
  applyingNode.value = true
  try {
    const remoteEdited = remoteDirty.value && remoteUrl.value !== loadedRemote
    if (remoteEdited) { await node.setUrl('remote', remoteUrl.value); loadedRemote = remoteUrl.value }
    if (nodeMode.value !== loadedMode) { await node.setMode(nodeMode.value); loadedMode = nodeMode.value }
    else if (nodeMode.value === 'remote' && remoteEdited) { await node.setMode(nodeMode.value) }
    nodeMsg.value = 'Node settings applied'
  } catch (e: any) { nodeErr.value = e?.message ?? String(e) }
  finally { applyingNode.value = false }
}
async function confirmStartEmbedded() {
  if (applyingNode.value) return
  nodeMsg.value = ''; nodeErr.value = ''
  applyingNode.value = true
  try { await node.setMode('embedded'); loadedMode = 'embedded'; modeDirty.value = false; nodeMsg.value = 'Node settings applied' }
  catch (e: any) { nodeErr.value = e?.message ?? String(e) }
  finally { showEmbeddedConfirm.value = false; applyingNode.value = false }
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
  <div class="page-content">
    <div class="page-halves">
      <div class="min-w-0 space-y-5">
        <h2 class="px-1 text-xs font-semibold text-muted-foreground">General</h2>
        <section class="panel-card space-y-3">
          <h2 class="section-title">Appearance</h2>
          <div class="flex flex-wrap items-center gap-3">
            <div class="min-w-0 flex-1">
              <p class="field-label">Theme</p>
              <p class="hint">Dark is the default.</p>
            </div>
            <div class="flex gap-1 rounded-xl bg-background p-1" role="group" aria-label="Theme">
              <button
                v-for="t in (['dark', 'light'] as const)"
                :key="t"
                type="button"
                class="h-8.5 rounded-lg px-4 text-sm font-semibold capitalize transition-colors"
                :class="ui.theme === t ? 'bg-foreground/10 text-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-pressed="ui.theme === t"
                @click="ui.theme !== t && ui.toggleTheme()"
              >{{ t }}</button>
            </div>
          </div>
          <label class="flex flex-wrap items-center gap-2 text-sm text-foreground">
            <input
              type="checkbox"
              aria-label="show startup animation"
              :checked="ui.splashEnabled"
              @change="ui.setSplashEnabled(($event.target as HTMLInputElement).checked)"
            />
            Show startup animation
          </label>
          <p class="hint">
            Plays the go-syrius logo intro each time the wallet opens. Takes effect on the next launch.
          </p>
        </section>

        <section class="panel-card space-y-3">
          <h2 class="section-title">Wallet name</h2>
          <input class="w-full rounded-lg border border-input bg-background px-3 py-2.5 text-foreground outline-none focus:border-primary" placeholder="Wallet name" v-model="walletName" aria-label="wallet name" />
          <Button :disabled="!canRename" @click="doRename">Rename</Button>
          <p v-if="renameMsg" class="text-primary text-sm">{{ renameMsg }}</p>
          <p v-if="renameErr" class="text-destructive text-sm" role="alert">{{ renameErr }}</p>
        </section>

        <section class="panel-card space-y-3">
          <h2 class="section-title">Node</h2>
          <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="remote" @change="modeDirty = true" /> Remote</label>
          <input class="w-full rounded-lg border border-input bg-background px-3 py-2.5 font-mono text-sm text-foreground outline-none focus:border-primary" v-model="remoteUrl" @input="remoteDirty = true" aria-label="wss endpoint url" />
          <p class="hint">Running your own znnd? Point Remote at ws://127.0.0.1:35998.</p>
          <label class="flex items-center gap-2 text-foreground"><input type="radio" v-model="nodeMode" value="embedded" @change="modeDirty = true" /> Embedded</label>
          <p class="hint">Runs a full node in-app at ws://127.0.0.1:35998</p>

          <div v-if="node.mode === 'embedded' && node.sync" class="rounded bg-background p-3 space-y-1 text-sm text-foreground">
            <div v-if="node.sync.targetHeight > 0" class="h-2 w-full rounded bg-card"><div class="h-2 rounded bg-primary" :style="`width:${node.sync.percent}%`"></div></div>
            <!-- Stalled wins over every other rendering: a node that died before any
                 peer answered stalls at targetHeight 0, and that must not read as the
                 optimistic "connecting to peers…". Heights only when we have a target. -->
            <p v-if="node.sync.state === 'stalled'" class="text-destructive" role="alert">
              sync stalled — restart go-syrius if this persists<template v-if="node.sync.targetHeight > 0"> · {{ node.sync.currentHeight.toLocaleString('en-US') }} / {{ node.sync.targetHeight.toLocaleString('en-US') }}</template>
            </p>
            <p v-else-if="node.sync.targetHeight === 0" class="text-muted-foreground">connecting to peers…</p>
            <p v-else>{{ node.sync.state }} · {{ node.sync.currentHeight.toLocaleString('en-US') }} / {{ node.sync.targetHeight.toLocaleString('en-US') }} ({{ node.sync.percent.toFixed(1) }}%)<template v-if="node.sync.etaSeconds > 0"> · ETA {{ fmtEta(node.sync.etaSeconds) }}</template></p>
            <p class="text-muted-foreground">{{ node.sync.peers }} peers · {{ (embeddedSize / 1e9).toFixed(2) }} GB on disk</p>
          </div>

          <Button v-if="node.mode !== 'embedded'" variant="outline" @click="doDeleteEmbedded">Delete embedded data ({{ (embeddedSize / 1e9).toFixed(2) }} GB)</Button>

          <div v-if="showEmbeddedConfirm" class="rounded border border-destructive/40 bg-background p-3 space-y-2">
            <p class="text-destructive text-sm">Embedded mode runs a full Zenon node in-app: it needs several GB of disk and can take hours to fully sync. Continue?</p>
            <div class="flex gap-2">
              <Button :disabled="applyingNode" @click="confirmStartEmbedded">Start embedded</Button>
              <Button variant="outline" @click="showEmbeddedConfirm = false">Cancel</Button>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <Button :disabled="applyingNode" @click="applyNode" aria-label="Apply node">Apply node</Button>
            <Button v-if="!node.connected" variant="outline" @click="retryNode">Retry</Button>
          </div>
          <p class="hint">{{ node.connected ? `Connected (${node.mode}) · height ${node.height.toLocaleString('en-US')}` : `Disconnected (${node.mode})` }}</p>
          <p v-if="chainMismatch" class="text-destructive text-sm" role="alert">
            Configured Chain ID {{ chainId }} differs from the connected node's chain {{ node.chainId }} — sends will be rejected until they match.
          </p>
          <p v-if="nodeMsg" class="text-primary text-sm">{{ nodeMsg }}</p>
          <p v-if="nodeErr" class="text-destructive text-sm" role="alert">{{ nodeErr }}</p>
        </section>

        <section class="panel-card space-y-3">
          <h2 class="section-title">About</h2>
          <div class="flex justify-between gap-4 text-sm">
            <span class="text-muted-foreground">Version</span>
            <span class="font-mono text-foreground">{{ buildVersion }}</span>
          </div>
          <div class="flex justify-between gap-4 text-sm">
            <span class="text-muted-foreground">Build commit</span>
            <span class="font-mono text-foreground">{{ buildCommit }}</span>
          </div>
          <p class="hint">
            Verify downloaded binaries against the SHA256SUMS file published with each release.
          </p>
        </section>
  
      </div>

      <div class="min-w-0 space-y-5">
        <h2 class="px-1 text-xs font-semibold text-muted-foreground">Security</h2>
        <section class="panel-card space-y-3">
          <h2 class="section-title">Change password</h2>
          <input type="password" class="w-full rounded-lg border border-input bg-background px-3 py-2.5 text-foreground outline-none focus:border-primary" placeholder="Current password" v-model="oldP" aria-label="current password" />
          <input type="password" class="w-full rounded-lg border border-input bg-background px-3 py-2.5 text-foreground outline-none focus:border-primary" placeholder="New password" v-model="newP" aria-label="new password" />
          <input type="password" class="w-full rounded-lg border border-input bg-background px-3 py-2.5 text-foreground outline-none focus:border-primary" placeholder="Confirm new password" v-model="confirmP" aria-label="confirm new password" />
          <Button :disabled="!canChange" @click="doChange">Change</Button>
          <p v-if="cpMsg" class="text-primary text-sm">{{ cpMsg }}</p>
          <p v-if="cpErr" class="text-destructive text-sm" role="alert">{{ cpErr }}</p>
        </section>

        <section class="space-y-3 rounded-2xl border border-destructive/40 bg-destructive/[0.06] p-6">
          <h2 class="section-title flex items-center gap-2"><TriangleAlertIcon :size="18" class="text-destructive" />Recovery phrase</h2>
          <p class="text-sm text-destructive">Anyone who sees these words controls your funds. Reveal only in private, and never type them into a website.</p>
          <template v-if="revealed">
            <div class="rounded bg-background p-3 font-mono text-sm break-words text-foreground">{{ revealed }}</div>
            <Button variant="outline" @click="hide">Hide</Button>
          </template>
          <template v-else>
            <input type="password" class="w-full rounded-lg border border-input bg-background px-3 py-2.5 text-foreground outline-none focus:border-primary" placeholder="Password" v-model="revealP" aria-label="reveal password" />
            <Button variant="outline" class="border-destructive/60 text-destructive" @click="doReveal">Reveal</Button>
          </template>
          <p v-if="revErr" class="text-destructive text-sm" role="alert">{{ revErr }}</p>
        </section>

        <section class="panel-card space-y-3">
          <h2 class="section-title">Auto-lock</h2>
          <label class="flex flex-wrap items-center gap-2 text-sm text-foreground">
            Lock the wallet after
            <select
              aria-label="auto-lock timeout"
              class="select-field h-9 w-auto min-w-32"
              :value="String(autoLockMinutes)"
              @change="applyAutoLock(Number(($event.target as HTMLSelectElement).value))"
            >
              <option value="1">1 minute</option>
              <option value="5">5 minutes</option>
              <option value="15">15 minutes</option>
              <option value="30">30 minutes</option>
              <option value="0">Never</option>
            </select>
            of inactivity
          </label>
          <p class="hint">
            The wallet locks itself and returns to the unlock screen after this much inactivity.
          </p>
          <p v-if="autoLockErr" class="text-destructive text-sm" role="alert">{{ autoLockErr }}</p>
        </section>

        <details class="panel-card">
          <summary class="section-title cursor-pointer">Advanced network settings</summary>
          <div class="mt-4 space-y-3">
          <p class="hint">The chain the wallet builds transactions for (1 = mainnet, 73404 = testnet).</p>
          <Field label="Chain ID">
            <Input type="number" v-model="chainId" aria-label="chain id" />
          </Field>
          <p class="hint">
            Connected node chain: {{ node.connected && node.chainId !== 0 ? node.chainId : '—' }}
          </p>
          <Button @click="applyChainId">Apply network</Button>
          <p v-if="chainMsg" class="text-primary text-sm">{{ chainMsg }}</p>
          <p v-if="chainErr" class="text-destructive text-sm" role="alert">{{ chainErr }}</p>

          <div class="mt-4 border-t border-border pt-4 space-y-2">
            <label class="flex flex-wrap items-center gap-2 text-sm text-foreground">
              <input
                type="checkbox"
                aria-label="enable mainnet transactions"
                :checked="allowMainnetSend"
                @click.prevent="requestMainnetToggle"
              />
              Enable mainnet transactions
            </label>
            <p class="text-xs text-destructive">
              This allows go-syrius and approved WalletConnect sessions to sign and publish transactions with real funds on chain 1.
            </p>
            <div v-if="showMainnetConfirm" class="rounded border border-destructive/40 bg-background p-3 space-y-2">
              <p class="text-sm text-destructive">
                Mainnet transactions use real ZNN, QSR, and tokens. Verify the connected node, destination, amount, and contract call before every confirmation.
              </p>
              <div class="flex gap-2">
                <Button aria-label="Confirm mainnet transactions" @click="confirmMainnetTransactions">I understand, enable</Button>
                <Button variant="outline" @click="cancelMainnetTransactions">Cancel</Button>
              </div>
            </div>
            <p v-if="mainnetMsg" class="text-primary text-sm">{{ mainnetMsg }}</p>
            <p v-if="mainnetErr" class="text-destructive text-sm" role="alert">{{ mainnetErr }}</p>
          </div>
          </div>
        </details>
      </div>
    </div>
  </div>
</template>
