<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Vendor = components['schemas']['Vendor']
type VendorUser = components['schemas']['VendorUser']
type VendorPayout = components['schemas']['VendorPayout']

const vendors = ref<Vendor[]>([])
const loading = ref(false)
const error = ref('')
const toast = useToast()

const statuses = [
  { label: 'Active', value: 'active' },
  { label: 'Pending', value: 'pending' },
  { label: 'Suspended', value: 'suspended' },
]

function sev(s: string) {
  return s === 'active' ? 'success' : s === 'suspended' ? 'danger' : 'warn'
}

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/vendors')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load vendors')
    return
  }
  vendors.value = data.items ?? []
}

// ---- create vendor ----
const createOpen = ref(false)
const form = ref<{ name: string; contact_email: string; commission_rate: number; payout_terms_days: number; status: string }>({
  name: '', contact_email: '', commission_rate: 10, payout_terms_days: 30, status: 'active',
})
const saving = ref(false)

function openCreate() {
  form.value = { name: '', contact_email: '', commission_rate: 10, payout_terms_days: 30, status: 'active' }
  createOpen.value = true
}

async function create() {
  saving.value = true
  const { data, error: err } = await api.POST('/admin/vendors', {
    body: {
      name: form.value.name,
      contact_email: form.value.contact_email || null,
      commission_rate: String(form.value.commission_rate),
      payout_terms_days: form.value.payout_terms_days,
      status: form.value.status as Vendor['status'],
    },
  })
  saving.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Create failed', detail: errMessage(err), life: 4000 })
    return
  }
  createOpen.value = false
  toast.add({ severity: 'success', summary: 'Vendor created', life: 2000 })
  load()
}

// ---- detail (users + payouts + edit) ----
const detail = ref<Vendor | null>(null)
const detailOpen = ref(false)
const users = ref<VendorUser[]>([])
const payouts = ref<VendorPayout[]>([])
const busy = ref(false)

async function open(v: Vendor) {
  detail.value = { ...v }
  detailOpen.value = true
  await Promise.all([loadUsers(v.id), loadPayouts(v.id)])
}

async function loadUsers(id: number) {
  const { data } = await api.GET('/admin/vendors/{id}/users', { params: { path: { id } } })
  users.value = data?.items ?? []
}
async function loadPayouts(id: number) {
  const { data } = await api.GET('/admin/vendors/{id}/payouts', { params: { path: { id } } })
  payouts.value = data?.items ?? []
}

async function saveVendor() {
  if (!detail.value) return
  busy.value = true
  const v = detail.value
  const { error: err } = await api.PUT('/admin/vendors/{id}', {
    params: { path: { id: v.id } },
    body: {
      name: v.name, contact_email: v.contact_email ?? null, status: v.status,
      commission_rate: v.commission_rate, payout_terms_days: v.payout_terms_days,
    },
  })
  busy.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Vendor saved', life: 2000 })
  load()
}

// ---- add user ----
const userForm = ref({ email: '', password: '', full_name: '', role: 'member' })
async function addUser() {
  if (!detail.value) return
  busy.value = true
  const { error: err } = await api.POST('/admin/vendors/{id}/users', {
    params: { path: { id: detail.value.id } },
    body: { ...userForm.value, role: userForm.value.role as VendorUser['role'] },
  })
  busy.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Add user failed', detail: errMessage(err), life: 4000 })
    return
  }
  userForm.value = { email: '', password: '', full_name: '', role: 'member' }
  loadUsers(detail.value.id)
}

