<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Lead = components['schemas']['Lead']
type LeadInput = components['schemas']['LeadInput']

const leads = ref<Lead[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const toast = useToast()

const sources: LeadInput['source'][] = ['manual', 'storefront_form', 'rfq', 'import', 'referral']
const form = reactive<LeadInput>({ source: 'manual', company_name: '', contact_name: '', email: '', phone: '' })

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/leads')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load leads')
    return
  }
  leads.value = data.items ?? []
}

function openCreate() {
  Object.assign(form, { source: 'manual', company_name: '', contact_name: '', email: '', phone: '' })
  dialogOpen.value = true
}

async function save() {
  saving.value = true
  const { error: err } = await api.POST('/admin/leads', { body: { ...form } })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  dialogOpen.value = false
  load()
}

async function convert(lead: Lead) {
  const { data, error: err } = await api.POST('/admin/leads/{id}/convert', { params: { path: { id: lead.id } } })
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Convert failed', detail: errMessage(err), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Converted', detail: `Customer #${data.customer_id} + opportunity created`, life: 3000 })
  load()
}

function sev(s: string) {
  return s === 'converted' ? 'success' : s === 'disqualified' ? 'danger' : s === 'qualified' ? 'info' : 'warn'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>Leads <span class="muted">({{ leads.length }})</span></h1>
      <div class="actions">
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New lead" @click="openCreate" />
      </div>
    </div>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="leads" :loading="loading" paginator :rows="10" dataKey="id" stripedRows>
      <template #empty>No leads yet.</template>
      <Column field="company_name" header="Company" />
      <Column field="contact_name" header="Contact" />
      <Column field="email" header="Email" />
      <Column field="source" header="Source" />
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template>
      </Column>
      <Column header="" style="width: 9rem">
        <template #body="{ data }">
          <Button
            label="Convert"
            icon="pi pi-arrow-right"
            size="small"
            :disabled="data.status === 'converted'"
            @click="convert(data)"
          />
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" modal header="New lead" :style="{ width: '30rem' }">
      <div class="field"><label>Source</label><Select v-model="form.source" :options="sources" /></div>
      <div class="field"><label>Company name</label><InputText v-model="form.company_name as string" /></div>
      <div class="field"><label>Contact name</label><InputText v-model="form.contact_name as string" /></div>
      <div class="field"><label>Email</label><InputText v-model="form.email as string" /></div>
      <div class="field"><label>Phone</label><InputText v-model="form.phone as string" /></div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Create" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
.header h1 { margin: 0; }
.actions { display: flex; gap: 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 1rem; }
.mb { margin-bottom: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.9rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
</style>
