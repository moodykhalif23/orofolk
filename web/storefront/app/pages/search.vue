<script setup lang="ts">
import ProductCard from '~/components/ProductCard.vue'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Button from 'primevue/button'
import type { components } from '@teggo/api/schema'

type Facet = components['schemas']['Facet']
interface RangeSel { attr: string; min: number; max: number }

const route = useRoute()
const router = useRouter()
const client = useClient()

const q = computed(() => (route.query.q as string)?.trim() ?? '')
const sort = computed(() => (route.query.sort as string) || 'relevance')

// Selected attribute facets come from the URL (?filter=<json>) so results are
// shareable and SSR-stable.
const selected = computed<Record<string, string>>(() => {
  try {
    return route.query.filter ? JSON.parse(route.query.filter as string) : {}
  } catch {
    return {}
  }
})

const sortOptions = [
  { label: 'Relevance', value: 'relevance' },
  { label: 'Name', value: 'name' },
  { label: 'Newest', value: 'newest' },
]

const { data, error } = await useAsyncData(
  () => `catalog-${q.value}-${route.query.filter ?? ''}-${route.query.range ?? ''}-${sort.value}`,
  async () => {
    const query: Record<string, string | number> = { page: 1, page_size: 24, sort: sort.value }
    if (q.value) query.q = q.value
    if (route.query.filter) query.filter = route.query.filter as string
    if (route.query.range) query.range = route.query.range as string
    const { data, error } = await client.GET('/storefront/catalog', { params: { query } })
    if (error) throw createError({ statusCode: 502, statusMessage: 'Search unavailable' })
    return data
  },
  { watch: [() => route.query.q, () => route.query.filter, () => route.query.range, () => route.query.sort] },
)

useSeoMeta({
  title: () => (q.value ? `Search: ${q.value} — Teggo Store` : 'Catalog — Teggo Store'),
  description: () => `Product search and filtering${q.value ? ` for "${q.value}"` : ''}.`,
})

// Selected numeric range (one attribute at a time) from ?range=<json>.
const rangeSel = computed<RangeSel | null>(() => {
  try {
    return route.query.range ? JSON.parse(route.query.range as string) : null
  } catch {
    return null
  }
})

function navigate(next: { filter?: Record<string, string>; sort?: string; range?: RangeSel | null }) {
  const filter = next.filter ?? selected.value
  const range = next.range !== undefined ? next.range : rangeSel.value
  const query: Record<string, string> = {}
  if (q.value) query.q = q.value
  const s = (next.sort ?? route.query.sort) as string | undefined
  if (s) query.sort = s
  if (Object.keys(filter).length) query.filter = JSON.stringify(filter)
  if (range) query.range = JSON.stringify(range)
  router.push({ path: '/search', query })
}

function toggleFacet(attr: string, value: string) {
  const f = { ...selected.value }
  if (f[attr] === value) delete f[attr]
  else f[attr] = value
  navigate({ filter: f })
}

function clearFilters() {
  navigate({ filter: {}, range: null })
}

function isSelected(attr: string, value: string) {
  return selected.value[attr] === value
}

// ---- Numeric facets render a range Slider instead of value buttons ----
function isNumericFacet(f: Facet): boolean {
  return f.values.length >= 2 && f.values.every((v) => v.value !== '' && isFinite(Number(v.value)))
}
function bounds(f: Facet): { min: number; max: number } {
  const nums = f.values.map((v) => Number(v.value))
  return { min: Math.floor(Math.min(...nums)), max: Math.ceil(Math.max(...nums)) }
}
function rangeFor(f: Facet): number[] {
  const b = bounds(f)
  if (rangeSel.value?.attr === f.attr) {
    return [Math.max(b.min, rangeSel.value.min), Math.min(b.max, rangeSel.value.max)]
  }
  return [b.min, b.max]
}

