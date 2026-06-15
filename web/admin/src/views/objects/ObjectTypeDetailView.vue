<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import ToggleSwitch from 'primevue/toggleswitch'
import Checkbox from 'primevue/checkbox'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import { useAuthStore } from '@/stores/auth'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type ObjectType = components['schemas']['ObjectType']
type ObjectField = components['schemas']['ObjectField']
type ObjectRecord = components['schemas']['ObjectRecord']
type Validation = components['schemas']['AttributeValidation']
type DataType = NonNullable<ObjectField['data_type']>

const dataTypes: DataType[] = ['text', 'number', 'boolean', 'select', 'multiselect', 'date', 'file', 'price']

const route = useRoute()
const router = useRouter()
const toast = useToast()
const auth = useAuthStore()
const canManage = computed(() => auth.can('object.manage'))

const typeId = Number(route.params.id)
const typeRow = ref<ObjectType | null>(null)
const fields = ref<ObjectField[]>([])
const records = ref<ObjectRecord[]>([])
const recordsTotal = ref(0)
const loading = ref(false)
const error = ref('')

const code = computed(() => typeRow.value?.code ?? '')
const displayFields = computed(() => fields.value.slice(0, 5))

async function loadType() {
  loading.value = true
  error.value = ''
  const { data, error: e } = await api.GET('/admin/object-types/{id}', { params: { path: { id: typeId } } })
  loading.value = false
  if (e || !data) {
    error.value = errMessage(e, 'Failed to load type')
    return
  }
  typeRow.value = data
  fields.value = data.fields ?? []
  loadRecords()
}

async function loadRecords() {
  if (!code.value) return
  const { data } = await api.GET('/admin/objects/{code}', {
    params: { path: { code: code.value }, query: { page_size: 100 } },
  })
  records.value = data?.items ?? []
  recordsTotal.value = data?.total ?? 0
}

// ---- Field schema builder ------------------------------------------------

const fieldDialog = ref(false)
const fieldSaving = ref(false)
const editingFieldId = ref<number | null>(null)
const ff = reactive({
  code: '', label: '', data_type: 'text' as DataType, options: '',
  pattern: '', min_length: null as number | null, max_length: null as number | null,
  min: null as number | null, max: null as number | null,
  min_select: null as number | null, max_select: null as number | null,
  is_required: false, sort_order: 0,
})
const ffText = computed(() => ['text', 'file', 'date'].includes(ff.data_type as string))
const ffNumeric = computed(() => ff.data_type === 'number' || ff.data_type === 'price')
const ffOptions = computed(() => ff.data_type === 'select' || ff.data_type === 'multiselect')
const ffMulti = computed(() => ff.data_type === 'multiselect')

