<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Button, Table, TableBody, TableCell, TableEmpty, TableHead, TableHeader, TableRow } from 'nom-ui'
import * as Nom from '../../wailsjs/go/app/NomService'
import type { app } from '../../wailsjs/go/models'
import { useTokenStore } from '../stores/token'
import { useTxStore } from '../stores/tx'
import { useWalletStore } from '../stores/wallet'
import { formatAmount } from '../lib/format'
import { ChevronLeftIcon, ChevronRightIcon, XIcon } from '@lucide/vue'
import MonoTruncate from '../components/MonoTruncate.vue'
import TokensPanel from '../components/TokensPanel.vue'

const token = useTokenStore()
const tx = useTxStore()
const wallet = useWalletStore()

// One table shows either the owned tokens (default) or, after a search, the
// matching on-chain tokens. Paginated at 10 per page either way.
const PAGE_SIZE = 10
const showingSearch = ref(false)
const tableTokens = computed(() => (showingSearch.value ? token.searchResults : token.myTokens))
const myTokensPage = ref(0)
const myTokensPageCount = computed(() => Math.max(1, Math.ceil(tableTokens.value.length / PAGE_SIZE)))
const pagedMyTokens = computed(() =>
  tableTokens.value.slice(myTokensPage.value * PAGE_SIZE, myTokensPage.value * PAGE_SIZE + PAGE_SIZE),
)
// Keep the page in range if the list shrinks (e.g. after a refresh).
watch(myTokensPageCount, (n) => { if (myTokensPage.value >= n) myTokensPage.value = Math.max(0, n - 1) })

const error = ref('')
const searchQuery = ref('')
// issue form
const iName = ref(''), iSymbol = ref(''), iDomain = ref(''), iTotal = ref(''), iMax = ref('')
const iDecimals = ref(8), iMintable = ref(true), iBurnable = ref(true), iUtility = ref(false)
// mint form (per token, keyed by zts)
const mintZts = ref(''), mintAmount = ref(''), mintReceiver = ref('')
// burn form (per-row)
const burnZts = ref(''), rowBurnAmount = ref('')
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
// Mint/Update expand one shared inline row per token: opening one closes the
// other, clicking the same action again collapses it.
function startMint(zts: string) {
  updZts.value = ''; burnZts.value = ''
  if (mintZts.value === zts) { mintZts.value = ''; return }
  mintZts.value = zts; mintAmount.value = ''; mintReceiver.value = activeAddress.value
}
async function mint() {
  error.value = ''
  try { tx.awaitConfirm(await Nom.PrepareMint(mintZts.value, mintAmount.value, mintReceiver.value)) } catch (e) { fail(e) }
}
async function doSearch() {
  error.value = ''
  if (!searchQuery.value.trim()) { clearSearch(); return }
  try {
    await token.search(searchQuery.value)
    showingSearch.value = true
    myTokensPage.value = 0
    closeActionRow()
  } catch (e) { fail(e) }
}
function clearSearch() {
  searchQuery.value = ''
  showingSearch.value = false
  token.clearSearch()
  myTokensPage.value = 0
  closeActionRow()
}
async function burn(zts: string, amount: string) {
  error.value = ''
  try { tx.awaitConfirm(await Nom.PrepareBurn(zts, amount)) } catch (e) { fail(e) }
}
function startUpdate(zts: string, owner: string) {
  mintZts.value = ''; burnZts.value = ''
  if (updZts.value === zts) { updZts.value = ''; return }
  updZts.value = zts; updOwner.value = owner; updDisableMint.value = false; updDisableBurn.value = false
}
function startBurn(zts: string) {
  mintZts.value = ''; updZts.value = ''
  if (burnZts.value === zts) { burnZts.value = ''; return }
  burnZts.value = zts; rowBurnAmount.value = ''
}
function closeActionRow() { mintZts.value = ''; updZts.value = ''; burnZts.value = '' }
async function update(t: app.TokenInfo) {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareUpdateToken(updZts.value, updOwner.value, t.isMintable && !updDisableMint.value, t.isBurnable && !updDisableBurn.value))
  } catch (e) { fail(e) }
}
</script>

