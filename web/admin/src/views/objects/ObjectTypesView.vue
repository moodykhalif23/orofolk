<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import { useAuthStore } from '@/stores/auth'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type ObjectType = components['schemas']['ObjectType']

const router = useRouter()
const toast = useToast()
const auth = useAuthStore()
const rows = ref<ObjectType[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const form = reactive({ code: '', label: '', label_plural: '', description: '', is_active: true })

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: e } = await api.GET('/admin/object-types')
  loading.value = false
  if (e) {
    error.value = errMessage(e, 'Failed to load custom objects')
    return
  }
  rows.value = data?.items ?? []
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { code: '', label: '', label_plural: '', description: '', is_active: true })
  dialogOpen.value = true
}

function openEdit(t: ObjectType) {
  editingId.value = t.id ?? null
  Object.assign(form, {
    code: t.code ?? '', label: t.label ?? '', label_plural: t.label_plural ?? '',
    description: t.description ?? '', is_active: t.is_active ?? true,
  })
  dialogOpen.value = true
}

async function save() {
  if (!form.code.trim() || !form.label.trim()) {
    toast.add({ severity: 'warn', summary: 'Code and label are required', life: 2500 })
    return
  }
  saving.value = true
  const body: components['schemas']['ObjectTypeInput'] = {
    code: form.code.trim(), label: form.label.trim(), label_plural: form.label_plural.trim(),
    description: form.description.trim(), is_active: form.is_active,
  }
  const { error: e } = editingId.value
    ? await api.PUT('/admin/object-types/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/admin/object-types', { body })
  saving.value = false
  if (e) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(e), life: 4000 })
    return
  }
  dialogOpen.value = false
  toast.add({ severity: 'success', summary: editingId.value ? 'Type updated' : 'Type created', life: 2000 })
  load()
}

async function remove(t: ObjectType) {
  if (!confirm(`Delete "${t.label}"? This removes its field schema.`)) return
  const { error: e } = await api.DELETE('/admin/object-types/{id}', { params: { path: { id: t.id! } } })
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Delete failed — does it still have records?'), life: 4500 })
    return
  }
  toast.add({ severity: 'success', summary: 'Type deleted', life: 2000 })
  load()
}

function manage(t: ObjectType) {
  router.push({ name: 'object-type-detail', params: { id: t.id } })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Custom objects">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button v-if="auth.can('object.manage')" icon="pi pi-plus" label="New type" @click="openCreate" />
      </template>
    </PageHeader>
    <p class="muted">
      Model any entity your business tracks — suppliers, locations, contracts, assets — with its own
      fields and validation rules, then capture records against it. The same engine that guards
      product data guards yours.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="rows" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-database" title="No custom objects yet" message="Define a type (e.g. Supplier) with its fields, then start capturing records.">
          <Button v-if="auth.can('object.manage')" icon="pi pi-plus" label="New type" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="label" header="Name" sortable />
      <Column field="code" header="Code"><template #body="{ data }"><code>{{ data.code }}</code></template></Column>
      <Column field="description" header="Description"><template #body="{ data }"><span class="muted">{{ data.description || '—' }}</span></template></Column>
      <Column header="Active"><template #body="{ data }"><Tag :value="data.is_active ? 'active' : 'off'" :severity="data.is_active ? 'success' : 'secondary'" /></template></Column>
      <Column header="" style="width: 13rem">
        <template #body="{ data }">
          <Button icon="pi pi-table" label="Manage" text size="small" @click="manage(data)" />
          <template v-if="auth.can('object.manage')">
            <Button icon="pi pi-pencil" text rounded size="small" title="Edit type" @click="openEdit(data)" />
            <Button icon="pi pi-trash" text rounded size="small" severity="danger" title="Delete" @click="remove(data)" />
          </template>
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" :header="editingId ? 'Edit type' : 'New custom object'" modal :style="{ width: '34rem' }">
      <div class="form">
        <div class="field">
          <label>Code</label>
          <InputText v-model="form.code" :disabled="!!editingId" placeholder="supplier" fluid />
          <small v-if="editingId" class="muted">Code is the storage key and can't change.</small>
        </div>
        <div class="row">
          <div class="field"><label>Name</label><InputText v-model="form.label" placeholder="Supplier" fluid /></div>
          <div class="field"><label>Plural</label><InputText v-model="form.label_plural" placeholder="Suppliers" fluid /></div>
        </div>
        <div class="field"><label>Description <span class="muted">(optional)</span></label><Textarea v-model="form.description" rows="2" fluid /></div>
        <div class="check"><ToggleSwitch v-model="form.is_active" inputId="ot-active" /><label for="ot-active">Active</label></div>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button :label="editingId ? 'Save' : 'Create'" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.check { display: flex; align-items: center; gap: 0.5rem; }
.check label { font-size: 0.9rem; font-weight: 600; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
