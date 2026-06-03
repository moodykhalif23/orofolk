<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import Button from 'primevue/button'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import InputText from 'primevue/inputtext'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useAuthStore } from '@/stores/auth'
import type { components } from '@teggo/api/schema'

type ReportDefinition = components['schemas']['ReportDefinition']
type EntitySchema = components['schemas']['ReportEntitySchema']
type Measure = components['schemas']['ReportMeasure']
type Filter = components['schemas']['ReportFilter']

const auth = useAuthStore()
const toast = useToast()
const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''

const defs = ref<ReportDefinition[]>([])
const schema = ref<Record<string, EntitySchema>>({})
const error = ref('')
const aggOptions = ['sum', 'avg', 'min', 'max']
const opOptions = ['eq', 'ne', 'gt', 'gte', 'lt', 'lte']
const cadences = ['daily', 'weekly', 'monthly']

// ---- editor state ----
const editing = ref<number | null>(null)
const form = reactive({
  name: '',
  entity: '',
  dimensions: [] as string[],
  measures: [] as Measure[],
  filters: [] as Filter[],
})
const saving = ref(false)

const entityNames = computed(() => Object.keys(schema.value))
const entityDims = computed(() => schema.value[form.entity]?.dimensions ?? [])
const entityMeasures = computed(() => schema.value[form.entity]?.measures ?? [])
const entityFilters = computed(() => schema.value[form.entity]?.filters ?? [])

async function load() {
  error.value = ''
  const { data, error: err } = await api.GET('/admin/reports')
  const { data: ent } = await api.GET('/admin/reports/entities')
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load reports')
    return
  }
  defs.value = data.items ?? []
  schema.value = ent?.entities ?? {}
}

function newReport() {
  editing.value = null
  Object.assign(form, { name: '', entity: entityNames.value[0] ?? '', dimensions: [], measures: [], filters: [] })
  runResult.value = null
  runs.value = []
  schedules.value = []
}

function editReport(d: ReportDefinition) {
  editing.value = d.id
  Object.assign(form, {
    name: d.name,
    entity: d.entity,
    dimensions: [...(d.dimensions ?? [])],
    measures: (d.measures ?? []).map((m) => ({ ...m })),
    filters: (d.filters ?? []).map((f) => ({ ...f })),
  })
  runResult.value = null
  loadRuns(d.id)
  loadSchedules(d.id)
}

function addMeasure() {
  form.measures.push({ field: 'count', agg: 'count' })
}
function onMeasureField(m: Measure) {
  if (m.field === 'count') m.agg = 'count'
  else if (m.agg === 'count') m.agg = 'sum'
}
function addFilter() {
  form.filters.push({ field: entityFilters.value[0] ?? '', op: 'eq', value: '' })
}

// Normalize measures: "count" pseudo-field → {agg:'count'} with no field.
function payload() {
  const measures = form.measures.map((m) =>
    m.field === 'count' ? { agg: 'count' } : { field: m.field, agg: m.agg },
  )
  return { name: form.name, entity: form.entity, dimensions: form.dimensions, measures, filters: form.filters }
}

async function save() {
  if (!form.name || !form.entity || form.measures.length === 0) {
    toast.add({ severity: 'warn', summary: 'Name, entity, and at least one measure are required', life: 3000 })
    return
  }
  saving.value = true
  const body = payload() as components['schemas']['ReportDefinitionInput']
  const { data, error: err } = editing.value
    ? await api.PUT('/admin/reports/{id}', { params: { path: { id: editing.value } }, body })
    : await api.POST('/admin/reports', { body })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Save failed (unknown field?)'), life: 5000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Report saved', life: 2000 })
  if (data) editing.value = data.id
  load()
}

async function remove(d: ReportDefinition) {
  await api.DELETE('/admin/reports/{id}', { params: { path: { id: d.id } } })
  if (editing.value === d.id) newReport()
  load()
}

// ---- run ----
const runResult = ref<components['schemas']['ReportRunResult'] | null>(null)
const running = ref(false)
async function run() {
  if (editing.value == null) return
  running.value = true
  const { data, error: err } = await api.POST('/admin/reports/{id}/run', { params: { path: { id: editing.value } } })
  running.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Run failed'), life: 5000 })
    return
  }
  runResult.value = data
  loadRuns(editing.value)
}

