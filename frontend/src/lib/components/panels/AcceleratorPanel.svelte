<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../../../wailsjs/go/app/NomService'
  import { projects, selectedProject, votablePillars, accError, loadProjects, openProject, loadVotablePillars } from '../../stores/accelerator'
  import { tx, awaitConfirm } from '../../stores/tx'
  import { formatAmount } from '../../format'
  import Field from '../ui/Field.svelte'
  import Input from '../ui/Input.svelte'
  import Button from '../ui/Button.svelte'

  let error = ''
  // donate
  let donateAmount = '', donateToken = 'QSR'
  // vote
  let voteId = '', votePillar = '', voteChoice = 0 // 0=yes,1=no,2=abstain (embedded.Vote*)
  // create project
  let cName = '', cDesc = '', cUrl = '', cZnn = '', cQsr = ''
  // add/update phase — both are keyed by the PROJECT id on-chain (UpdatePhase
  // updates the project's current phase)
  let phProjectId = '', phName = '', phDesc = '', phUrl = '', phZnn = '', phQsr = ''

  const STATUS = ['Voting', 'Active', 'Paid', 'Closed', 'Completed']
  function statusLabel(n: number) { return STATUS[n] ?? `#${n}` }

  onMount(() => { loadProjects(); loadVotablePillars() })
  $: if ($tx.status === 'done') { loadProjects(); loadVotablePillars() }
  $: if ($votablePillars.length > 0 && votePillar === '') votePillar = $votablePillars[0]

  function fail(e: any) { error = e?.message ?? String(e) }

  async function donate() {
    error = ''
    try { awaitConfirm((await Nom.PrepareDonate(donateAmount, donateToken)) as any) } catch (e) { fail(e) }
  }
  async function vote() {
    error = ''
    try { awaitConfirm((await Nom.PrepareVote(voteId, votePillar, voteChoice)) as any) } catch (e) { fail(e) }
  }
  async function createProject() {
    error = ''
    try { awaitConfirm((await Nom.PrepareCreateProject(cName, cDesc, cUrl, cZnn, cQsr)) as any) } catch (e) { fail(e) }
  }
  async function addPhase() {
    error = ''
    try { awaitConfirm((await Nom.PrepareAddPhase(phProjectId, phName, phDesc, phUrl, phZnn, phQsr)) as any) } catch (e) { fail(e) }
  }
  async function updatePhase() {
    error = ''
    try { awaitConfirm((await Nom.PrepareUpdatePhase(phProjectId, phName, phDesc, phUrl, phZnn, phQsr)) as any) } catch (e) { fail(e) }
  }
</script>