// ---- payouts ----
async function generatePayout() {
  if (!detail.value) return
  busy.value = true
  const { error: err } = await api.POST('/admin/vendors/{id}/payouts', { params: { path: { id: detail.value.id } } })
  busy.value = false
  if (err) {
    toast.add({ severity: 'warn', summary: 'No payout generated', detail: errMessage(err, 'Nothing to pay out'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Payout generated', life: 2000 })
  loadPayouts(detail.value.id)
}

async function pay(p: VendorPayout) {
  busy.value = true
  const { error: err } = await api.POST('/admin/payouts/{id}/pay', { params: { path: { id: p.id } }, body: {} })
  busy.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Mark paid failed', detail: errMessage(err), life: 4000 })
    return
  }
  if (detail.value) loadPayouts(detail.value.id)
}

function paySev(s: string) {
  return s === 'paid' ? 'success' : s === 'cancelled' ? 'danger' : 'warn'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Vendors" :meta="vendors.length">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button label="Add vendor" icon="pi pi-plus" @click="openCreate" />
      </template>
    </PageHeader>
    <p class="muted">Marketplace sellers. Commission is the operator's take; orders split per vendor and settle into payouts.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="vendors" :loading="loading" dataKey="id" stripedRows @rowClick="open($event.data)" class="clickable">
      <template #empty>
        <EmptyState icon="pi pi-shop" title="No vendors yet" message="Add a vendor to let third parties list and sell products on your marketplace.">
          <Button label="Add vendor" icon="pi pi-plus" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="name" header="Name" />
      <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template></Column>
      <Column header="Commission"><template #body="{ data }">{{ data.commission_rate }}%</template></Column>
      <Column header="Payout terms"><template #body="{ data }">{{ data.payout_terms_days }} days</template></Column>
      <Column field="contact_email" header="Contact" />
    </DataTable>

    <!-- Create -->
    <Dialog v-model:visible="createOpen" modal header="Add vendor" :style="{ width: '32rem' }">
      <div class="field"><label>Name</label><InputText v-model="form.name" /></div>
      <div class="field"><label>Contact email</label><InputText v-model="form.contact_email" /></div>
      <div class="row">
        <div class="field"><label>Commission %</label><InputNumber v-model="form.commission_rate" :min="0" :max="100" :maxFractionDigits="4" /></div>
        <div class="field"><label>Payout terms (days)</label><InputNumber v-model="form.payout_terms_days" :min="0" /></div>
      </div>
      <div class="field"><label>Status</label><Select v-model="form.status" :options="statuses" optionLabel="label" optionValue="value" /></div>
      <template #footer>
        <Button label="Cancel" text @click="createOpen = false" />
        <Button label="Create" :loading="saving" :disabled="!form.name" @click="create" />
      </template>
    </Dialog>

    <!-- Detail -->
    <Dialog v-model:visible="detailOpen" modal :header="detail?.name ?? 'Vendor'" :style="{ width: '46rem' }">
      <template v-if="detail">
        <section class="block">
          <h3>Profile</h3>
          <div class="row">
            <div class="field"><label>Name</label><InputText v-model="detail.name" /></div>
            <div class="field"><label>Status</label><Select v-model="detail.status" :options="statuses" optionLabel="label" optionValue="value" /></div>
          </div>
          <div class="row">
            <div class="field"><label>Commission %</label><InputText v-model="detail.commission_rate" /></div>
            <div class="field"><label>Payout terms (days)</label><InputNumber v-model="detail.payout_terms_days" :min="0" /></div>
          </div>
          <div class="field"><label>Contact email</label><InputText :modelValue="detail.contact_email ?? ''" @update:modelValue="detail.contact_email = ($event as string) || null" /></div>
          <Button label="Save profile" icon="pi pi-save" :loading="busy" @click="saveVendor" />
        </section>

        <section class="block">
          <h3>Portal users</h3>
          <DataTable :value="users" dataKey="id" stripedRows>
            <template #empty>No portal users.</template>
            <Column field="full_name" header="Name" />
            <Column field="email" header="Email" />
            <Column header="Role"><template #body="{ data }"><Tag :value="data.role" /></template></Column>
          </DataTable>
          <div class="adduser">
            <InputText v-model="userForm.full_name" placeholder="Full name" />
            <InputText v-model="userForm.email" placeholder="Email" />
            <InputText v-model="userForm.password" placeholder="Password" type="password" />
            <Select v-model="userForm.role" :options="[{label:'Member',value:'member'},{label:'Admin',value:'admin'}]" optionLabel="label" optionValue="value" />
            <Button label="Add" icon="pi pi-user-plus" :loading="busy" :disabled="!userForm.email || !userForm.password || !userForm.full_name" @click="addUser" />
          </div>
        </section>

        <section class="block">
          <div class="payhdr">
            <h3>Payouts</h3>
            <Button label="Generate payout" icon="pi pi-wallet" size="small" :loading="busy" @click="generatePayout" />
          </div>
          <DataTable :value="payouts" dataKey="id" stripedRows>
            <template #empty>No payouts yet. Generate one from delivered orders.</template>
            <Column header="Amount"><template #body="{ data }">{{ data.amount }} {{ data.currency }}</template></Column>
            <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="paySev(data.status)" /></template></Column>
            <Column field="reference" header="Reference" />
            <Column header=""><template #body="{ data }"><Button v-if="data.status === 'pending'" label="Mark paid" size="small" text :loading="busy" @click="pay(data)" /></template></Column>
          </DataTable>
        </section>
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.clickable :deep(tbody tr) { cursor: pointer; }
.block { margin-bottom: 1.5rem; }
.block h3 { margin: 0 0 0.5rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; margin-bottom: 0.75rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.adduser { display: grid; grid-template-columns: 1fr 1fr 1fr 8rem auto; gap: 0.5rem; align-items: center; margin-top: 0.5rem; }
.payhdr { display: flex; align-items: center; justify-content: space-between; }
</style>
