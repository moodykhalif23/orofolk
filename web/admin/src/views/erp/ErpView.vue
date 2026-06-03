<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Connection = components['schemas']['IntegrationConnection']
type SyncLog = components['schemas']['SyncLog']

const toast = useToast()
const connections = ref<Connection[]>([])
const logs = ref<SyncLog[]>([])
const error = ref('')

const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const form = reactive({ provider: 'generic_webhook', kind: 'erp', endpoint: '', secret: '', is_active: true })

async function load() {
  error.value = ''
  const { data: c, error: ce } = await api.GET('/admin/erp/connections')
  const { data: l } = await api.GET('/admin/erp/sync-logs')
  if (ce) {
    error.value = errMessage(ce, 'Failed to load')
    return
  }
  connections.value = c?.items ?? []
  logs.value = l?.items ?? []
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { provider: 'generic_webhook', kind: 'erp', endpoint: '', secret: '', is_active: true })
  dialogOpen.value = true
}
function openEdit(c: Connection) {
  editingId.value = c.id
  Object.assign(form, { provider: c.provider, kind: c.kind, endpoint: c.endpoint ?? '', secret: '', is_active: c.is_active })
  dialogOpen.value = true
}

async function save() {
  if (!form.provider) return
  saving.value = true
  const body: components['schemas']['IntegrationConnectionInput'] = {
    provider: form.provider,
    kind: form.kind as 'erp' | 'accounting',
    endpoint: form.endpoint || null,
    is_active: form.is_active,
  }
  if (form.secret) body.secret = form.secret
  const { error: err } = editingId.value
    ? await api.PUT('/admin/erp/connections/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/admin/erp/connections', { body })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Save failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Connection saved', life: 2000 })
  dialogOpen.value = false
  load()
}

async function runSync(c: Connection) {
  const { data, error: err } = await api.POST('/admin/erp/connections/{id}/sync', { params: { path: { id: c.id } } })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Sync failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: `Synced ${data?.synced ?? 0} (${data?.errors ?? 0} errors)`, life: 3000 })
  load()
}

const sev = (s: string) => (s === 'sent' || s === 'processed' ? 'success' : s === 'error' ? 'danger' : 'secondary')

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>ERP / accounting sync</h1>
      <div class="actions">
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New connection" @click="openCreate" />
      </div>
    </div>
    <p class="muted">Outbound: confirmed orders + issued invoices are pushed (HMAC-signed) to the connection endpoint, idempotently. Inbound: the ERP posts master data (e.g. inventory) to <code>/webhooks/erp/{id}</code>.</p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <h3>Connections</h3>
    <DataTable :value="connections" dataKey="id" stripedRows class="mb">
      <template #empty>No connections yet.</template>
      <Column field="provider" header="Provider" />
      <Column field="kind" header="Kind" />
      <Column field="endpoint" header="Endpoint" />
      <Column header="Secret"><template #body="{ data }"><i :class="data.has_secret ? 'pi pi-check' : 'pi pi-minus'" /></template></Column>
      <Column header="Active"><template #body="{ data }"><Tag :value="data.is_active ? 'active' : 'off'" :severity="data.is_active ? 'success' : 'secondary'" /></template></Column>
      <Column header="" style="width:9rem">
        <template #body="{ data }">
          <Button icon="pi pi-sync" text rounded size="small" title="Run sweep" @click="runSync(data)" />
          <Button icon="pi pi-pencil" text rounded size="small" @click="openEdit(data)" />
        </template>
      </Column>
    </DataTable>

    <h3>Sync log</h3>
    <DataTable :value="logs" dataKey="id" stripedRows scrollable scrollHeight="360px">
      <template #empty>No sync activity yet.</template>
      <Column field="id" header="#" style="width:4rem" />
      <Column field="direction" header="Dir" />
      <Column field="entity_type" header="Entity" />
      <Column field="entity_id" header="Id" />
      <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template></Column>
      <Column field="external_id" header="External" />
      <Column field="error" header="Error" />
      <Column field="created_at" header="When" />
    </DataTable>

    <Dialog v-model:visible="dialogOpen" modal :header="editingId ? 'Edit connection' : 'New connection'" :style="{ width: '32rem' }">
      <div class="field"><label>Provider</label><InputText v-model="form.provider" placeholder="generic_webhook" /></div>
      <div class="field"><label>Kind</label><Select v-model="form.kind" :options="['erp', 'accounting']" /></div>
      <div class="field"><label>Endpoint (outbound)</label><InputText v-model="form.endpoint" placeholder="https://erp.example/webhook" /></div>
      <div class="field"><label>Secret <span class="muted">(HMAC; leave blank to keep)</span></label><InputText v-model="form.secret" type="password" /></div>
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
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
h3 { margin: 1.25rem 0 0.5rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
</style>