<div class="space-y-4 p-4">
  <section class="space-y-2 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Projects</h2>
    {#each $projects as p}
      <div class="space-y-1 border-b border-border/60 py-2 text-sm last:border-b-0">
        <p class="font-mono text-text">{p.name} · {statusLabel(p.status)} · {formatAmount(p.znnFundsNeeded, 8)} ZNN / {formatAmount(p.qsrFundsNeeded, 8)} QSR</p>
        <p class="text-xs text-muted">votes: {p.votes.yes} yes / {p.votes.no} no / {p.votes.total} total</p>
        <Button variant="outline" class="px-2 py-1 text-xs" on:click={() => openProject(p.id)} aria-label={`open ${p.name}`}>Phases</Button>
        {#if $selectedProject && $selectedProject.id === p.id}
          {#each $selectedProject.phases as ph}
            <div class="ml-4 mt-1 text-xs text-muted">
              {ph.name} · {statusLabel(ph.status)} · {ph.votes.yes}/{ph.votes.no}/{ph.votes.total} · <span class="font-mono">{ph.id}</span>
            </div>
          {:else}
            <p class="ml-4 mt-1 text-xs text-muted">No phases.</p>
          {/each}
        {/if}
      </div>
    {/each}
    {#if $projects.length === 0}<p class="text-xs text-muted">No projects.</p>{/if}
  </section>

  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Donate</h2>
    <div class="flex items-end gap-2">
      <div class="flex-1">
        <Field label="Amount (base units)">
          <Input bind:value={donateAmount} placeholder="amount (base units)" ariaLabel="donate amount" />
        </Field>
      </div>
      <select class="rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" bind:value={donateToken} aria-label="donate token">
        <option value="ZNN">ZNN</option>
        <option value="QSR">QSR</option>
      </select>
      <Button variant="primary" on:click={donate}>Donate</Button>
    </div>
  </section>

  {#if $votablePillars.length > 0}
  <section class="space-y-3 rounded-lg border border-border bg-surface p-4">
    <h2 class="text-sm font-medium text-text">Vote (Pillar operator)</h2>
    <div class="flex flex-wrap items-end gap-2">
      <div class="min-w-[18rem] flex-1">
        <Field label="Target id">
          <Input bind:value={voteId} placeholder="project or phase id (0x…)" ariaLabel="vote target id" />
        </Field>
      </div>
      <select class="rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" bind:value={votePillar} aria-label="vote pillar">
        {#each $votablePillars as name}<option value={name}>{name}</option>{/each}
      </select>
      <select class="rounded border border-border bg-elevated px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" bind:value={voteChoice} aria-label="vote choice">
        <option value={0}>Yes</option>
        <option value={1}>No</option>
        <option value={2}>Abstain</option>
      </select>
      <Button variant="primary" on:click={vote}>Vote</Button>
    </div>
  </section>
  {/if}

  <details class="rounded-lg border border-border bg-surface p-4">
    <summary class="cursor-pointer text-sm font-medium text-text">Create / manage</summary>
    <div class="mt-3 space-y-5">
      <div class="space-y-2">
        <h3 class="text-xs font-medium text-muted">Create project (1 ZNN fee)</h3>
        <div class="grid grid-cols-2 gap-2">
          <Field label="Name"><Input bind:value={cName} placeholder="name" ariaLabel="create name" /></Field>
          <Field label="URL"><Input bind:value={cUrl} placeholder="url" ariaLabel="create url" /></Field>
          <Field label="ZNN needed (base units)"><Input bind:value={cZnn} placeholder="ZNN needed (base units)" ariaLabel="create znn" /></Field>
          <Field label="QSR needed (base units)"><Input bind:value={cQsr} placeholder="QSR needed (base units)" ariaLabel="create qsr" /></Field>
        </div>
        <Field label="Description"><Input bind:value={cDesc} placeholder="description" ariaLabel="create description" /></Field>
        <Button variant="primary" on:click={createProject}>Create project</Button>
      </div>
      <div class="space-y-2">
        <h3 class="text-xs font-medium text-muted">Add / update phase</h3>
        <p class="text-xs text-muted">Both use the project id; Update phase edits the project's current (voting) phase.</p>
        <Field label="Project id"><Input bind:value={phProjectId} placeholder="project id" ariaLabel="project id" /></Field>
        <div class="grid grid-cols-2 gap-2">
          <Field label="Name"><Input bind:value={phName} placeholder="name" ariaLabel="phase name" /></Field>
          <Field label="URL"><Input bind:value={phUrl} placeholder="url" ariaLabel="phase url" /></Field>
          <Field label="ZNN needed (base units)"><Input bind:value={phZnn} placeholder="ZNN needed (base units)" ariaLabel="phase znn" /></Field>
          <Field label="QSR needed (base units)"><Input bind:value={phQsr} placeholder="QSR needed (base units)" ariaLabel="phase qsr" /></Field>
        </div>
        <Field label="Description"><Input bind:value={phDesc} placeholder="description" ariaLabel="phase description" /></Field>
        <div class="flex gap-2">
          <Button variant="outline" on:click={addPhase}>Add phase</Button>
          <Button variant="outline" on:click={updatePhase}>Update phase</Button>
        </div>
      </div>
    </div>
  </details>

  {#if error || $accError}<p class="text-sm text-error" role="alert">{error || $accError}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
</div>
