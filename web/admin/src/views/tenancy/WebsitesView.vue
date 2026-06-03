<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Website = components['schemas']['Website']

const websites = ref<Website[]>([])
const orgName = ref('')
const loading = ref(false)
const error = ref('')
const toast = useToast()

const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const formError = ref('')
const form = reactive({ name: '', domain: '', default_currency: 'USD', default_locale: 'en' })

async function load() {
  loading.value = true
  error.value = ''
  const { data: wsData, error: wsErr } = await api.GET('/admin/websites')
  const { data: orgData } = await api.GET('/admin/organization')
  loading.value = false
  if (wsErr || !wsData) {
    error.value = errMessage(wsErr, 'Failed to load websites')
    return
  }
  websites.value = wsData.items ?? []
  orgName.value = orgData?.name ?? ''
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { name: '', domain: '', default_currency: 'USD', default_locale: 'en' })
  formError.value = ''
  dialogOpen.value = true
}
function openEdit(w: Website) {
  editingId.value = w.id
  Object.assign(form, { name: w.name, domain: w.domain, default_currency: w.default_currency, default_locale: w.default_locale })
  formError.value = ''
  dialogOpen.value = true
}

async function save() {
  if (!form.name || !form.domain || form.default_currency.length !== 3) {
    formError.value = 'Name, domain, and a 3-letter currency are required.'
    return
  }
  saving.value = true
  const body = { ...form }
  const { error: err } = editingId.value
    ? await api.PUT('/admin/websites/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/admin/websites', { body })
  saving.value = false
  if (err) {
    formError.value = errMessage(err, 'Save failed (domain may already be in use)')
    return
  }
  toast.add({ severity: 'success', summary: 'Website saved', life: 2500 })
  dialogOpen.value = false
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>Websites <span class="muted">{{ orgName }}</span></h1>
      <div class="actions">
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New website" @click="openCreate" />
      </div>
    </div>
    <p class="muted">Each website serves a distinct domain, currency, and locale. The storefront resolves the active website from the request host.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="websites" :loading="loading" dataKey="id" stripedRows>
      <template #empty>No websites yet.</template>
      <Column field="name" header="Name" />
      <Column field="domain" header="Domain" />
      <Column field="default_currency" header="Currency" />
      <Column field="default_locale" header="Locale" />
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.is_active ? 'active' : 'off'" :severity="data.is_active ? 'success' : 'secondary'" /></template>
      </Column>
      <Column header="" style="width: 5rem">
        <template #body="{ data }"><Button icon="pi pi-pencil" severity="secondary" text rounded @click="openEdit(data)" /></template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" modal :header="editingId ? 'Edit website' : 'New website'" :style="{ width: '30rem' }">
      <Message v-if="formError" severity="error" :closable="false" class="mb">{{ formError }}</Message>
      <div class="field"><label>Name</label><InputText v-model="form.name" /></div>
      <div class="field"><label>Domain</label><InputText v-model="form.domain" placeholder="store.example.com" /></div>
      <div class="row">
        <div class="field"><label>Currency</label><InputText v-model="form.default_currency" maxlength="3" /></div>
        <div class="field"><label>Locale</label><InputText v-model="form.default_locale" /></div>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.header { display: flex; align-items: center; justify-content: space-between; }
.header h1 { margin: 0; }
.actions { display: flex; gap: 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.mb { margin-bottom: 1rem; }
.row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
</style>
