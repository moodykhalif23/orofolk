<script setup lang="ts">
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import ToggleSwitch from 'primevue/toggleswitch'
import Message from 'primevue/message'
import type { components } from '@teggo/api/schema'

definePageMeta({ middleware: 'auth' })
useSeoMeta({ title: 'Company users — Teggo Store' })

type User = components['schemas']['CustomerUser']

const client = useClient()
const router = useRouter()

const users = ref<User[]>([])
const forbidden = ref(false)
const error = ref('')
const busy = ref(false)

const roles = [
  { label: 'Buyer', value: 'buyer' },
  { label: 'Approver', value: 'approver' },
  { label: 'Admin', value: 'admin' },
]

async function load() {
  error.value = ''
  const { data, error: err, response } = await client.GET('/storefront/account/users')
  if (response?.status === 403) {
    forbidden.value = true
    return
  }
  if (err || !data) {
    error.value = 'Could not load users.'
    return
  }
  users.value = data.items ?? []
}

// Create dialog
const createOpen = ref(false)
const form = reactive({ email: '', password: '', full_name: '', role: 'buyer', spending_limit: '' })

function openCreate() {
  form.email = ''
  form.password = ''
  form.full_name = ''
  form.role = 'buyer'
  form.spending_limit = ''
  error.value = ''
  createOpen.value = true
}

async function createUser() {
  if (!form.email || !form.password || !form.full_name) return
  busy.value = true
  const { error: err, response } = await client.POST('/storefront/account/users', {
    body: {
      email: form.email,
      password: form.password,
      full_name: form.full_name,
      role: form.role as User['role'],
      spending_limit: form.spending_limit.trim() || null,
    },
  })
  busy.value = false
  if (err) {
    error.value = response?.status === 409 ? 'A user with this email already exists.' : 'Could not create the user.'
    return
  }
  createOpen.value = false
  await load()
}

// Edit dialog
const editOpen = ref(false)
const editTarget = ref<User | null>(null)
const edit = reactive({ full_name: '', role: 'buyer', spending_limit: '', is_active: true })

function openEdit(u: User) {
  editTarget.value = u
  edit.full_name = u.full_name
  edit.role = u.role
  edit.spending_limit = u.spending_limit ?? ''
  edit.is_active = u.is_active
  error.value = ''
  editOpen.value = true
}

async function saveEdit() {
  if (!editTarget.value) return
  busy.value = true
  const { error: err, response } = await client.PATCH('/storefront/account/users/{id}', {
    params: { path: { id: editTarget.value.id } },
    body: {
      full_name: edit.full_name,
      role: edit.role as User['role'],
      spending_limit: edit.spending_limit.trim() || null,
      is_active: edit.is_active,
    },
  })
  busy.value = false
  if (err) {
    error.value = response?.status === 400
      ? 'You cannot remove your own admin access.'
      : 'Could not update the user.'
    return
  }
  editOpen.value = false
  await load()
}

await load()
</script>

<template>
  <section class="wrap">
    <Button icon="pi pi-arrow-left" label="Account settings" text severity="secondary" @click="router.push('/account/settings')" />

    <Message v-if="forbidden" severity="warn" :closable="false">
      You need the company-admin role to manage users.
    </Message>

    <template v-else>
      <div class="head">
        <h1>Company users</h1>
        <Button label="Invite user" icon="pi pi-user-plus" @click="openCreate" />
      </div>

      <Message v-if="error" severity="error" :closable="true" class="mb">{{ error }}</Message>

      <DataTable :value="users" dataKey="id" stripedRows>
        <template #empty>No users.</template>
        <Column field="full_name" header="Name" />
        <Column field="email" header="Email" />
        <Column header="Role"><template #body="{ data }"><Tag :value="data.role" /></template></Column>
        <Column header="Spending limit"><template #body="{ data }">{{ data.spending_limit || 'Unlimited' }}</template></Column>
        <Column header="Status">
          <template #body="{ data }">
            <Tag :value="data.is_active ? 'active' : 'inactive'" :severity="data.is_active ? 'success' : 'secondary'" />
          </template>
        </Column>
        <Column>
          <template #body="{ data }">
            <Button icon="pi pi-pencil" text rounded size="small" @click="openEdit(data)" />
          </template>
        </Column>
      </DataTable>
    </template>

    <Dialog v-model:visible="createOpen" modal header="Invite user" :style="{ width: '28rem' }" :closable="!busy">
      <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
      <div class="field"><label>Full name</label><InputText v-model="form.full_name" /></div>
      <div class="field"><label>Email</label><InputText v-model="form.email" /></div>
      <div class="field"><label>Temporary password</label><Password v-model="form.password" :feedback="false" toggleMask fluid /></div>
      <div class="field"><label>Role</label><Select v-model="form.role" :options="roles" optionLabel="label" optionValue="value" /></div>
      <div class="field"><label>Spending limit <span class="muted">(optional)</span></label><InputText v-model="form.spending_limit" placeholder="Unlimited" /></div>
      <template #footer>
        <Button label="Cancel" text :disabled="busy" @click="createOpen = false" />
        <Button label="Invite" :loading="busy" @click="createUser" />
      </template>
    </Dialog>

    <Dialog v-model:visible="editOpen" modal :header="editTarget ? `Edit ${editTarget.full_name}` : 'Edit user'" :style="{ width: '28rem' }" :closable="!busy">
      <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
      <div class="field"><label>Full name</label><InputText v-model="edit.full_name" /></div>
      <div class="field"><label>Role</label><Select v-model="edit.role" :options="roles" optionLabel="label" optionValue="value" /></div>
      <div class="field"><label>Spending limit <span class="muted">(optional)</span></label><InputText v-model="edit.spending_limit" placeholder="Unlimited" /></div>
      <div class="field-row"><ToggleSwitch v-model="edit.is_active" inputId="active" /><label for="active">Active</label></div>
      <template #footer>
        <Button label="Cancel" text :disabled="busy" @click="editOpen = false" />
        <Button label="Save" :loading="busy" @click="saveEdit" />
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
.wrap { max-width: 920px; }
.head { display: flex; align-items: center; justify-content: space-between; margin: 0.5rem 0 1rem; }
.head h1 { margin: 0; }
.mb { margin-bottom: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.9rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.field-row { display: flex; align-items: center; gap: 0.6rem; margin: 0.5rem 0; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
</style>
