<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Summary = components['schemas']['CatalogHealthSummary']
type Item = components['schemas']['CatalogHealthItem']

const loading = ref(false)
const error = ref('')
const summary = ref<Summary | null>(null)
const worst = ref<Item[]>([])

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: e } = await api.GET('/admin/data-health/catalog', {
    params: { query: { limit: 50 } },
  })
  loading.value = false
  if (e) {
    error.value = errMessage(e, 'Failed to load catalog health')
    return
  }
  summary.value = data?.summary ?? null
  worst.value = data?.worst ?? []
}

const avg = computed(() => Math.round((summary.value?.avg_completeness ?? 0) * 10) / 10)
const scored = computed(() => summary.value?.products_scored ?? 0)
const tier = (pct: number) => (pct >= 80 ? 'good' : pct >= 50 ? 'warn' : 'bad')

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Catalog data health">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text :loading="loading" @click="load" />
      </template>
    </PageHeader>
    <p class="muted">
      How complete your catalog is against the <strong>required</strong> attributes each product's
      family declares. Fix the worst offenders below to lift the score — every filled attribute is
      data your storefront, search and channels can actually use.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div v-if="summary" class="stats">
      <div class="stat hero" :class="tier(avg)">
        <div class="stat-value">{{ avg }}<span class="pct">%</span></div>
        <div class="stat-label">Average completeness</div>
        <div class="bar"><div class="bar-fill" :style="{ width: avg + '%' }" /></div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ scored }}</div>
        <div class="stat-label">Products scored</div>
      </div>
      <div class="stat">
        <div class="stat-value ok">{{ summary.complete_count ?? 0 }}</div>
        <div class="stat-label">Fully complete</div>
      </div>
      <div class="stat">
        <div class="stat-value bad-text">{{ summary.incomplete_count ?? 0 }}</div>
        <div class="stat-label">Incomplete</div>
      </div>
      <div class="stat">
        <div class="stat-value">{{ summary.products_with_family ?? 0 }} / {{ summary.products_total ?? 0 }}</div>
        <div class="stat-label">With a family / total</div>
      </div>
    </div>

    <h3>Needs attention</h3>
    <DataTable :value="worst" dataKey="id" stripedRows :loading="loading">
      <template #empty>
        <EmptyState
          icon="pi pi-verified"
          title="Nothing incomplete"
          message="Every scored product has all its required attributes filled. Nice and tidy."
        />
      </template>
      <Column field="sku" header="SKU" style="width: 10rem" />
      <Column field="name" header="Product" />
      <Column header="Completeness" style="width: 16rem">
        <template #body="{ data }">
          <div class="cell-complete">
            <div class="bar"><div class="bar-fill" :class="tier(data.completeness ?? 0)" :style="{ width: (data.completeness ?? 0) + '%' }" /></div>
            <span class="cell-pct">{{ data.completeness ?? 0 }}%</span>
            <span class="muted cell-frac">{{ data.required_present ?? 0 }}/{{ data.required_total ?? 0 }}</span>
          </div>
        </template>
      </Column>
      <Column header="Missing required">
        <template #body="{ data }">
          <Tag v-for="code in data.missing ?? []" :key="code" :value="code" severity="warn" class="miss" />
        </template>
      </Column>
    </DataTable>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
h3 { margin: 1.5rem 0 0.5rem; }

.stats {
  display: grid;
  grid-template-columns: 1.6fr repeat(4, 1fr);
  gap: 0.75rem;
  margin: 1rem 0 0.5rem;
}
@media (max-width: 900px) { .stats { grid-template-columns: 1fr 1fr; } }
.stat {
  border: 1px solid var(--teggo-border, #e2e8f0);
  border-radius: var(--teggo-radius, 3px);
  background: var(--teggo-surface, #fff);
  padding: 0.9rem 1rem;
}
.stat-value { font-size: 1.5rem; font-weight: 800; letter-spacing: -0.02em; }
.stat-value .pct { font-size: 1rem; font-weight: 700; margin-left: 1px; }
.stat-value.ok { color: #15803d; }
.stat-value.bad-text { color: #b91c1c; }
.stat-label { color: var(--p-text-muted-color, #64748b); font-size: 0.8rem; margin-top: 0.15rem; }
.stat.hero .stat-value { font-size: 2.2rem; }
.stat.hero.good .stat-value { color: #15803d; }
.stat.hero.warn .stat-value { color: #b45309; }
.stat.hero.bad .stat-value { color: #b91c1c; }

.bar { height: 6px; border-radius: 999px; background: var(--p-surface-200, #e2e8f0); overflow: hidden; margin-top: 0.5rem; }
.bar-fill { height: 100%; background: var(--p-primary-color, #0e7490); border-radius: 999px; }
.bar-fill.good { background: #16a34a; }
.bar-fill.warn { background: #d97706; }
.bar-fill.bad { background: #dc2626; }

.cell-complete { display: flex; align-items: center; gap: 0.5rem; }
.cell-complete .bar { flex: 1; margin-top: 0; }
.cell-pct { font-weight: 700; font-size: 0.85rem; min-width: 2.8rem; text-align: right; }
.cell-frac { font-size: 0.78rem; min-width: 2.5rem; }
.miss { margin: 0 0.25rem 0.25rem 0; }
</style>
