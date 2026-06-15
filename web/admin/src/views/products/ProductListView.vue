<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useAuthStore } from '@/stores/auth'
import type { components } from '@teggo/api/schema'
import ProductFormDialog from './ProductFormDialog.vue'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type AdminProduct = components['schemas']['AdminProduct']

const route = useRoute()
const router = useRouter()
const products = ref<AdminProduct[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')

const dialogOpen = ref(false)
const editing = ref<AdminProduct | null>(null)
const term = ref('')
// Row-expansion reveals the fields not worth a column (description, attributes, cost).
const expandedRows = ref<Record<string, boolean>>({})

const toast = useToast()
const confirm = useConfirm()

// ---- Bulk CSV import / export ----
const auth = useAuthStore()
const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''
const src = (u?: string | null) => (u ? `${apiBase}${u}` : '')
const exporting = ref(false)

// Bulk import now runs through the generic import engine (dry-run → commit).
function goImport() {
  router.push({ name: 'imports', query: { target: 'products' } })
}
async function exportCsv() {
  exporting.value = true
  try {
    const res = await fetch(`${apiBase}/admin/products/export`, {
      headers: { Authorization: `Bearer ${auth.token ?? ''}` },
    })
    if (!res.ok) throw new Error(`Export failed (${res.status})`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'products.csv'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Export failed'), life: 4000 })
  } finally {
    exporting.value = false
  }
}
async function load() {
  loading.value = true
  error.value = ''
  const query: { page: number; page_size: number; q?: string } = { page: 1, page_size: 100 }
  const q = term.value.trim()
  if (q) query.q = q
  const { data, error: err } = await api.GET('/admin/products', { params: { query } })
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load products')
    return
  }
  products.value = data.items
  total.value = data.total ?? data.items.length
}

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}
function openEdit(p: AdminProduct) {
  editing.value = p
  dialogOpen.value = true
}

function confirmDelete(p: AdminProduct) {
  confirm.require({
    message: `Delete product "${p.name}"? It will be soft-deleted.`,
    header: 'Confirm delete',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      const { error: err } = await api.DELETE('/admin/products/{id}', {
        params: { path: { id: p.id } },
      })
      if (err) {
        toast.add({ severity: 'error', summary: 'Delete failed', detail: errMessage(err), life: 4000 })
        return
      }
      toast.add({ severity: 'success', summary: 'Deleted', detail: p.name, life: 2500 })
      load()
    },
  })
}

function onSaved() {
  dialogOpen.value = false
  load()
}

function statusSeverity(s: string) {
  return s === 'active' ? 'success' : s === 'draft' ? 'warn' : 'danger'
}

onMounted(load)
// Opened from the dashboard "New product" quick action.
onMounted(() => { if (route.query.new) openCreate() })
</script>

<template>
  <div class="page">
    <PageHeader title="Products" :meta="total">
      <template #actions>
        <span class="p-input-icon-left search">
          <InputText
            v-model="term"
            placeholder="Search products…"
            @keyup.enter="load"
          />
          <Button icon="pi pi-search" severity="secondary" text @click="load" />
        </span>
        <Button icon="pi pi-download" label="Export" severity="secondary" outlined :loading="exporting" @click="exportCsv" />
        <Button icon="pi pi-upload" label="Import" severity="secondary" outlined @click="goImport" />
        <Button icon="pi pi-plus" label="New product" @click="openCreate" />
      </template>
    </PageHeader>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable
      v-model:expandedRows="expandedRows"
      :value="products"
      :loading="loading"
      paginator
      :rows="10"
      :rowsPerPageOptions="[10, 25, 50]"
      dataKey="id"
      stripedRows
      removableSort
    >
      <template #empty>
        <EmptyState icon="pi pi-box" title="No products yet" message="Add a product to your catalog, or import them from your ERP.">
          <Button icon="pi pi-plus" label="New product" @click="openCreate" />
        </EmptyState>
      </template>
      <template #expansion="{ data }">
        <div class="expand">
          <div v-if="data.description" class="ex-row"><span class="ex-k">Description</span><span>{{ data.description }}</span></div>
          <div class="ex-row"><span class="ex-k">Slug</span><span>{{ data.slug }}</span></div>
          <div v-if="data.cost_price && data.cost_price !== '0'" class="ex-row"><span class="ex-k">Cost price</span><span>{{ data.cost_price }}</span></div>
          <div v-if="Object.keys(data.attributes ?? {}).length" class="ex-row">
            <span class="ex-k">Attributes</span>
            <span class="ex-tags"><Tag v-for="(v, k) in data.attributes" :key="k" :value="`${k}: ${v}`" severity="secondary" /></span>
          </div>
        </div>
      </template>
      <Column expander style="width: 3rem" />
      <Column header="" style="width: 3.5rem">
        <template #body="{ data }">
          <img v-if="data.image" :src="src(data.image)" :alt="data.name" class="thumb" loading="lazy" />
          <span v-else class="thumb-ph"><i class="pi pi-image" /></span>
        </template>
      </Column>
      <Column field="sku" header="SKU" sortable />
      <Column field="name" header="Name" sortable />
      <Column field="type" header="Type" sortable />
      <Column field="unit" header="Unit" />
      <Column header="Status" sortable field="status">
        <template #body="{ data }">
          <Tag :value="data.status" :severity="statusSeverity(data.status)" />
        </template>
      </Column>
      <Column header="" style="width: 7rem">
        <template #body="{ data }">
          <Button icon="pi pi-pencil" severity="secondary" text rounded @click="openEdit(data)" />
          <Button icon="pi pi-trash" severity="danger" text rounded @click="confirmDelete(data)" />
        </template>
      </Column>
    </DataTable>

    <ProductFormDialog v-model:open="dialogOpen" :product="editing" @saved="onSaved" />
  </div>
</template>

<style scoped>
.mb {
  margin-bottom: 1rem;
}
.muted {
  color: var(--p-text-muted-color, #64748b);
}
.thumb {
  width: 2.4rem;
  height: 2.4rem;
  object-fit: cover;
  border-radius: 6px;
  display: block;
}
.thumb-ph {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.4rem;
  height: 2.4rem;
  border-radius: 6px;
  background: var(--p-surface-100, #f1f5f9);
  color: var(--p-surface-300, #cbd5e1);
}
.expand {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.5rem 0.25rem;
}
.ex-row {
  display: flex;
  gap: 1rem;
  align-items: baseline;
}
.ex-k {
  flex: 0 0 8rem;
  font-size: 0.78rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  color: var(--p-text-muted-color, #94a3b8);
}
.ex-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
</style>
