<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import { useAuthStore } from '@/stores/auth'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type CustomerGroup = components['schemas']['CustomerGroup']
interface GroupCustomer {
  id: number
  name: string
  is_active: boolean
  payment_terms_days: number
  credit_limit: string
}

const router = useRouter()
const auth = useAuthStore()
const rows = ref<CustomerGroup[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const name = ref('')
const toast = useToast()

// Members drill-in.
const membersOpen = ref(false)
const membersLoading = ref(false)
const membersError = ref('')
const activeGroup = ref<CustomerGroup | null>(null)
const members = ref<GroupCustomer[]>([])

const BASE = import.meta.env.VITE_API_BASE_URL ?? ''

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/customer-groups')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load groups')
    return
  }
  rows.value = data.items ?? []
}

function openCreate() {
  name.value = ''
  dialogOpen.value = true
}

async function save() {
  saving.value = true
  const { error: err } = await api.POST('/admin/customer-groups', { body: { name: name.value } })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  dialogOpen.value = false
  toast.add({ severity: 'success', summary: 'Group created', life: 2000 })
  load()
}

// Open the members drawer for a group and fetch its customers. The endpoint
// isn't in the generated client yet, so use a plain authenticated fetch.
async function openMembers(group: CustomerGroup) {
  activeGroup.value = group
  membersOpen.value = true
  members.value = []
  membersError.value = ''
  membersLoading.value = true
  try {
    const res = await fetch(`${BASE}/admin/customer-groups/${group.id}/customers`, {
      headers: auth.token ? { Authorization: `Bearer ${auth.token}` } : {},
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    members.value = (await res.json()).items ?? []
  } catch (e) {
    membersError.value = e instanceof Error ? e.message : 'Failed to load members'
  } finally {
    membersLoading.value = false
  }
}

function goToCustomer(c: GroupCustomer) {
  membersOpen.value = false
  router.push({ name: 'customer-detail', params: { id: c.id } }).catch(() => {
    router.push(`/customers/${c.id}`).catch(() => {})
  })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Customer groups">
      <template #actions>
        <Button icon="pi pi-plus" label="New group" @click="openCreate" />
      </template>
    </PageHeader>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <p class="hint">Select a group to view the customers assigned to it.</p>
    <DataTable
      :value="rows"
      :loading="loading"
      dataKey="id"
      stripedRows
      rowHover
      selectionMode="single"
      @row-click="openMembers($event.data)"
    >
      <template #empty>
        <EmptyState icon="pi pi-users" title="No customer groups yet" message="Group customers to assign shared price lists, catalogs, and terms in one place.">
          <Button icon="pi pi-plus" label="New group" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="id" header="ID" style="width: 5rem" />
      <Column field="name" header="Name" sortable />
      <Column header="" style="width: 8rem">
        <template #body="{ data }">
          <Button
            label="View members"
            icon="pi pi-users"
            text
            size="small"
            @click.stop="openMembers(data)"
          />
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" header="New customer group" modal :style="{ width: '400px' }">
      <div class="field"><label>Name</label><InputText v-model="name" fluid /></div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="membersOpen"
      :header="activeGroup ? `Customers in “${activeGroup.name}”` : 'Customers'"
      modal
      :style="{ width: '640px' }"
    >
      <div v-if="membersLoading" class="loading"><ProgressSpinner style="width: 2.5rem; height: 2.5rem" /></div>
      <Message v-else-if="membersError" severity="error" :closable="false">{{ membersError }}</Message>
      <DataTable v-else :value="members" dataKey="id" stripedRows rowHover @row-click="goToCustomer($event.data)">
        <template #empty>No customers are assigned to this group yet.</template>
        <Column field="name" header="Customer" sortable />
        <Column field="payment_terms_days" header="Terms" style="width: 6rem">
          <template #body="{ data }">{{ data.payment_terms_days }}d</template>
        </Column>
        <Column field="is_active" header="Status" style="width: 7rem">
          <template #body="{ data }">
            <span :class="['status', data.is_active ? 'on' : 'off']">{{ data.is_active ? 'Active' : 'Inactive' }}</span>
          </template>
        </Column>
        <Column header="" style="width: 4rem">
          <template #body="{ data }">
            <Button icon="pi pi-arrow-right" text size="small" aria-label="Open customer" @click.stop="goToCustomer(data)" />
          </template>
        </Column>
      </DataTable>
    </Dialog>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.hint { margin: 0 0 0.75rem; font-size: 0.85rem; color: var(--p-text-muted-color, #64748b); }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.loading { display: flex; justify-content: center; padding: 2rem 0; }
.status { font-size: 0.8rem; font-weight: 600; }
.status.on { color: var(--p-green-600, #16a34a); }
.status.off { color: var(--p-text-muted-color, #94a3b8); }
:deep(.p-datatable-tbody > tr) { cursor: pointer; }
</style>
