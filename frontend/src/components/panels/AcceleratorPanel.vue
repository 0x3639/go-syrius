<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import { formatAmount } from '../../lib/format'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()

const { projects, selectedProject, votablePillars, error: accError } = storeToRefs(acc)

const error = ref('')

// donate
const donateAmount = ref('')
const donateToken = ref('QSR')
// vote — 0=yes,1=no,2=abstain (embedded.Vote*)
const voteId = ref('')
const votePillar = ref('')
const voteChoice = ref(0)
// create project
const cName = ref('')
const cDesc = ref('')
const cUrl = ref('')
const cZnn = ref('')
const cQsr = ref('')
// add/update phase — both keyed by the PROJECT id on-chain (UpdatePhase
// updates the project's current phase)
const phProjectId = ref('')
const phName = ref('')
const phDesc = ref('')
const phUrl = ref('')
const phZnn = ref('')
const phQsr = ref('')

const STATUS = ['Voting', 'Active', 'Paid', 'Closed', 'Completed']
function statusLabel(n: number) {
  return STATUS[n] ?? `#${n}`
}

function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e)
}

// READ actions go through the store's load methods.
onMounted(() => {
  acc.loadProjects()
  acc.loadVotablePillars()
})

// Refresh accelerator data once a write completes, mirroring the Svelte
// reactive `$: if ($tx.status === 'done')`.
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') {
      acc.loadProjects()
      acc.loadVotablePillars()
    }
  },
)

// Default the vote pillar to the first available, like the Svelte reactive.
watch(
  votablePillars,
  (list) => {
    if (list.length > 0 && votePillar.value === '') votePillar.value = list[0]
  },
  { immediate: true },
)

// WRITE actions: NoM-confirm pattern — prepare the call, hand the preview to the
// global NomConfirm dialog via tx.awaitConfirm. The panel renders no modal.
// Amounts are passed verbatim as base-unit integer strings (matching the Svelte
// original; the backend parses base-10 integers and re-validates).
async function donate() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareDonate(donateAmount.value, donateToken.value))
  } catch (e) {
    fail(e)
  }
}
async function vote() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareVote(voteId.value, votePillar.value, voteChoice.value))
  } catch (e) {
    fail(e)
  }
}
async function createProject() {
  error.value = ''
  try {
    tx.awaitConfirm(
      await Nom.PrepareCreateProject(cName.value, cDesc.value, cUrl.value, cZnn.value, cQsr.value),
    )
  } catch (e) {
    fail(e)
  }
}
async function addPhase() {
  error.value = ''
  try {
    tx.awaitConfirm(
      await Nom.PrepareAddPhase(
        phProjectId.value,
        phName.value,
        phDesc.value,
        phUrl.value,
        phZnn.value,
        phQsr.value,
      ),
    )
  } catch (e) {
    fail(e)
  }
}
async function updatePhase() {
  error.value = ''
  try {
    tx.awaitConfirm(
      await Nom.PrepareUpdatePhase(
        phProjectId.value,
        phName.value,
        phDesc.value,
        phUrl.value,
        phZnn.value,
        phQsr.value,
      ),
    )
  } catch (e) {
    fail(e)
  }
}
</script>

