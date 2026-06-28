# Nav Underline Restyle — Design Spec

**Date:** 2026-06-28
**Status:** Approved design, pre-implementation
**Branch:** `ui-nav-underline` (off `main`)

## Goal

Restyle the top Network-of-Momentum navigation bar in `Home.vue` to match the reference:
tab labels **evenly spaced across the full width**, the **selected** tab shown in **green
with a green underline**, others muted gray, and a thin divider line under the whole row.
The bar must **respace automatically** when the optional Governance tab is shown/hidden.

## Approach

Use nom-ui's existing **`variant="underline"`** Tabs styling (already in the library, built on
reka-ui) instead of the current default "pill" variant. The variant provides exactly the
target look:

- `TabsList variant="underline"` → `flex w-full justify-stretch border-b …` (full-width row,
  bottom divider, horizontal-scroll fallback if it ever overflows).
- `TabsTrigger variant="underline"` → `flex-1 … border-b-2 border-transparent
  text-muted-foreground hover:text-foreground data-[state=active]:border-primary
  data-[state=active]:text-primary`.

`flex-1` per trigger gives even spacing AND automatic respacing when the reactive `TABS`
array changes from 7→8 (Governance toggle) — no extra logic. `data-[state=active]:*` gives the
green text + green underline on selection.

## Change

`frontend/src/views/Home.vue` only — the NoM tab bar:

```vue
<TabsList variant="underline">
  <TabsTrigger v-for="t in TABS" :key="t" variant="underline" :value="t">{{ t }}</TabsTrigger>
</TabsList>
```

Removing the current `class="w-full justify-start overflow-x-auto"` (the variant supplies
width + overflow handling). No other component, store, backend, or the panel sub-tab rows
(Vote/Actions/Propose/Create) change — those keep the default pill style (scope: main nav only).

## Testing

- `pnpm run typecheck`, `pnpm test` (existing `Home.test.ts` tab tests stay green — its nom-ui
  stubs ignore the variant prop, so behavior is unchanged), `pnpm run build`.
- Visual confirmation in `wails dev` (running): even spacing, green active text + underline,
  divider line; toggle Governance in Settings and confirm the row respaces.

## Out of scope

- Panel sub-tab rows (Accelerator/Governance Vote/Actions/Propose/Create).
- Any color/typography changes beyond adopting the existing underline variant.
