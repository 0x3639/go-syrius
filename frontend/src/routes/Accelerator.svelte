<script lang="ts">
  import { onMount } from 'svelte'
  import * as Nom from '../../wailsjs/go/app/NomService'
  import { projects, selectedProject, votablePillars, accError, loadProjects, openProject, loadVotablePillars } from '../lib/stores/accelerator'
  import { tx, awaitConfirm } from '../lib/stores/tx'
  import { view } from '../lib/stores/nav'
  import { formatAmount } from '../lib/format'
  import TxModal from '../lib/components/TxModal.svelte'
  import TxResult from '../lib/components/TxResult.svelte'

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

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <h1 class="text-xl">Accelerator-Z</h1>
    <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={() => view.set('dashboard')}>Back</button>
  </div>

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Projects</h2>
    {#each $projects as p}
      <div class="border-b border-muted/20 py-2 text-sm space-y-1">
        <p class="font-mono">{p.name} · {statusLabel(p.status)} · {formatAmount(p.znnFundsNeeded, 8)} ZNN / {formatAmount(p.qsrFundsNeeded, 8)} QSR</p>
        <p class="text-xs text-muted">votes: {p.votes.yes} yes / {p.votes.no} no / {p.votes.total} total</p>
        <button class="rounded border border-muted/40 px-2 py-0.5 text-xs" on:click={() => openProject(p.id)} aria-label={`open ${p.name}`}>Phases</button>
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

  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Donate</h2>
    <div class="flex gap-2 items-center">
      <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="amount (base units)" bind:value={donateAmount} aria-label="donate amount" />
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={donateToken} aria-label="donate token">
        <option value="ZNN">ZNN</option>
        <option value="QSR">QSR</option>
      </select>
      <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={donate}>Donate</button>
    </div>
  </section>

  {#if $votablePillars.length > 0}
  <section class="rounded bg-surface p-4 space-y-2">
    <h2 class="text-sm text-muted">Vote (Pillar operator)</h2>
    <div class="flex flex-wrap gap-2 items-center">
      <input class="rounded bg-bg px-2 py-1 text-sm w-80" placeholder="project or phase id (0x…)" bind:value={voteId} aria-label="vote target id" />
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={votePillar} aria-label="vote pillar">
        {#each $votablePillars as name}<option value={name}>{name}</option>{/each}
      </select>
      <select class="rounded bg-bg px-2 py-1 text-sm" bind:value={voteChoice} aria-label="vote choice">
        <option value={0}>Yes</option>
        <option value={1}>No</option>
        <option value={2}>Abstain</option>
      </select>
      <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={vote}>Vote</button>
    </div>
  </section>
  {/if}

  <details class="rounded bg-surface p-4">
    <summary class="text-sm text-muted cursor-pointer">Create / manage</summary>
    <div class="mt-3 space-y-4">
      <div class="space-y-2">
        <h3 class="text-xs text-muted">Create project (1 ZNN fee)</h3>
        <div class="grid grid-cols-2 gap-2">
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={cName} aria-label="create name" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="url" bind:value={cUrl} aria-label="create url" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="ZNN needed (base units)" bind:value={cZnn} aria-label="create znn" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="QSR needed (base units)" bind:value={cQsr} aria-label="create qsr" />
        </div>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="description" bind:value={cDesc} aria-label="create description" />
        <button class="rounded bg-accent px-3 py-1 text-bg text-sm" on:click={createProject}>Create project</button>
      </div>
      <div class="space-y-2">
        <h3 class="text-xs text-muted">Add / update phase</h3>
        <p class="text-xs text-muted">Both use the project id; Update phase edits the project's current (voting) phase.</p>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="project id" bind:value={phProjectId} aria-label="project id" />
        <div class="grid grid-cols-2 gap-2">
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="name" bind:value={phName} aria-label="phase name" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="url" bind:value={phUrl} aria-label="phase url" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="ZNN needed (base units)" bind:value={phZnn} aria-label="phase znn" />
          <input class="rounded bg-bg px-2 py-1 text-sm" placeholder="QSR needed (base units)" bind:value={phQsr} aria-label="phase qsr" />
        </div>
        <input class="w-full rounded bg-bg px-2 py-1 text-sm" placeholder="description" bind:value={phDesc} aria-label="phase description" />
        <div class="flex gap-2">
          <button class="rounded border border-muted/40 px-3 py-1 text-sm" on:click={addPhase}>Add phase</button>
          <button class="rounded border border-muted/40 px-3 py-1 text-sm" on:click={updatePhase}>Update phase</button>
        </div>
      </div>
    </div>
  </details>

  {#if error || $accError}<p class="text-error text-sm" role="alert">{error || $accError}</p>{/if}

  {#if $tx.status === 'preparing'}<p class="text-muted">Preparing… (PoW if required)</p>{/if}
  {#if $tx.status === 'error'}<p class="text-error" role="alert">{$tx.error}</p>{/if}
  {#if $tx.status === 'awaiting' && $tx.preview}<TxModal />{/if}
  {#if $tx.status === 'done'}<TxResult />{/if}
</div>
