<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'
import { useTokenStore } from '../stores/token'
import { useTxStore } from '../stores/tx'
import { useWalletStore } from '../stores/wallet'
import { formatAmount } from '../lib/format'
import NomConfirm from '../components/NomConfirm.vue'

const router = useRouter()
const token = useTokenStore()
const tx = useTxStore()
const wallet = useWalletStore()

const error = ref('')
const lookupZts = ref('')
// issue form
const iName = ref(''), iSymbol = ref(''), iDomain = ref(''), iTotal = ref(''), iMax = ref('')
const iDecimals = ref(8), iMintable = ref(true), iBurnable = ref(true), iUtility = ref(false)
// mint form (per token, keyed by zts)
const mintZts = ref(''), mintAmount = ref(''), mintReceiver = ref('')
// burn form
const burnAmount = ref('')
// update form
const updZts = ref(''), updOwner = ref(''), updDisableMint = ref(false), updDisableBurn = ref(false)

const activeAddress = computed(() => wallet.activeAddress())

onMounted(() => token.refresh())
watch(() => tx.status, (s) => { if (s === 'done') token.refresh() })

function fail(e: any) { error.value = e?.message ?? String(e) }

async function issue() {
  error.value = ''
  try { tx.awaitConfirm(await Nom.PrepareIssueToken(iName.value, iSymbol.value, iDomain.value, iTotal.value, iMax.value, iDecimals.value, iMintable.value, iBurnable.value, iUtility.value)) } catch (e) { fail(e) }
}
function startMint(zts: string) { mintZts.value = zts; mintAmount.value = ''; mintReceiver.value = activeAddress.value }
async function mint() {
  error.value = ''
  try { tx.awaitConfirm(await Nom.PrepareMint(mintZts.value, mintAmount.value, mintReceiver.value)) } catch (e) { fail(e) }
}
async function doLookup() { error.value = ''; try { await token.lookup(lookupZts.value) } catch (e) { fail(e) } }
async function burn(zts: string) {
  error.value = ''
  try { tx.awaitConfirm(await Nom.PrepareBurn(zts, burnAmount.value)) } catch (e) { fail(e) }
}
function startUpdate(zts: string, owner: string) { updZts.value = zts; updOwner.value = owner; updDisableMint.value = false; updDisableBurn.value = false }
async function update(t: app.TokenInfo) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareUpdateToken(updZts.value, updOwner.value, t.isMintable && !updDisableMint.value, t.isBurnable && !updDisableBurn.value))
  } catch (e) { fail(e) }
}
</script>