<template>
  <div class="space-y-4 p-4">
    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Projects</h2>
      <div
        v-for="p in projects"
        :key="p.id"
        class="space-y-1 border-b border-border/60 py-2 text-sm last:border-b-0"
      >
        <p class="font-mono text-foreground">
          {{ p.name }} · {{ statusLabel(p.status) }} · {{ formatAmount(p.znnFundsNeeded, 8) }} ZNN /
          {{ formatAmount(p.qsrFundsNeeded, 8) }} QSR
        </p>
        <p class="text-xs text-muted-foreground">
          votes: {{ p.votes.yes }} yes / {{ p.votes.no }} no / {{ p.votes.total }} total
        </p>
        <Button
          variant="outline"
          class="px-2 py-1 text-xs"
          :aria-label="`open ${p.name}`"
          @click="acc.openProject(p.id)"
          >Phases</Button
        >
        <template v-if="selectedProject && selectedProject.id === p.id">
          <div
            v-for="ph in selectedProject.phases"
            :key="ph.id"
            class="ml-4 mt-1 text-xs text-muted-foreground"
          >
            {{ ph.name }} · {{ statusLabel(ph.status) }} ·
            {{ ph.votes.yes }}/{{ ph.votes.no }}/{{ ph.votes.total }} ·
            <span class="font-mono">{{ ph.id }}</span>
          </div>
          <p
            v-if="selectedProject.phases.length === 0"
            class="ml-4 mt-1 text-xs text-muted-foreground"
          >
            No phases.
          </p>
        </template>
      </div>
      <p v-if="projects.length === 0" class="text-xs text-muted-foreground">No projects.</p>
    </section>

    <section class="space-y-3 rounded-lg border border-border bg-card p-4">
      <h2 class="text-sm font-medium text-foreground">Donate</h2>
      <div class="flex items-end gap-2">
        <div class="flex-1">
          <Field label="Amount (base units)">
            <Input
              v-model="donateAmount"
              placeholder="amount (base units)"
              aria-label="donate amount"
            />
          </Field>
        </div>
        <select
          v-model="donateToken"
          class="rounded border border-border bg-muted px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-primary"
          aria-label="donate token"
        >
          <option value="ZNN">ZNN</option>
          <option value="QSR">QSR</option>
        </select>
        <Button @click="donate">Donate</Button>
      </div>
    </section>

    <section
      v-if="votablePillars.length > 0"
      class="space-y-3 rounded-lg border border-border bg-card p-4"
    >
      <h2 class="text-sm font-medium text-foreground">Vote (Pillar operator)</h2>
      <div class="flex flex-wrap items-end gap-2">
        <div class="min-w-[18rem] flex-1">
          <Field label="Target id">
            <Input
              v-model="voteId"
              placeholder="project or phase id (0x…)"
              aria-label="vote target id"
            />
          </Field>
        </div>
        <select
          v-model="votePillar"
          class="rounded border border-border bg-muted px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-primary"
          aria-label="vote pillar"
        >
          <option v-for="name in votablePillars" :key="name" :value="name">{{ name }}</option>
        </select>
        <select
          v-model.number="voteChoice"
          class="rounded border border-border bg-muted px-3 py-2 text-foreground outline-none focus:ring-2 focus:ring-primary"
          aria-label="vote choice"
        >
          <option :value="0">Yes</option>
          <option :value="1">No</option>
          <option :value="2">Abstain</option>
        </select>
        <Button @click="vote">Vote</Button>
      </div>
    </section>

    <details class="rounded-lg border border-border bg-card p-4">
      <summary class="cursor-pointer text-sm font-medium text-foreground">Create / manage</summary>
      <div class="mt-3 space-y-5">
        <div class="space-y-2">
          <h3 class="text-xs font-medium text-muted-foreground">Create project (1 ZNN fee)</h3>
          <div class="grid grid-cols-2 gap-2">
            <Field label="Name"
              ><Input v-model="cName" placeholder="name" aria-label="create name"
            /></Field>
            <Field label="URL"
              ><Input v-model="cUrl" placeholder="url" aria-label="create url"
            /></Field>
            <Field label="ZNN needed (base units)"
              ><Input
                v-model="cZnn"
                placeholder="ZNN needed (base units)"
                aria-label="create znn"
            /></Field>
            <Field label="QSR needed (base units)"
              ><Input
                v-model="cQsr"
                placeholder="QSR needed (base units)"
                aria-label="create qsr"
            /></Field>
          </div>
          <Field label="Description"
            ><Input v-model="cDesc" placeholder="description" aria-label="create description"
          /></Field>
          <Button @click="createProject">Create project</Button>
        </div>
        <div class="space-y-2">
          <h3 class="text-xs font-medium text-muted-foreground">Add / update phase</h3>
          <p class="text-xs text-muted-foreground">
            Both use the project id; Update phase edits the project's current (voting) phase.
          </p>
          <Field label="Project id"
            ><Input v-model="phProjectId" placeholder="project id" aria-label="project id"
          /></Field>
          <div class="grid grid-cols-2 gap-2">
            <Field label="Name"
              ><Input v-model="phName" placeholder="name" aria-label="phase name"
            /></Field>
            <Field label="URL"
              ><Input v-model="phUrl" placeholder="url" aria-label="phase url"
            /></Field>
            <Field label="ZNN needed (base units)"
              ><Input
                v-model="phZnn"
                placeholder="ZNN needed (base units)"
                aria-label="phase znn"
            /></Field>
            <Field label="QSR needed (base units)"
              ><Input
                v-model="phQsr"
                placeholder="QSR needed (base units)"
                aria-label="phase qsr"
            /></Field>
          </div>
          <Field label="Description"
            ><Input v-model="phDesc" placeholder="description" aria-label="phase description"
          /></Field>
          <div class="flex gap-2">
            <Button variant="outline" @click="addPhase">Add phase</Button>
            <Button variant="outline" @click="updatePhase">Update phase</Button>
          </div>
        </div>
      </div>
    </details>

    <p v-if="error || accError" class="text-sm text-destructive" role="alert">
      {{ error || accError }}
    </p>

    <p v-if="tx.status === 'preparing'" class="text-muted-foreground">
      Preparing… (PoW if required)
    </p>
    <p v-if="tx.status === 'error'" class="text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