function resetFieldForm() {
  Object.assign(ff, {
    code: '', label: '', data_type: 'text', options: '', pattern: '',
    min_length: null, max_length: null, min: null, max: null, min_select: null, max_select: null,
    is_required: false, sort_order: fields.value.length,
  })
}
function openFieldCreate() {
  editingFieldId.value = null
  resetFieldForm()
  fieldDialog.value = true
}
function openFieldEdit(f: ObjectField) {
  editingFieldId.value = f.id ?? null
  resetFieldForm()
  const v = (f.validation ?? {}) as Validation
  Object.assign(ff, {
    code: f.code ?? '', label: f.label ?? '', data_type: f.data_type ?? 'text',
    options: (f.options ?? []).join(', '),
    pattern: v.pattern ?? '', min_length: v.min_length ?? null, max_length: v.max_length ?? null,
    min: v.min ?? null, max: v.max ?? null, min_select: v.min_select ?? null, max_select: v.max_select ?? null,
    is_required: f.is_required ?? false, sort_order: f.sort_order ?? 0,
  })
  fieldDialog.value = true
}
function buildValidation(): Validation {
  const v: Validation = {}
  if (ffText.value) {
    if (ff.pattern.trim()) v.pattern = ff.pattern.trim()
    if (ff.min_length != null) v.min_length = ff.min_length
    if (ff.max_length != null) v.max_length = ff.max_length
  } else if (ffNumeric.value) {
    if (ff.min != null) v.min = ff.min
    if (ff.max != null) v.max = ff.max
  } else if (ffMulti.value) {
    if (ff.min_select != null) v.min_select = ff.min_select
    if (ff.max_select != null) v.max_select = ff.max_select
  }
  return v
}
async function saveField() {
  if (!ff.code.trim() || !ff.label.trim()) {
    toast.add({ severity: 'warn', summary: 'Code and label are required', life: 2500 })
    return
  }
  fieldSaving.value = true
  const body: components['schemas']['ObjectFieldInput'] = {
    code: ff.code.trim(), label: ff.label.trim(), data_type: ff.data_type,
    is_required: ff.is_required, sort_order: ff.sort_order, validation: buildValidation(),
  }
  if (ffOptions.value) body.options = ff.options.split(',').map((s) => s.trim()).filter(Boolean)
  const { error: e } = editingFieldId.value
    ? await api.PUT('/admin/object-fields/{id}', { params: { path: { id: editingFieldId.value } }, body })
    : await api.POST('/admin/object-types/{id}/fields', { params: { path: { id: typeId } }, body })
  fieldSaving.value = false
  if (e) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(e), life: 4000 })
    return
  }
  fieldDialog.value = false
  toast.add({ severity: 'success', summary: 'Field saved', life: 1800 })
  loadType()
}
async function deleteField(f: ObjectField) {
  if (!confirm(`Delete field "${f.label}"? Existing record values for it remain but stop being validated.`)) return
  const { error: e } = await api.DELETE('/admin/object-fields/{id}', { params: { path: { id: f.id! } } })
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Delete failed'), life: 4000 })
    return
  }
  loadType()
}

// ---- Records (schema-driven form) ----------------------------------------

const recordDialog = ref(false)
const recordSaving = ref(false)
const editingRecordId = ref<number | null>(null)
const recordError = ref('')
// Values are heterogeneous (string/number/boolean/array) and bound straight to
// typed PrimeVue inputs, so `any` is the pragmatic element type here.
const recordForm = reactive<Record<string, any>>({})

function resetRecordForm(rec?: ObjectRecord) {
  for (const k of Object.keys(recordForm)) delete recordForm[k]
  const data = (rec?.data ?? {}) as Record<string, unknown>
  for (const f of fields.value) {
    const c = f.code!
    const existing = data[c]
    if (existing !== undefined) {
      recordForm[c] = existing
    } else {
      recordForm[c] = f.data_type === 'multiselect' ? [] : f.data_type === 'boolean' ? false : null
    }
  }
}
function openRecordCreate() {
  editingRecordId.value = null
  recordError.value = ''
  resetRecordForm()
  recordDialog.value = true
}
function openRecordEdit(rec: ObjectRecord) {
  editingRecordId.value = rec.id ?? null
  recordError.value = ''
  resetRecordForm(rec)
  recordDialog.value = true
}
async function saveRecord() {
  recordError.value = ''
  const data: Record<string, unknown> = {}
  for (const f of fields.value) {
    const v = recordForm[f.code!]
    if (v === null || v === '' || (Array.isArray(v) && v.length === 0)) continue
    data[f.code!] = v
  }
  recordSaving.value = true
  const body: components['schemas']['ObjectRecordInput'] = { data }
  const { error: e } = editingRecordId.value
    ? await api.PUT('/admin/objects/{code}/{id}', { params: { path: { code: code.value, id: editingRecordId.value } }, body })
    : await api.POST('/admin/objects/{code}', { params: { path: { code: code.value } }, body })
  recordSaving.value = false
  if (e) {
    recordError.value = violationText(e)
    return
  }
  recordDialog.value = false
  toast.add({ severity: 'success', summary: 'Record saved', life: 1800 })
  loadRecords()
}
async function deleteRecord(rec: ObjectRecord) {
  if (!confirm('Delete this record?')) return
  const { error: e } = await api.DELETE('/admin/objects/{code}/{id}', { params: { path: { code: code.value, id: rec.id! } } })
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Delete failed'), life: 4000 })
    return
  }
  loadRecords()
}