const runRows = computed(() =>
  (runResult.value?.rows ?? []).map((r) => {
    const o: Record<string, string> = {}
    ;(runResult.value?.columns ?? []).forEach((c, i) => (o[c] = r[i] ?? ''))
    return o
  }),
)

function download(url: string) {
  // Authenticated fetch → blob (the download route requires a bearer token).
  fetch(`${apiBase}${url}`, { headers: { Authorization: `Bearer ${auth.token ?? ''}` } })
    .then((res) => res.blob())
    .then((blob) => {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = url.split('/').pop() ?? 'report.csv'
      a.click()
      URL.revokeObjectURL(a.href)
    })
}
async function runCsv() {
  if (editing.value == null) return
  // run with format=csv records an artifact; download it via the run list.
  await api.POST('/admin/reports/{id}/run', { params: { path: { id: editing.value }, query: { format: 'csv' } } })
  await loadRuns(editing.value)
  const withFile = runs.value.find((r) => r.file_url)
  if (withFile?.file_url) download(withFile.file_url)
}

// ---- runs + schedules ----
const runs = ref<components['schemas']['ReportRun'][]>([])
async function loadRuns(id: number) {
  const { data } = await api.GET('/admin/reports/{id}/runs', { params: { path: { id } } })
  runs.value = data?.items ?? []
}

const schedules = ref<components['schemas']['ReportSchedule'][]>([])
const sched = reactive({ cadence: 'daily', recipients: '' })
async function loadSchedules(id: number) {
  const { data } = await api.GET('/admin/reports/{id}/schedules', { params: { path: { id } } })
  schedules.value = data?.items ?? []
}
async function addSchedule() {
  if (editing.value == null) return
  const recipients = sched.recipients.split(',').map((s) => s.trim()).filter(Boolean)
  const { error: err } = await api.POST('/admin/reports/{id}/schedules', {
    params: { path: { id: editing.value } },
    body: { cadence: sched.cadence as 'daily' | 'weekly' | 'monthly', recipients },
  })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Schedule failed'), life: 4000 })
    return
  }
  sched.recipients = ''
  loadSchedules(editing.value)
}
async function removeSchedule(sid: number) {
  if (editing.value == null) return
  await api.DELETE('/admin/reports/{id}/schedules/{schedID}', {
    params: { path: { id: editing.value, schedID: sid } },
  })
  loadSchedules(editing.value)
}

onMounted(load)
</script>