<template>
  <div class="mx-auto mt-8 w-[44rem] space-y-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl">Tokens</h1>
      <button class="rounded border border-muted-foreground/40 px-2 py-1 text-xs text-muted-foreground" @click="router.push('/home')">Back</button>
    </div>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">My tokens</h2>
      <div v-for="t in token.myTokens" :key="t.tokenStandard" class="border-b border-muted-foreground/20 py-2 text-sm space-y-1">
        <p class="font-mono">{{ t.symbol }} · {{ t.name }} · {{ formatAmount(t.totalSupply, t.decimals) }}/{{ formatAmount(t.maxSupply, t.decimals) }} · dec {{ t.decimals }}<template v-if="!t.isMintable"> · fixed</template></p>
        <p class="text-xs text-muted-foreground">{{ t.tokenStandard }}</p>
        <div class="flex flex-wrap gap-2 items-center">
          <button v-if="t.isMintable" class="rounded border border-muted-foreground/40 px-2 py-0.5 text-xs" @click="startMint(t.tokenStandard)" :aria-label="`mint ${t.symbol}`">Mint</button>
          <button class="rounded border border-muted-foreground/40 px-2 py-0.5 text-xs" @click="startUpdate(t.tokenStandard, t.owner)" :aria-label="`update ${t.symbol}`">Update</button>
        </div>
        <div v-if="mintZts === t.tokenStandard" class="flex flex-wrap gap-2 items-center pt-1">
          <input class="rounded bg-background px-2 py-1 text-xs" placeholder="amount (base units)" v-model="mintAmount" aria-label="mint amount" />
          <input class="rounded bg-background px-2 py-1 text-xs w-72" placeholder="receiver" v-model="mintReceiver" aria-label="mint receiver" />
          <button class="rounded bg-primary px-3 py-1 text-primary-foreground text-xs" @click="mint">Confirm mint</button>
        </div>
        <div v-if="updZts === t.tokenStandard" class="flex flex-wrap gap-2 items-center pt-1">
          <input class="rounded bg-background px-2 py-1 text-xs w-72" placeholder="new owner" v-model="updOwner" aria-label="update owner" />
          <label v-if="t.isMintable" class="text-xs"><input type="checkbox" v-model="updDisableMint" /> disable minting</label>
          <label v-if="t.isBurnable" class="text-xs"><input type="checkbox" v-model="updDisableBurn" /> disable burning</label>
          <button class="rounded bg-primary px-3 py-1 text-primary-foreground text-xs" @click="update(t)">Confirm update</button>
        </div>
      </div>
      <p v-if="token.myTokens.length === 0" class="text-xs text-muted-foreground">No tokens owned.</p>
    </section>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Look up / burn</h2>
      <div class="flex gap-2">
        <input class="flex-1 rounded bg-background px-3 py-2" placeholder="zts1…" v-model="lookupZts" aria-label="lookup zts" />
        <button class="rounded border border-muted-foreground/40 px-3 py-1 text-xs" @click="doLookup">Look up</button>
      </div>
      <template v-if="token.lookedUp">
        <p class="text-sm font-mono">{{ token.lookedUp.symbol }} · {{ token.lookedUp.name }} · {{ token.lookedUp.tokenStandard }}</p>
        <div v-if="token.lookedUp.isBurnable" class="flex gap-2 items-center">
          <input class="rounded bg-background px-2 py-1 text-xs" placeholder="amount (base units)" v-model="burnAmount" aria-label="burn amount" />
          <button class="rounded bg-primary px-3 py-1 text-primary-foreground text-xs" @click="burn(token.lookedUp.tokenStandard)">Burn</button>
        </div>
        <p v-else class="text-xs text-muted-foreground">Token is not burnable.</p>
      </template>
    </section>

    <section class="rounded bg-card p-4 space-y-2">
      <h2 class="text-sm text-muted-foreground">Issue a token (1 ZNN fee)</h2>
      <div class="grid grid-cols-2 gap-2">
        <input class="rounded bg-background px-2 py-1 text-sm" placeholder="name" v-model="iName" aria-label="issue name" />
        <input class="rounded bg-background px-2 py-1 text-sm" placeholder="symbol (A-Z0-9)" v-model="iSymbol" aria-label="issue symbol" />
        <input class="rounded bg-background px-2 py-1 text-sm" placeholder="domain (optional)" v-model="iDomain" aria-label="issue domain" />
        <input class="rounded bg-background px-2 py-1 text-sm" type="number" min="0" max="18" placeholder="decimals" v-model.number="iDecimals" aria-label="issue decimals" />
        <input class="rounded bg-background px-2 py-1 text-sm" placeholder="total supply (base units)" v-model="iTotal" aria-label="issue total" />
        <input class="rounded bg-background px-2 py-1 text-sm" placeholder="max supply (base units)" v-model="iMax" aria-label="issue max" />
      </div>
      <div class="flex gap-4 text-xs">
        <label><input type="checkbox" v-model="iMintable" /> mintable</label>
        <label><input type="checkbox" v-model="iBurnable" /> burnable</label>
        <label><input type="checkbox" v-model="iUtility" /> utility</label>
      </div>
      <button class="rounded bg-primary px-3 py-1 text-primary-foreground" @click="issue" aria-label="issue token">Issue token</button>
    </section>

    <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing… (PoW if required)</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>

    <NomConfirm />
  </div>
</template>