// Local draft so the slider moves while dragging; committed to the URL on release.
const draft = reactive<Record<string, number[]>>({})
watchEffect(() => {
  for (const f of (data.value?.facets ?? []) as Facet[]) {
    if (isNumericFacet(f)) draft[f.attr] = rangeFor(f)
  }
})
function onRange(f: Facet) {
  const v = draft[f.attr] ?? rangeFor(f)
  const lo = v[0] ?? bounds(f).min
  const hi = v[1] ?? bounds(f).max
  const b = bounds(f)
  // A range spanning the full bounds means "no filter".
  navigate({ range: lo <= b.min && hi >= b.max ? null : { attr: f.attr, min: lo, max: hi } })
}
</script>

<template>
  <section>
    <div class="head">
      <h1 class="title">
        Catalog <span v-if="q" class="muted">— “{{ q }}”</span>
        <span v-if="data" class="muted count">({{ data.total }})</span>
      </h1>
      <Select
        :modelValue="sort"
        :options="sortOptions"
        optionLabel="label"
        optionValue="value"
        @update:modelValue="navigate({ sort: $event })"
      />
    </div>

    <Message v-if="error" severity="error" :closable="false">Search is unavailable right now.</Message>

    <div v-else class="layout">
      <aside class="facets">
        <div class="facets-head">
          <strong>Filters</strong>
          <Button v-if="Object.keys(selected).length" label="Clear" size="small" text @click="clearFilters" />
        </div>
        <div v-for="f in ((data?.facets ?? []) as Facet[])" :key="f.attr" class="facet">
          <div class="facet-name">{{ f.attr }}</div>
          <template v-if="isNumericFacet(f)">
            <Slider
              v-model="draft[f.attr]"
              range
              :min="bounds(f).min"
              :max="bounds(f).max"
              class="facet-slider"
              @slideend="onRange(f)"
            />
            <div class="range-labels">
              <span>{{ (draft[f.attr] ?? rangeFor(f))[0] }}</span>
              <span>{{ (draft[f.attr] ?? rangeFor(f))[1] }}</span>
            </div>
          </template>
          <template v-else>
            <button
              v-for="v in f.values"
              :key="v.value"
              class="facet-value"
              :class="{ on: isSelected(f.attr, v.value) }"
              @click="toggleFacet(f.attr, v.value)"
            >
              <span>{{ v.value }}</span>
              <span class="n">{{ v.count }}</span>
            </button>
          </template>
        </div>
        <p v-if="!(data?.facets?.length)" class="muted">No filters available.</p>
      </aside>

      <div class="results">
        <div v-if="data?.items?.length" class="grid">
          <ProductCard v-for="p in data.items" :key="p.public_id" :product="p" />
        </div>
        <p v-else class="muted">No products match your filters.</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.25rem; }
.title { margin: 0; }
.count { font-weight: 400; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.layout { display: grid; grid-template-columns: 220px 1fr; gap: 1.5rem; align-items: start; }
.facets { border: 1px solid var(--p-surface-200, #e2e8f0); border-radius: 10px; padding: 1rem; }
.facets-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem; }
.facet { margin-bottom: 1rem; }
.facet-name { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; color: var(--p-text-muted-color, #64748b); margin-bottom: 0.35rem; }
.facet-value {
  display: flex; justify-content: space-between; width: 100%; gap: 0.5rem;
  background: none; border: none; padding: 0.3rem 0.4rem; cursor: pointer; border-radius: 6px;
  text-align: left; font-size: 0.9rem; color: inherit;
}
.facet-value:hover { background: var(--p-surface-100, #f1f5f9); }
.facet-value.on { background: var(--p-primary-100, #e0e7ff); font-weight: 600; }
.facet-value .n { color: var(--p-text-muted-color, #64748b); }
.facet-slider { margin: 0.85rem 0.4rem 0.4rem; }
.range-labels {
  display: flex;
  justify-content: space-between;
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
  color: var(--p-text-muted-color, #64748b);
}
.results .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 1rem; }
@media (max-width: 720px) {
  .layout { grid-template-columns: 1fr; }
  .results .grid { grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); gap: 0.75rem; }
}
</style>
