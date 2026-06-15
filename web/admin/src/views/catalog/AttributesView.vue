<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Attribute = components['schemas']['Attribute']
type AttributeInput = components['schemas']['AttributeInput']
type Validation = components['schemas']['AttributeValidation']

const rows = ref<Attribute[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const toast = useToast()

const dataTypes: AttributeInput['data_type'][] = [
  'text', 'number', 'boolean', 'select', 'multiselect', 'date', 'file', 'price',
]

const form = reactive({
  code: '',
  label: '',
  data_type: 'text' as AttributeInput['data_type'],
  is_filterable: false,
  is_variant_axis: false,
  options: '', // comma-separated, for select/multiselect
  pattern: '',
  min_length: null as number | null,
  max_length: null as number | null,
  min: null as number | null,
  max: null as number | null,
  min_select: null as number | null,
  max_select: null as number | null,
})

const isText = computed(() => ['text', 'file', 'date'].includes(form.data_type as string))
const isNumeric = computed(() => form.data_type === 'number' || form.data_type === 'price')
const hasOptions = computed(() => form.data_type === 'select' || form.data_type === 'multiselect')
const isMulti = computed(() => form.data_type === 'multiselect')

function resetForm() {
  Object.assign(form, {
    code: '', label: '', data_type: 'text', is_filterable: false, is_variant_axis: false,
    options: '', pattern: '', min_length: null, max_length: null, min: null, max: null,
    min_select: null, max_select: null,
  })
}

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/attributes')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load attributes')
    return
  }
  rows.value = data.items ?? []
}