<template>
  <div class="mx-auto max-w-[48rem] space-y-6">
    <!-- Every token the active address HOLDS (any issuer) — this is the only
         place third-party ZTS balances are visible (Dashboard shows ZNN/QSR). -->
    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Held balances</h2>
      <TokensPanel />
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <h2 class="text-sm text-muted-foreground">Issue a token (1 ZNN fee)</h2>
      <div class="grid grid-cols-2 gap-2">
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" placeholder="name" v-model="iName" aria-label="issue name" />
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" placeholder="symbol (A-Z0-9)" v-model="iSymbol" aria-label="issue symbol" />
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" placeholder="domain (optional)" v-model="iDomain" aria-label="issue domain" />
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" type="number" min="0" max="18" placeholder="decimals" v-model.number="iDecimals" aria-label="issue decimals" />
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" placeholder="total supply (base units)" v-model="iTotal" aria-label="issue total" />
        <input class="rounded-md border border-input bg-transparent px-2 py-1 text-sm" placeholder="max supply (base units)" v-model="iMax" aria-label="issue max" />
      </div>
      <div class="flex gap-4 text-xs">
        <label><input type="checkbox" v-model="iMintable" /> mintable</label>
        <label><input type="checkbox" v-model="iBurnable" /> burnable</label>
        <label><input type="checkbox" v-model="iUtility" /> utility</label>
      </div>
      <Button @click="issue" aria-label="issue token">Issue token</Button>
    </section>

    <section class="rounded-xl border border-border bg-card p-5 space-y-2">
      <div class="flex flex-wrap items-center gap-2 pb-1">
        <h2 class="text-sm text-muted-foreground">
          {{ showingSearch ? `Search results (${tableTokens.length})` : 'My tokens' }}
        </h2>
        <div class="ml-auto flex items-center gap-2">
          <input
            class="w-64 rounded-md border border-input bg-transparent px-3 py-1.5 text-sm"
            placeholder="zts1…, name, or symbol" v-model="searchQuery" aria-label="token search"
            @keydown.enter="doSearch"
          />
          <Button variant="outline" size="sm" @click="doSearch">Search</Button>
          <Button v-if="showingSearch" variant="outline" size="sm" aria-label="clear search" @click="clearSearch">Clear</Button>
        </div>
      </div>
      <Table class="w-full table-fixed text-xs">
        <colgroup>
          <col class="w-20" />
          <col class="w-24" />
          <col class="w-28" />
          <col class="w-12" />
          <col />
          <col class="w-48" />
        </colgroup>
        <TableHeader>
          <TableRow>
            <TableHead>Symbol</TableHead>
            <TableHead>Name</TableHead>
            <TableHead title="Circulating supply already minted / maximum that can ever exist. 'fixed' = minting disabled.">
              <span class="block leading-tight">Supply</span>
              <span class="block font-normal leading-tight text-muted-foreground">(minted / max)</span>
            </TableHead>
            <TableHead>Dec</TableHead>
            <TableHead>Standard</TableHead>
            <TableHead class="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableEmpty v-if="tableTokens.length === 0" :colspan="6">
            {{ showingSearch ? 'No tokens match.' : 'No tokens owned.' }}
          </TableEmpty>
          <template v-for="t in pagedMyTokens" :key="t.tokenStandard">
            <TableRow>
              <TableCell class="truncate font-mono text-foreground">{{ t.symbol }}</TableCell>
              <TableCell class="truncate">{{ t.name }}</TableCell>
              <TableCell class="whitespace-nowrap font-mono text-xs">
                {{ formatAmount(t.totalSupply, t.decimals) }} / {{ formatAmount(t.maxSupply, t.decimals) }}
                <span v-if="!t.isMintable" class="text-muted-foreground"> · fixed</span>
              </TableCell>
              <TableCell class="font-mono text-xs">{{ t.decimals }}</TableCell>
              <TableCell class="pr-4"><MonoTruncate :value="t.tokenStandard" class="text-xs text-muted-foreground" /></TableCell>
              <TableCell>
                <!-- Mint/Update are owner-only (matters for search results);
                     Burn works for any held burnable token. -->
                <div class="flex justify-end gap-2">
                  <Button v-if="t.isMintable && t.owner === activeAddress" variant="outline" size="sm" @click="startMint(t.tokenStandard)" :aria-label="`mint ${t.symbol}`">Mint</Button>
                  <Button v-if="t.owner === activeAddress" variant="outline" size="sm" @click="startUpdate(t.tokenStandard, t.owner)" :aria-label="`update ${t.symbol}`">Update</Button>
                  <Button v-if="t.isBurnable" variant="outline" size="sm" @click="startBurn(t.tokenStandard)" :aria-label="`burn ${t.symbol}`">Burn</Button>
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="mintZts === t.tokenStandard || updZts === t.tokenStandard || burnZts === t.tokenStandard">
              <TableCell :colspan="6">
                <div class="flex items-center gap-2">
                  <div v-if="mintZts === t.tokenStandard" class="flex flex-1 flex-wrap items-center gap-2">
                    <input class="rounded-md border border-input bg-transparent px-2 py-1 text-xs" placeholder="amount (base units)" v-model="mintAmount" aria-label="mint amount" />
                    <input class="w-72 rounded-md border border-input bg-transparent px-2 py-1 text-xs" placeholder="receiver" v-model="mintReceiver" aria-label="mint receiver" />
                    <Button size="sm" @click="mint">Confirm mint</Button>
                  </div>
                  <div v-if="updZts === t.tokenStandard" class="flex flex-1 flex-wrap items-center gap-2">
                    <input class="w-72 rounded-md border border-input bg-transparent px-2 py-1 text-xs" placeholder="new owner" v-model="updOwner" aria-label="update owner" />
                    <label v-if="t.isMintable" class="text-xs"><input type="checkbox" v-model="updDisableMint" /> disable minting</label>
                    <label v-if="t.isBurnable" class="text-xs"><input type="checkbox" v-model="updDisableBurn" /> disable burning</label>
                    <Button size="sm" @click="update(t)">Confirm update</Button>
                  </div>
                  <div v-if="burnZts === t.tokenStandard" class="flex flex-1 flex-wrap items-center gap-2">
                    <input class="rounded-md border border-input bg-transparent px-2 py-1 text-xs" placeholder="amount (base units)" v-model="rowBurnAmount" aria-label="row burn amount" />
                    <span class="text-xs text-muted-foreground">burns from your held balance — irreversible</span>
                    <Button size="sm" @click="burn(t.tokenStandard, rowBurnAmount)">Confirm burn</Button>
                  </div>
                  <button
                    type="button" :aria-label="`close ${t.symbol} actions`" title="Close"
                    class="ml-auto grid h-7 w-7 shrink-0 place-items-center rounded border border-border text-muted-foreground transition-colors hover:bg-foreground/[0.06] hover:text-foreground"
                    @click="closeActionRow"
                  ><XIcon :size="14" /></button>
                </div>
              </TableCell>
            </TableRow>
          </template>
        </TableBody>
      </Table>
      <div v-if="myTokensPageCount > 1" class="flex items-center justify-end gap-3 pt-1 text-xs text-muted-foreground">
        <span>Page {{ myTokensPage + 1 }} / {{ myTokensPageCount }}</span>
        <button
          type="button" aria-label="previous token page" :disabled="myTokensPage === 0"
          class="grid h-7 w-7 place-items-center rounded border border-border transition-colors hover:bg-foreground/[0.06] disabled:opacity-40"
          @click="myTokensPage--"
        ><ChevronLeftIcon :size="14" /></button>
        <button
          type="button" aria-label="next token page" :disabled="myTokensPage >= myTokensPageCount - 1"
          class="grid h-7 w-7 place-items-center rounded border border-border transition-colors hover:bg-foreground/[0.06] disabled:opacity-40"
          @click="myTokensPage++"
        ><ChevronRightIcon :size="14" /></button>
      </div>
    </section>

    <p v-if="error" class="text-destructive text-sm" role="alert">{{ error }}</p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">Preparing… (PoW if required)</p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
