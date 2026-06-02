<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import Card from 'primevue/card'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Customer = components['schemas']['Customer']
type CustomerUser = components['schemas']['CustomerUser']
type CustomerAddress = components['schemas']['CustomerAddress']

const route = useRoute()
const router = useRouter()
const toast = useToast()
const id = Number(route.params.id)

const customer = ref<Customer | null>(null)
const ancestors = ref<{ id: number; depth: number }[]>([])
const users = ref<CustomerUser[]>([])
const addresses = ref<CustomerAddress[]>([])
const error = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  const [c, h, u, a] = await Promise.all([
    api.GET('/admin/customers/{id}', { params: { path: { id } } }),
    api.GET('/admin/customers/{id}/hierarchy', { params: { path: { id } } }),
    api.GET('/admin/customers/{id}/users', { params: { path: { id } } }),
    api.GET('/admin/customers/{id}/addresses', { params: { path: { id } } }),
  ])
  loading.value = false
  if (c.error || !c.data) {
    error.value = errMessage(c.error, 'Customer not found')
    return
  }
  customer.value = c.data
  ancestors.value = h.data?.ancestors ?? []
  users.value = u.data?.items ?? []
  addresses.value = a.data?.items ?? []
}

// --- add user ---
const userDialog = ref(false)
const savingUser = ref(false)
const userForm = reactive({ email: '', password: '', full_name: '', role: 'buyer' as 'buyer' | 'approver' | 'admin', spending_limit: '' })
function openUser() {
  Object.assign(userForm, { email: '', password: '', full_name: '', role: 'buyer', spending_limit: '' })
  userDialog.value = true
}
async function saveUser() {
  savingUser.value = true
  const { error: err } = await api.POST('/admin/customers/{id}/users', {
    params: { path: { id } },
    body: {
      email: userForm.email,
      password: userForm.password,
      full_name: userForm.full_name,
      role: userForm.role,
      spending_limit: userForm.spending_limit || null,
    },
  })
  savingUser.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errMessage(err), life: 4000 })
    return
  }
  userDialog.value = false
  toast.add({ severity: 'success', summary: 'User added', life: 2000 })
  load()
}

