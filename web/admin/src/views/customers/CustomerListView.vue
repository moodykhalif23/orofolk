<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import CustomerFormDialog from './CustomerFormDialog.vue'

type Customer = components['schemas']['Customer']

const router = useRouter()
const toast = useToast()
const confirm = useConfirm()

const rows = ref<Customer[]>([])
const total = ref(0)
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const editing = ref<Customer | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/customers', {
    params: { query: { page: 1, page_size: 100 } },
  })
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load customers')
    return
  }
  rows.value = data.items
  total.value = data.total ?? data.items.length
}

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}
function openEdit(c: Customer) {
  editing.value = c
  dialogOpen.value = true
}
function openDetail(c: Customer) {
  router.push({ name: 'customer-detail', params: { id: c.id } })
}

function confirmDelete(c: Customer) {
  confirm.require({
    message: `Delete customer "${c.name}"? It will be soft-deleted.`,
    header: 'Confirm delete',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Delete', severity: 'danger' },
    accept: async () => {
      const { error: err } = await api.DELETE('/admin/customers/{id}', { params: { path: { id: c.id } } })
      if (err) {
        toast.add({ severity: 'error', summary: 'Delete failed', detail: errMessage(err), life: 4000 })
        return
      }
      toast.add({ severity: 'success', summary: 'Deleted', detail: c.name, life: 2500 })
      load()
    },
  })
}

function onSaved() {
  dialogOpen.value = false
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>Customers <span class="muted">({{ total }})</span></h1>
      <div class="actions">
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New customer" @click="openCreate" />
      </div>
    </div>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable
      :value="rows"
      :loading="loading"
      paginator
      :rows="10"
      :rowsPerPageOptions="[10, 25, 50]"
      dataKey="id"
      stripedRows
      removableSort
      @rowClick="openDetail($event.data)"
      class="clickable"
    >
      <template #empty>No customers yet — create one.</template>
      <Column field="name" header="Name" sortable />
      <Column field="tax_id" header="Tax ID" />
      <Column header="Terms">
        <template #body="{ data }">{{ data.payment_terms_days }}d</template>
      </Column>
      <Column header="Credit limit">
        <template #body="{ data }">{{ data.credit_limit }}</template>
      </Column>
      <Column header="Active">
        <template #body="{ data }">
          <Tag :value="data.is_active ? 'active' : 'inactive'" :severity="data.is_active ? 'success' : 'secondary'" />
        </template>
      </Column>
      <Column header="" style="width: 7rem">
        <template #body="{ data }">
          <Button icon="pi pi-pencil" severity="secondary" text rounded @click.stop="openEdit(data)" />
          <Button icon="pi pi-trash" severity="danger" text rounded @click.stop="confirmDelete(data)" />
        </template>
      </Column>
    </DataTable>

    <CustomerFormDialog v-model:open="dialogOpen" :customer="editing" @saved="onSaved" />
  </div>
</template>

<style scoped>
.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
.header h1 { margin: 0; }
.actions { display: flex; gap: 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 1rem; }
.mb { margin-bottom: 1rem; }
.clickable :deep(tbody tr) { cursor: pointer; }
</style>
