<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Setting = components['schemas']['ConfigSetting']

const settings = ref<Setting[]>([])
const loading = ref(false)
const error = ref('')
const toast = useToast()
const scopes = ['org', 'website', 'group', 'customer']

const dialogOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const form = reactive({ scope: 'org', scope_id: null as number | null, key: '', value: '' })

// Resolve preview
const probe = reactive({ key: '', website_id: null as number | null, group_id: null as number | null, customer_id: null as number | null })
const probeResult = ref('')

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/settings')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load settings')
    return
  }
  settings.value = data.items ?? []
}

function fmt(v: unknown) {
  return JSON.stringify(v)
}

function openCreate() {
  Object.assign(form, { scope: 'org', scope_id: null, key: '', value: '' })
  formError.value = ''
  dialogOpen.value = true
}

async function save() {
  if (!form.key) {
    formError.value = 'Key is required.'
    return
  }
  if ((form.scope === 'org') !== (form.scope_id === null)) {
    formError.value = form.scope === 'org' ? 'Org scope must not have a scope id.' : 'This scope needs a scope id (website/group/customer).'
    return
  }
  let parsed: unknown
  try {
    parsed = JSON.parse(form.value)
  } catch {
    formError.value = 'Value must be valid JSON (e.g. true, 100, "x", {"a":1}).'
    return
  }
  saving.value = true
  const { error: err } = await api.PUT('/admin/settings', {
    body: { scope: form.scope as Setting['scope'], scope_id: form.scope_id, key: form.key, value: parsed },
  })
  saving.value = false
  if (err) {
    formError.value = errMessage(err, 'Save failed')
    return
  }
  toast.add({ severity: 'success', summary: 'Setting saved', life: 2500 })
  dialogOpen.value = false
  load()
}

async function remove(s: Setting) {
  const { error: err } = await api.DELETE('/admin/settings/{id}', { params: { path: { id: s.id } } })
  if (!err) load()
}

async function runProbe() {
  probeResult.value = ''
  if (!probe.key) return
  const { data, error: err } = await api.GET('/admin/settings/resolve', {
    params: {
      query: {
        key: probe.key,
        website_id: probe.website_id ?? undefined,
        group_id: probe.group_id ?? undefined,
        customer_id: probe.customer_id ?? undefined,
      },
    },
  })
  if (err || !data) {
    probeResult.value = 'resolve failed'
    return
  }
  probeResult.value = data.found ? `${JSON.stringify(data.value)}  (from ${data.scope} scope)` : 'no value set'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Configuration" :meta="settings.length">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New setting" @click="openCreate" />
      </template>
    </PageHeader>
    <p class="muted">Settings cascade: a key can be set at org, website, customer-group or customer scope. The most specific value wins (customer &gt; group &gt; website &gt; org).</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="settings" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-cog" title="No settings yet" message="Add a configuration key to override a platform default for this organization.">
          <Button icon="pi pi-plus" label="New setting" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="key" header="Key" />
      <Column header="Scope"><template #body="{ data }"><Tag :value="data.scope" /></template></Column>
      <Column field="scope_id" header="Scope id"><template #body="{ data }">{{ data.scope_id ?? '—' }}</template></Column>
      <Column header="Value"><template #body="{ data }"><code>{{ fmt(data.value) }}</code></template></Column>
      <Column header="" style="width: 5rem">
        <template #body="{ data }"><Button icon="pi pi-trash" text rounded severity="danger" @click="remove(data)" /></template>
      </Column>
    </DataTable>

    <div class="probe">
      <h3>Resolve preview</h3>
      <div class="probe-row">
        <InputText v-model="probe.key" placeholder="key" />
        <InputNumber v-model="probe.website_id" placeholder="website id" :useGrouping="false" />
        <InputNumber v-model="probe.group_id" placeholder="group id" :useGrouping="false" />
        <InputNumber v-model="probe.customer_id" placeholder="customer id" :useGrouping="false" />
        <Button label="Resolve" icon="pi pi-search" @click="runProbe" />
      </div>
      <p v-if="probeResult" class="probe-result"><code>{{ probeResult }}</code></p>
    </div>

    <Dialog v-model:visible="dialogOpen" modal header="New setting" :style="{ width: '32rem' }">
      <Message v-if="formError" severity="error" :closable="false" class="mb">{{ formError }}</Message>
      <div class="grid2">
        <div class="field"><label>Scope</label><Select v-model="form.scope" :options="scopes" /></div>
        <div class="field"><label>Scope id <span class="muted">(not for org)</span></label><InputNumber v-model="form.scope_id" :useGrouping="false" :disabled="form.scope === 'org'" /></div>
      </div>
      <div class="field"><label>Key</label><InputText v-model="form.key" placeholder="e.g. free_ship_threshold" /></div>
      <div class="field"><label>Value (JSON)</label><Textarea v-model="form.value" rows="3" class="mono" placeholder='e.g. 250 or true or "blue"' /></div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.mb { margin-bottom: 1rem; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.mono :deep(textarea) { font-family: ui-monospace, monospace; font-size: 0.85rem; }
.probe { margin-top: 1.5rem; border-top: 1px solid var(--p-surface-200, #e2e8f0); padding-top: 1rem; }
.probe h3 { margin: 0 0 0.5rem; font-size: 0.95rem; }
.probe-row { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
.probe-result { margin-top: 0.6rem; }
</style>