// --- add address ---
const addrDialog = ref(false)
const savingAddr = ref(false)
const addrForm = reactive({
  type: 'shipping' as 'billing' | 'shipping',
  is_default: false,
  line1: '',
  line2: '',
  city: '',
  region: '',
  postal_code: '',
  country: '',
})
function openAddr() {
  Object.assign(addrForm, { type: 'shipping', is_default: false, line1: '', line2: '', city: '', region: '', postal_code: '', country: '' })
  addrDialog.value = true
}
async function saveAddr() {
  savingAddr.value = true
  const { error: err } = await api.POST('/admin/customers/{id}/addresses', {
    params: { path: { id } },
    body: {
      type: addrForm.type,
      is_default: addrForm.is_default,
      line1: addrForm.line1,
      line2: addrForm.line2 || null,
      city: addrForm.city,
      region: addrForm.region || null,
      postal_code: addrForm.postal_code || null,
      country: addrForm.country,
    },
  })
  savingAddr.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errMessage(err), life: 4000 })
    return
  }
  addrDialog.value = false
  toast.add({ severity: 'success', summary: 'Address added', life: 2000 })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <Button icon="pi pi-arrow-left" label="Customers" text severity="secondary" @click="router.push({ name: 'customers' })" />
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <template v-if="customer">
      <h1 class="title">{{ customer.name }}</h1>

      <div class="grid">
        <Card>
          <template #title>Overview</template>
          <template #content>
            <dl class="kv">
              <dt>Tax ID</dt><dd>{{ customer.tax_id ?? '—' }}</dd>
              <dt>Payment terms</dt><dd>{{ customer.payment_terms_days }} days</dd>
              <dt>Credit limit</dt><dd>{{ customer.credit_limit }}</dd>
              <dt>Group</dt><dd>{{ customer.customer_group_id ?? '—' }}</dd>
              <dt>Active</dt>
              <dd><Tag :value="customer.is_active ? 'active' : 'inactive'" :severity="customer.is_active ? 'success' : 'secondary'" /></dd>
            </dl>
          </template>
        </Card>

        <Card>
          <template #title>Hierarchy (ancestors)</template>
          <template #content>
            <ol v-if="ancestors.length" class="ancestors">
              <li v-for="a in ancestors" :key="a.id">Customer #{{ a.id }} <span class="muted">(depth {{ a.depth }})</span></li>
            </ol>
            <p v-else class="muted">Top-level customer (no parent).</p>
          </template>
        </Card>
      </div>

      <Tabs value="users" class="tabs">
        <TabList>
          <Tab value="users">Users ({{ users.length }})</Tab>
          <Tab value="addresses">Addresses ({{ addresses.length }})</Tab>
        </TabList>
        <TabPanels>
          <TabPanel value="users">
            <div class="tabhead">
              <Button icon="pi pi-plus" label="Add user" size="small" @click="openUser" />
            </div>
            <DataTable :value="users" dataKey="id" stripedRows>
              <template #empty>No users.</template>
              <Column field="full_name" header="Name" />
              <Column field="email" header="Email" />
              <Column field="role" header="Role" />
              <Column header="Spending limit"><template #body="{ data }">{{ data.spending_limit ?? '—' }}</template></Column>
            </DataTable>
          </TabPanel>
          <TabPanel value="addresses">
            <div class="tabhead">
              <Button icon="pi pi-plus" label="Add address" size="small" @click="openAddr" />
            </div>
            <DataTable :value="addresses" dataKey="id" stripedRows>
              <template #empty>No addresses.</template>
              <Column field="type" header="Type" />
              <Column field="line1" header="Line 1" />
              <Column field="city" header="City" />
              <Column field="country" header="Country" />
              <Column header="Default"><template #body="{ data }"><Tag v-if="data.is_default" value="default" severity="info" /></template></Column>
            </DataTable>
          </TabPanel>
        </TabPanels>
      </Tabs>
    </template>

    <!-- Add user dialog -->
    <Dialog v-model:visible="userDialog" header="Add customer user" modal :style="{ width: '440px' }">
      <form class="form" @submit.prevent="saveUser">
        <div class="field"><label>Full name</label><InputText v-model="userForm.full_name" fluid /></div>
        <div class="field"><label>Email</label><InputText v-model="userForm.email" fluid /></div>
        <div class="field"><label>Password</label><Password v-model="userForm.password" :feedback="false" toggleMask fluid /></div>
        <div class="field"><label>Role</label><Select v-model="userForm.role" :options="['buyer', 'approver', 'admin']" fluid /></div>
        <div class="field"><label>Spending limit (optional)</label><InputText v-model="userForm.spending_limit" fluid /></div>
      </form>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="userDialog = false" />
        <Button label="Save" :loading="savingUser" @click="saveUser" />
      </template>
    </Dialog>

    <!-- Add address dialog -->
    <Dialog v-model:visible="addrDialog" header="Add address" modal :style="{ width: '480px' }">
      <form class="form" @submit.prevent="saveAddr">
        <div class="grid2">
          <div class="field"><label>Type</label><Select v-model="addrForm.type" :options="['billing', 'shipping']" fluid /></div>
          <div class="check"><Checkbox v-model="addrForm.is_default" binary inputId="def" /><label for="def">Default</label></div>
        </div>
        <div class="field"><label>Line 1</label><InputText v-model="addrForm.line1" fluid /></div>
        <div class="field"><label>Line 2</label><InputText v-model="addrForm.line2" fluid /></div>
        <div class="grid2">
          <div class="field"><label>City</label><InputText v-model="addrForm.city" fluid /></div>
          <div class="field"><label>Region</label><InputText v-model="addrForm.region" fluid /></div>
        </div>
        <div class="grid2">
          <div class="field"><label>Postal code</label><InputText v-model="addrForm.postal_code" fluid /></div>
          <div class="field"><label>Country (2-letter)</label><InputText v-model="addrForm.country" maxlength="2" fluid /></div>
        </div>
      </form>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="addrDialog = false" />
        <Button label="Save" :loading="savingAddr" @click="saveAddr" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.title { margin: 0.5rem 0 1rem; }
.mb { margin-bottom: 1rem; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; margin-bottom: 1.5rem; }
.kv { display: grid; grid-template-columns: auto 1fr; gap: 0.4rem 1rem; margin: 0; }
.kv dt { font-weight: 600; color: var(--p-text-muted-color, #64748b); }
.kv dd { margin: 0; }
.ancestors { margin: 0; padding-left: 1.1rem; line-height: 1.8; }
.muted { color: var(--p-text-muted-color, #64748b); }
.tabhead { display: flex; justify-content: flex-end; margin-bottom: 0.75rem; }
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 0.9rem; align-items: end; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.check { display: flex; align-items: center; gap: 0.5rem; padding-bottom: 0.5rem; }
</style>