function openCreate() {
  editingId.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(a: Attribute) {
  editingId.value = a.id ?? null
  resetForm()
  const v = (a.validation ?? {}) as Validation
  Object.assign(form, {
    code: a.code ?? '',
    label: a.label ?? '',
    data_type: a.data_type ?? 'text',
    is_filterable: a.is_filterable ?? false,
    is_variant_axis: a.is_variant_axis ?? false,
    options: (a.options ?? []).join(', '),
    pattern: v.pattern ?? '',
    min_length: v.min_length ?? null,
    max_length: v.max_length ?? null,
    min: v.min ?? null,
    max: v.max ?? null,
    min_select: v.min_select ?? null,
    max_select: v.max_select ?? null,
  })
  dialogOpen.value = true
}

function buildValidation(): Validation {
  const v: Validation = {}
  if (isText.value) {
    if (form.pattern.trim()) v.pattern = form.pattern.trim()
    if (form.min_length != null) v.min_length = form.min_length
    if (form.max_length != null) v.max_length = form.max_length
  } else if (isNumeric.value) {
    if (form.min != null) v.min = form.min
    if (form.max != null) v.max = form.max
  } else if (isMulti.value) {
    if (form.min_select != null) v.min_select = form.min_select
    if (form.max_select != null) v.max_select = form.max_select
  }
  return v
}

async function save() {
  if (!form.code.trim() || !form.label.trim()) {
    toast.add({ severity: 'warn', summary: 'Code and label are required', life: 2500 })
    return
  }
  saving.value = true
  const body: AttributeInput = {
    code: form.code.trim(),
    label: form.label.trim(),
    data_type: form.data_type,
    is_filterable: form.is_filterable,
    is_variant_axis: form.is_variant_axis,
    validation: buildValidation(),
  }
  if (hasOptions.value) {
    body.options = form.options.split(',').map((s) => s.trim()).filter(Boolean)
  }
  const { error: err } = editingId.value
    ? await api.PUT('/admin/attributes/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/admin/attributes', { body })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  dialogOpen.value = false
  toast.add({ severity: 'success', summary: editingId.value ? 'Attribute updated' : 'Attribute created', life: 2000 })
  load()
}

// Human-readable summary of an attribute's rules for the table.
function rulesSummary(a: Attribute): string {
  const v = (a.validation ?? {}) as Validation
  const parts: string[] = []
  if (v.pattern) parts.push('pattern')
  if (v.min_length != null || v.max_length != null) parts.push(`${v.min_length ?? 0}–${v.max_length ?? '∞'} chars`)
  if (v.min != null || v.max != null) parts.push(`${v.min ?? '−∞'}–${v.max ?? '∞'}`)
  if (v.min_select != null || v.max_select != null) parts.push(`${v.min_select ?? 0}–${v.max_select ?? '∞'} sel`)
  if ((a.options ?? []).length) parts.push(`${a.options!.length} options`)
  return parts.join(' · ')
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Attributes">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New attribute" @click="openCreate" />
      </template>
    </PageHeader>
    <p class="muted">
      Attributes (color, size, material…) describe your products, power faceted search, and carry
      <strong>validation rules</strong> — the values products store are checked against them on every
      write and on CSV import.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <DataTable :value="rows" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-tags" title="No attributes yet" message="Attributes describe your products and power faceted search and data quality.">
          <Button icon="pi pi-plus" label="New attribute" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="code" header="Code" sortable />
      <Column field="label" header="Label" sortable />
      <Column field="data_type" header="Type" />
      <Column header="Rules">
        <template #body="{ data }"><span class="muted small">{{ rulesSummary(data) || '—' }}</span></template>
      </Column>
      <Column header="Flags">
        <template #body="{ data }">
          <Tag v-if="data.is_filterable" v-tooltip.top="'Buyers can filter the catalog by this attribute (shows as a storefront facet).'" value="filterable" severity="info" class="flag" />
          <Tag v-if="data.is_variant_axis" v-tooltip.top="'Distinguishes variants of the same product (e.g. size, colour).'" value="variant axis" severity="warn" class="flag" />
        </template>
      </Column>
      <Column header="" style="width: 4rem">
        <template #body="{ data }">
          <Button icon="pi pi-pencil" text rounded size="small" title="Edit" @click="openEdit(data)" />
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" :header="editingId ? 'Edit attribute' : 'New attribute'" modal :style="{ width: '480px' }">
      <form class="form" @submit.prevent="save">
        <div class="field">
          <label>Code</label>
          <InputText v-model="form.code" :disabled="!!editingId" fluid />
          <small v-if="editingId" class="muted">Code is the storage key and can't change.</small>
        </div>
        <div class="field"><label>Label</label><InputText v-model="form.label" fluid /></div>
        <div class="field"><label>Data type</label><Select v-model="form.data_type" :options="dataTypes" fluid /></div>

        <div v-if="hasOptions" class="field">
          <label>Allowed options <span class="muted">(comma-separated)</span></label>
          <InputText v-model="form.options" placeholder="red, blue, green" fluid />
          <small class="muted">Values must be one of these. Leave empty to allow any value.</small>
        </div>

        <!-- Validation rules, by data type -->
        <template v-if="isText">
          <div class="field"><label>Pattern <span class="muted">(regex, optional)</span></label><InputText v-model="form.pattern" placeholder="^[A-Z]{2,4}$" fluid /></div>
          <div class="row">
            <div class="field"><label>Min length</label><InputNumber v-model="form.min_length" :useGrouping="false" :min="0" fluid /></div>
            <div class="field"><label>Max length</label><InputNumber v-model="form.max_length" :useGrouping="false" :min="0" fluid /></div>
          </div>
        </template>
        <div v-else-if="isNumeric" class="row">
          <div class="field"><label>Min</label><InputNumber v-model="form.min" :useGrouping="false" fluid /></div>
          <div class="field"><label>Max</label><InputNumber v-model="form.max" :useGrouping="false" fluid /></div>
        </div>
        <div v-else-if="isMulti" class="row">
          <div class="field"><label>Min selections</label><InputNumber v-model="form.min_select" :useGrouping="false" :min="0" fluid /></div>
          <div class="field"><label>Max selections</label><InputNumber v-model="form.max_select" :useGrouping="false" :min="0" fluid /></div>
        </div>

        <div class="check"><Checkbox v-model="form.is_filterable" binary inputId="filt" /><label for="filt">Filterable (facet)</label></div>
        <div class="check"><Checkbox v-model="form.is_variant_axis" binary inputId="var" /><label for="var">Variant axis</label></div>
      </form>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button :label="editingId ? 'Save' : 'Create'" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.small { font-size: 0.82rem; }
.mb { margin-bottom: 1rem; }
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
.row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.check { display: flex; align-items: center; gap: 0.5rem; }
.flag { margin-right: 0.35rem; }
</style>
