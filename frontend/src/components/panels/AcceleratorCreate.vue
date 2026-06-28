<script setup lang="ts">
import { ref, watch } from 'vue'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()
const error = ref('')

// create project
const cName = ref('')
const cDesc = ref('')
const cUrl = ref('')
const cZnn = ref('')
const cQsr = ref('')
// add/update phase — both keyed by the PROJECT id on-chain
const phProjectId = ref('')
const phName = ref('')
const phDesc = ref('')
const phUrl = ref('')
const phZnn = ref('')
const phQsr = ref('')

function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e)
}
async function createProject() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareCreateProject(cName.value, cDesc.value, cUrl.value, cZnn.value, cQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function addPhase() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareAddPhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function updatePhase() {
  error.value = ''
  try {
    tx.awaitConfirm(await Nom.PrepareUpdatePhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') acc.loadProjects()
  },
)
</script>

<template>
  <div class="space-y-5 p-4">
    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h3 class="text-sm font-medium text-foreground">Post an AZ (create project · 1 ZNN fee)</h3>
      <div class="grid grid-cols-2 gap-2">
        <Field label="Name"><Input v-model="cName" placeholder="name" aria-label="create name" /></Field>
        <Field label="URL"><Input v-model="cUrl" placeholder="url" aria-label="create url" /></Field>
        <Field label="ZNN needed (base units)"><Input v-model="cZnn" placeholder="ZNN needed" aria-label="create znn" /></Field>
        <Field label="QSR needed (base units)"><Input v-model="cQsr" placeholder="QSR needed" aria-label="create qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="cDesc" placeholder="description" aria-label="create description" /></Field>
      <Button aria-label="create project" @click="createProject">Create project</Button>
    </section>

    <section class="space-y-2 rounded-lg border border-border bg-card p-4">
      <h3 class="text-sm font-medium text-foreground">Submit / update a phase</h3>
      <p class="text-xs text-muted-foreground">
        Both use the project id; Update edits the project's current (voting) phase. Only the project
        owner can add or update phases (enforced on-chain).
      </p>
      <Field label="Project id"><Input v-model="phProjectId" placeholder="project id (0x…)" aria-label="project id" /></Field>
      <div class="grid grid-cols-2 gap-2">
        <Field label="Name"><Input v-model="phName" placeholder="name" aria-label="phase name" /></Field>
        <Field label="URL"><Input v-model="phUrl" placeholder="url" aria-label="phase url" /></Field>
        <Field label="ZNN needed (base units)"><Input v-model="phZnn" placeholder="ZNN needed" aria-label="phase znn" /></Field>
        <Field label="QSR needed (base units)"><Input v-model="phQsr" placeholder="QSR needed" aria-label="phase qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="phDesc" placeholder="description" aria-label="phase description" /></Field>
      <div class="flex gap-2">
        <Button variant="outline" aria-label="add phase" @click="addPhase">Add phase</Button>
        <Button variant="outline" aria-label="update phase" @click="updatePhase">Update phase</Button>
      </div>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