<template>
  <div class="page">
    <h1>Report builder</h1>
    <p class="muted">Pick an entity, dimensions, measures, and filters — no SQL. Run on demand or schedule a CSV export.</p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div class="cols">
      <!-- Saved reports -->
      <aside class="saved">
        <Button icon="pi pi-plus" label="New report" size="small" class="mb" @click="newReport" />
        <ul>
          <li v-for="d in defs" :key="d.id" :class="{ active: editing === d.id }">
            <button class="link" @click="editReport(d)">{{ d.name }} <span class="muted">· {{ d.entity }}</span></button>
            <Button icon="pi pi-trash" severity="danger" text rounded size="small" @click="remove(d)" />
          </li>
        </ul>
      </aside>

      <!-- Editor -->
      <section class="editor">
        <div class="row2">
          <div class="field"><label>Name</label><InputText v-model="form.name" /></div>
          <div class="field"><label>Entity</label><Select v-model="form.entity" :options="entityNames" placeholder="Select entity" /></div>
        </div>

        <div class="field">
          <label>Dimensions (group by)</label>
          <MultiSelect v-model="form.dimensions" :options="entityDims" placeholder="None (single total row)" display="chip" />
        </div>

        <div class="field">
          <div class="lbl-row"><label>Measures</label><Button icon="pi pi-plus" label="Add" text size="small" @click="addMeasure" /></div>
          <div v-for="(m, i) in form.measures" :key="i" class="mrow">
            <Select v-model="m.field" :options="['count', ...entityMeasures]" @change="onMeasureField(m)" />
            <Select v-if="m.field !== 'count'" v-model="m.agg" :options="aggOptions" />
            <span v-else class="muted">count(*)</span>
            <Button icon="pi pi-times" text rounded size="small" @click="form.measures.splice(i, 1)" />
          </div>
        </div>

        <div class="field">
          <div class="lbl-row"><label>Filters</label><Button icon="pi pi-plus" label="Add" text size="small" @click="addFilter" /></div>
          <div v-for="(f, i) in form.filters" :key="i" class="mrow">
            <Select v-model="f.field" :options="entityFilters" />
            <Select v-model="f.op" :options="opOptions" />
            <InputText v-model="f.value as string" placeholder="value" />
            <Button icon="pi pi-times" text rounded size="small" @click="form.filters.splice(i, 1)" />
          </div>
        </div>

        <div class="actions">
          <Button label="Save" icon="pi pi-save" :loading="saving" @click="save" />
          <Button label="Run" icon="pi pi-play" severity="secondary" :disabled="editing == null" :loading="running" @click="run" />
          <Button label="Download CSV" icon="pi pi-download" severity="secondary" text :disabled="editing == null" @click="runCsv" />
        </div>

        <!-- Run result -->
        <div v-if="runResult" class="result">
          <h3>Result <span class="muted">· {{ runResult.row_count }} rows</span></h3>
          <DataTable :value="runRows" stripedRows scrollable scrollHeight="320px">
            <Column v-for="c in runResult.columns" :key="c" :field="c" :header="c" />
          </DataTable>
        </div>

        <!-- Runs + schedules -->
        <div v-if="editing != null" class="row2 mt">
          <div>
            <h3>Recent runs</h3>
            <ul class="runs">
              <li v-for="r in runs" :key="r.id">
                <Tag :value="r.status" :severity="r.status === 'ok' ? 'success' : r.status === 'error' ? 'danger' : 'warn'" />
                <span class="muted">{{ r.trigger }}</span>
                <span v-if="r.row_count != null">· {{ r.row_count }} rows</span>
                <a v-if="r.file_url" href="#" @click.prevent="download(r.file_url!)">download</a>
              </li>
              <li v-if="!runs.length" class="muted">No runs yet.</li>
            </ul>
          </div>
          <div>
            <h3>Schedules</h3>
            <ul class="runs">
              <li v-for="s in schedules" :key="s.id">
                <Tag :value="s.cadence" /> <span class="muted">{{ (s.recipients ?? []).join(', ') || 'no recipients' }}</span>
                <Button icon="pi pi-times" text rounded size="small" @click="removeSchedule(s.id)" />
              </li>
              <li v-if="!schedules.length" class="muted">No schedules.</li>
            </ul>
            <div class="mrow">
              <Select v-model="sched.cadence" :options="cadences" />
              <InputText v-model="sched.recipients" placeholder="emails, comma-sep" />
              <Button icon="pi pi-plus" label="Add" size="small" @click="addSchedule" />
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
h1 { margin: 0 0 0.25rem; }
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
.mt { margin-top: 1.5rem; }
.cols { display: grid; grid-template-columns: 16rem 1fr; gap: 1.5rem; margin-top: 1rem; }
.saved ul { list-style: none; padding: 0; margin: 0; }
.saved li { display: flex; align-items: center; justify-content: space-between; padding: 0.25rem 0; }
.saved li.active .link { font-weight: 700; }
.link { background: none; border: none; cursor: pointer; text-align: left; color: inherit; padding: 0.25rem 0; }
.editor { min-width: 0; }
.row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.lbl-row { display: flex; align-items: center; justify-content: space-between; }
.mrow { display: flex; gap: 0.5rem; align-items: center; margin-bottom: 0.5rem; }
.mrow :deep(.p-select), .mrow :deep(.p-inputtext) { min-width: 8rem; }
.actions { display: flex; gap: 0.5rem; margin: 1rem 0; }
.result { margin-top: 1rem; }
.runs { list-style: none; padding: 0; margin: 0; }
.runs li { display: flex; align-items: center; gap: 0.5rem; padding: 0.25rem 0; font-size: 0.9rem; }
</style>
