<script setup lang="ts">
import { useNodeRefresh } from '../../lib/useNodeRefresh'
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { Input, Button } from 'nom-ui'
import * as Nom from '../../../wailsjs/go/app/NomService'
import { useAcceleratorStore } from '../../stores/accelerator'
import { useTxStore } from '../../stores/tx'
import Field from '../Field.vue'

const acc = useAcceleratorStore()
const tx = useTxStore()
const { myProjects } = storeToRefs(acc)
const error = ref('')

useNodeRefresh(() => acc.loadMyProjects())

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
const selectedProject = computed(() => myProjects.value.some(p => p.id === phProjectId.value))
watch(myProjects, (projects) => {
  if (!projects.some(p => p.id === phProjectId.value)) phProjectId.value = ''
}, { flush: 'sync' })

function fail(e: unknown) {
  error.value = e instanceof Error ? e.message : String(e)
}
async function createProject() {
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareCreateProject(cName.value, cDesc.value, cUrl.value, cZnn.value, cQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function addPhase() {
  if (!selectedProject.value) return
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareAddPhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
async function updatePhase() {
  if (!selectedProject.value) return
  error.value = ''
  try {
    await tx.prepareCall(() => Nom.PrepareUpdatePhase(phProjectId.value, phName.value, phDesc.value, phUrl.value, phZnn.value, phQsr.value))
  } catch (e) {
    fail(e)
  }
}
watch(
  () => tx.status,
  (s) => {
    if (s === 'done') {
      acc.loadProjects()
      acc.loadMyProjects()
    }
  },
)
</script>

<template>
  <div class="space-y-5">
    <section class="space-y-3 panel-card">
      <h3 class="section-title">Post an AZ (create project · 1 ZNN fee)</h3>
      <div class="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
        <Field label="Name"><Input v-model="cName" placeholder="name" aria-label="create name" /></Field>
        <Field label="URL"><Input v-model="cUrl" placeholder="url" aria-label="create url" /></Field>
        <Field label="ZNN needed (base units)"><Input v-model="cZnn" placeholder="ZNN needed" aria-label="create znn" /></Field>
        <Field label="QSR needed (base units)"><Input v-model="cQsr" placeholder="QSR needed" aria-label="create qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="cDesc" placeholder="description" aria-label="create description" /></Field>
      <Button aria-label="create project" @click="createProject">Create project</Button>
    </section>

    <section class="space-y-3 panel-card">
      <h3 class="section-title">Request a phase payout</h3>
      <p class="text-xs text-muted-foreground">
        Pick one of your approved projects and submit a phase requesting its payout — pillars vote and
        approved phases pay out automatically. "Update phase" edits your current (still-voting) phase
        instead. Only the project owner can do this (enforced on-chain).
      </p>
      <Field label="Project">
        <select
          v-model="phProjectId"
          aria-label="project id"
          class="select-field"
        >
          <option value="" disabled>Select an approved project…</option>
          <option v-for="p in myProjects" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
      </Field>
      <p v-if="myProjects.length === 0" class="text-xs text-muted-foreground">
        No approved projects of yours yet. Create one above and wait for pillars to approve it before
        requesting a phase payout.
      </p>
      <div class="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
        <Field label="Phase name"><Input v-model="phName" placeholder="name" aria-label="phase name" /></Field>
        <Field label="URL"><Input v-model="phUrl" placeholder="url" aria-label="phase url" /></Field>
        <Field label="ZNN payout (base units)"><Input v-model="phZnn" placeholder="ZNN needed" aria-label="phase znn" /></Field>
        <Field label="QSR payout (base units)"><Input v-model="phQsr" placeholder="QSR needed" aria-label="phase qsr" /></Field>
      </div>
      <Field label="Description"><Input v-model="phDesc" placeholder="what this phase delivers" aria-label="phase description" /></Field>
      <div class="flex gap-2">
        <Button aria-label="add phase" :disabled="!selectedProject" @click="addPhase">Request phase payout</Button>
        <Button variant="outline" aria-label="update phase" :disabled="!selectedProject" @click="updatePhase">Update phase</Button>
      </div>
    </section>

    <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
    <p v-if="tx.status === 'error'" class="text-sm text-destructive" role="alert">{{ tx.error }}</p>
  </div>
</template>
