<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Card from 'primevue/card'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { useAuthStore } from '@/stores/auth'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'

type Dataset = components['schemas']['ExportDataset']

const auth = useAuthStore()
const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''

const datasets = ref<Dataset[]>([])
const loading = ref(false)
const error = ref('')
// Tracks the in-flight "<key>:<format>" download so only that button spins.
const busy = ref('')

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: e } = await api.GET('/admin/exports')
  loading.value = false
  if (e) {
    error.value = errMessage(e, 'Failed to load exports')
    return
  }
  datasets.value = data?.datasets ?? []
}

async function download(ds: Dataset, format: string) {
  busy.value = `${ds.key}:${format}`
  error.value = ''
  try {
    const res = await fetch(`${apiBase}/admin/exports/${ds.key}?format=${format}`, {
      headers: { Authorization: `Bearer ${auth.token ?? ''}` },
    })
    if (!res.ok) throw new Error(`Export failed (${res.status})`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${ds.key}.${format}`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Export failed'
  } finally {
    busy.value = ''
  }
}

function iconFor(key: string): string {
  switch (key) {
    case 'orders': return 'pi pi-shopping-cart'
    case 'order-items': return 'pi pi-list'
    case 'customers': return 'pi pi-building'
    case 'invoices': return 'pi pi-receipt'
    default: return 'pi pi-database'
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Data export" meta="your data, in your tools" />

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <p class="lede">
      Export the full record set for any dataset as CSV or Excel — drop it straight into your finance,
      budgeting or BI tools. Exports are scoped to your organization and to what you're permitted to see.
    </p>

    <p v-if="!loading && datasets.length === 0" class="muted empty">
      You don't have permission to export any datasets yet.
    </p>

    <div class="grid">
      <Card v-for="ds in datasets" :key="ds.key" class="ds">
        <template #content>
          <div class="ds-head">
            <span class="ds-icon"><i :class="iconFor(ds.key)" /></span>
            <div class="ds-id">
              <div class="ds-label">{{ ds.label }}</div>
              <div class="ds-desc">{{ ds.description }}</div>
            </div>
          </div>
          <div class="ds-actions">
            <Button
              label="CSV" icon="pi pi-download" size="small" severity="secondary" outlined
              :loading="busy === `${ds.key}:csv`" @click="download(ds, 'csv')"
            />
            <Button
              label="Excel" icon="pi pi-file-excel" size="small" severity="secondary" outlined
              :loading="busy === `${ds.key}:xlsx`" @click="download(ds, 'xlsx')"
            />
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.lede { color: var(--p-text-muted-color, #64748b); max-width: 46rem; line-height: 1.6; margin: 0.25rem 0 1.5rem; }
.empty { margin-top: 1rem; }
.muted { color: var(--p-text-muted-color, #94a3b8); }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
.ds { border: 1px solid var(--p-surface-200, #e2e8f0); }
.ds-head { display: flex; gap: 0.8rem; align-items: flex-start; margin-bottom: 1rem; }
.ds-icon {
  display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0;
  width: 38px; height: 38px; border-radius: 9px;
  color: var(--p-primary-color, #6366f1); font-size: 1.35rem;
}
.ds-id { min-width: 0; }
.ds-label { font-weight: 700; }
.ds-desc { color: var(--p-text-muted-color, #64748b); font-size: 0.85rem; margin-top: 0.15rem; line-height: 1.45; }
.ds-actions { display: flex; gap: 0.5rem; }
</style>