// violationText pulls field-level messages out of a 422 envelope.
function violationText(e: unknown): string {
  const vs = (e as { violations?: { message?: string }[] } | null)?.violations
  if (Array.isArray(vs) && vs.length) return vs.map((v) => v.message).filter(Boolean).join('; ')
  return errMessage(e, 'Save failed')
}

function renderCell(v: unknown): string {
  if (v === null || v === undefined || v === '') return '—'
  if (Array.isArray(v)) return v.join(', ')
  if (typeof v === 'boolean') return v ? 'Yes' : 'No'
  return String(v)
}

function goImport() {
  router.push({ name: 'imports', query: { target: 'object:' + code.value } })
}

onMounted(loadType)
</script>

<template>
  <div class="page">
    <PageHeader :title="typeRow?.label ?? 'Custom object'">
      <template #actions>
        <Button icon="pi pi-arrow-left" label="All types" text severity="secondary" @click="router.push({ name: 'object-types' })" />
      </template>
    </PageHeader>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <p v-if="typeRow?.description" class="muted">{{ typeRow.description }}</p>

    <!-- Field schema -->
    <div class="section-head">
      <h3>Fields</h3>
      <Button v-if="canManage" icon="pi pi-plus" label="Add field" size="small" @click="openFieldCreate" />
    </div>
    <DataTable :value="fields" :loading="loading" dataKey="id" stripedRows class="mb">
      <template #empty>
        <EmptyState icon="pi pi-list" title="No fields yet" message="Add fields to define this object's shape and the rules its records must satisfy.">
          <Button v-if="canManage" icon="pi pi-plus" label="Add field" @click="openFieldCreate" />
        </EmptyState>
      </template>
      <Column field="label" header="Label" />
      <Column field="code" header="Code"><template #body="{ data }"><code>{{ data.code }}</code></template></Column>
      <Column field="data_type" header="Type" />
      <Column header="Required"><template #body="{ data }"><i v-if="data.is_required" class="pi pi-check req" /><span v-else class="muted">—</span></template></Column>
      <Column header="Options"><template #body="{ data }"><span class="muted small">{{ (data.options ?? []).join(', ') || '—' }}</span></template></Column>
      <Column v-if="canManage" header="" style="width: 5rem">
        <template #body="{ data }">
          <Button icon="pi pi-pencil" text rounded size="small" @click="openFieldEdit(data)" />
          <Button icon="pi pi-trash" text rounded size="small" severity="danger" @click="deleteField(data)" />
        </template>
      </Column>
    </DataTable>

    <!-- Records -->
    <div class="section-head">
      <h3>Records <span class="muted">({{ recordsTotal }})</span></h3>
      <div class="section-actions">
        <Button v-if="canManage && fields.length" icon="pi pi-upload" label="Import" size="small" severity="secondary" outlined @click="goImport" />
        <Button v-if="canManage && fields.length" icon="pi pi-plus" label="New record" size="small" @click="openRecordCreate" />
      </div>
    </div>
    <DataTable :value="records" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-inbox" :title="fields.length ? 'No records yet' : 'Add fields first'" :message="fields.length ? 'Capture your first record against this schema.' : 'A record needs at least one field to fill in.'">
          <Button v-if="canManage && fields.length" icon="pi pi-plus" label="New record" @click="openRecordCreate" />
        </EmptyState>
      </template>
      <Column v-for="f in displayFields" :key="f.id ?? f.code" :header="f.label">
        <template #body="{ data }">{{ renderCell(data.data?.[f.code!]) }}</template>
      </Column>
      <Column v-if="canManage" header="" style="width: 5rem">
        <template #body="{ data }">
          <Button icon="pi pi-pencil" text rounded size="small" @click="openRecordEdit(data)" />
          <Button icon="pi pi-trash" text rounded size="small" severity="danger" @click="deleteRecord(data)" />
        </template>
      </Column>
    </DataTable>

    <!-- Field editor -->
    <Dialog v-model:visible="fieldDialog" :header="editingFieldId ? 'Edit field' : 'Add field'" modal :style="{ width: '480px' }">
      <div class="form">
        <div class="field"><label>Code</label><InputText v-model="ff.code" :disabled="!!editingFieldId" fluid /></div>
        <div class="field"><label>Label</label><InputText v-model="ff.label" fluid /></div>
        <div class="field"><label>Data type</label><Select v-model="ff.data_type" :options="dataTypes" fluid /></div>
        <div v-if="ffOptions" class="field">
          <label>Allowed options <span class="muted">(comma-separated)</span></label>
          <InputText v-model="ff.options" placeholder="red, blue, green" fluid />
        </div>
        <template v-if="ffText">
          <div class="field"><label>Pattern <span class="muted">(regex, optional)</span></label><InputText v-model="ff.pattern" fluid /></div>
          <div class="row">
            <div class="field"><label>Min length</label><InputNumber v-model="ff.min_length" :useGrouping="false" :min="0" fluid /></div>
            <div class="field"><label>Max length</label><InputNumber v-model="ff.max_length" :useGrouping="false" :min="0" fluid /></div>
          </div>
        </template>
        <div v-else-if="ffNumeric" class="row">
          <div class="field"><label>Min</label><InputNumber v-model="ff.min" :useGrouping="false" fluid /></div>
          <div class="field"><label>Max</label><InputNumber v-model="ff.max" :useGrouping="false" fluid /></div>
        </div>
        <div v-else-if="ffMulti" class="row">
          <div class="field"><label>Min selections</label><InputNumber v-model="ff.min_select" :useGrouping="false" :min="0" fluid /></div>
          <div class="field"><label>Max selections</label><InputNumber v-model="ff.max_select" :useGrouping="false" :min="0" fluid /></div>
        </div>
        <div class="check"><Checkbox v-model="ff.is_required" binary inputId="ff-req" /><label for="ff-req">Required</label></div>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="fieldDialog = false" />
        <Button :label="editingFieldId ? 'Save' : 'Add'" :loading="fieldSaving" @click="saveField" />
      </template>
    </Dialog>

    <!-- Record editor (schema-driven) -->
    <Dialog v-model:visible="recordDialog" :header="editingRecordId ? 'Edit record' : `New ${typeRow?.label ?? 'record'}`" modal :style="{ width: '40rem' }">
      <Message v-if="recordError" severity="error" :closable="false" class="mb">{{ recordError }}</Message>
      <div class="form">
        <div v-for="f in fields" :key="f.id ?? f.code" class="field">
          <label>{{ f.label }} <span v-if="f.is_required" class="req">*</span></label>
          <InputNumber v-if="f.data_type === 'number' || f.data_type === 'price'" v-model="recordForm[f.code!]" :useGrouping="false" fluid />
          <ToggleSwitch v-else-if="f.data_type === 'boolean'" v-model="recordForm[f.code!]" />
          <Select v-else-if="f.data_type === 'select'" v-model="recordForm[f.code!]" :options="f.options ?? []" showClear fluid />
          <MultiSelect v-else-if="f.data_type === 'multiselect'" v-model="recordForm[f.code!]" :options="f.options ?? []" display="chip" fluid />
          <InputText v-else v-model="recordForm[f.code!]" fluid />
        </div>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="recordDialog = false" />
        <Button :label="editingRecordId ? 'Save' : 'Create'" :loading="recordSaving" @click="saveRecord" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.small { font-size: 0.82rem; }
.mb { margin-bottom: 1rem; }
.section-head { display: flex; align-items: center; justify-content: space-between; margin: 1.5rem 0 0.5rem; }
.section-head h3 { margin: 0; }
.section-actions { display: flex; gap: 0.5rem; }
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.check { display: flex; align-items: center; gap: 0.5rem; }
.req { color: #dc2626; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
