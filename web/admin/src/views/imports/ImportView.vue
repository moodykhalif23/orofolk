<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useAuthStore } from '@/stores/auth'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'

type ImportTarget = components['schemas']['ImportTarget']
type ImportRun = components['schemas']['ImportRun']
type ImportRow = components['schemas']['ImportRow']
type ImportFieldSpec = components['schemas']['ImportFieldSpec']

const route = useRoute()
const toast = useToast()
const auth = useAuthStore()
const apiBase = import.meta.env.VITE_API_BASE_URL ?? ''

const targets = ref<ImportTarget[]>([])
const target = ref<string>('')
const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const committing = ref(false)
const run = ref<ImportRun | null>(null)
const preview = ref<ImportRow[]>([])
const runs = ref<ImportRun[]>([])
const error = ref('')
const matchField = ref('')
const normalize = ref<string[]>(['trim'])

const normalizeOptions = [
  { label: 'Trim spaces', value: 'trim' },
  { label: 'Lowercase', value: 'lower' },
  { label: 'Uppercase', value: 'upper' },
  { label: 'Collapse spaces', value: 'collapse' },
]

const targetOptions = computed(() => targets.value.map((t) => ({ label: t.label, value: t.key })))
const selected = computed(() => targets.value.find((t) => t.key === target.value) ?? null)
const matchOptions = computed(() => (selected.value?.columns ?? []).map((c) => ({ label: c, value: c })))

// Partner-ingest contract: the same engine, reached in one call by a supplier
// API key scoped to import.ingest. Tucked away — most users use the wizard above.
const showApi = ref(false)
const fields = computed<ImportFieldSpec[]>(() => selected.value?.fields ?? [])
const ingestUrl = computed(() => {
  const base = apiBase || window.location.origin
  const params = new URLSearchParams({ target: target.value })
  if (matchField.value) params.set('match', matchField.value)
  return `${base}/admin/imports/ingest?${params.toString()}`
})
function placeholderFor(f: ImportFieldSpec): unknown {
  switch (f.data_type) {
    case 'number':
    case 'price':
      return 0
    case 'boolean':
      return true
    case 'multiselect':
      return f.options?.slice(0, 1) ?? []
    case 'select':
      return f.options?.[0] ?? '…'
    case 'date':
      return '2026-01-01'
    case 'json':
      return {}
    default:
      return '…'
  }
}
const curlSample = computed(() => {
  const obj: Record<string, unknown> = {}
  for (const f of fields.value.slice(0, 6)) if (f.code) obj[f.code] = placeholderFor(f)
  const body = JSON.stringify([obj])
  return (
    `curl -X POST "${ingestUrl.value}" \\\n` +
    `  -H "Authorization: Bearer tgk_…" \\\n` +
    `  -H "Content-Type: application/json" \\\n` +
    `  -d '${body}'`
  )
})
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.add({ severity: 'success', summary: 'Copied to clipboard', life: 1500 })
  } catch {
    /* clipboard blocked — the text is selectable */
  }
}
const canCommit = computed(
  () => !!run.value && run.value.status === 'validated' && ((run.value.create_rows ?? 0) + (run.value.update_rows ?? 0)) > 0,
)

async function loadTargets() {
  const { data, error: e } = await api.GET('/admin/imports/targets')
  if (e) {
    error.value = errMessage(e, 'Failed to load import targets')
    return
  }
  targets.value = data?.targets ?? []
  const q = String(route.query.target ?? '')
  if (q && targets.value.some((t) => t.key === q)) {
    target.value = q
  } else if (!target.value && targets.value.length) {
    target.value = targets.value[0].key ?? ''
  }
}

async function loadRuns() {
  const { data } = await api.GET('/admin/imports/runs')
  runs.value = data?.items ?? []
}

function pickFile() {
  fileInput.value?.click()
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f || !target.value) return
  uploading.value = true
  error.value = ''
  run.value = null
  preview.value = []
  try {
    const fd = new FormData()
    fd.append('file', f)
    const params = new URLSearchParams({ target: target.value })
    if (/\.xlsx$/i.test(f.name)) params.set('format', 'xlsx')
    if (matchField.value) params.set('match', matchField.value)
    if (normalize.value.length) params.set('normalize', normalize.value.join(','))
    const res = await fetch(`${apiBase}/admin/imports?${params.toString()}`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${auth.token ?? ''}` },
      body: fd,
    })
    if (!res.ok) {
      const b = await res.json().catch(() => ({}))
      throw new Error(b.message ?? `Upload failed (${res.status})`)
    }
    const body = (await res.json()) as { run: ImportRun; preview: ImportRow[] }
    run.value = body.run
    preview.value = body.preview ?? []
    loadRuns()
  } catch (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Upload failed'), life: 4000 })
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function downloadTemplate() {
  try {
    const res = await fetch(`${apiBase}/admin/imports/template?target=${encodeURIComponent(target.value)}`, {
      headers: { Authorization: `Bearer ${auth.token ?? ''}` },
    })
    if (!res.ok) throw new Error(`Template failed (${res.status})`)
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = target.value.replace(':', '-') + '-template.csv'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Template download failed'), life: 4000 })
  }
}

async function commit() {
  if (!run.value?.id) return
  committing.value = true
  const { data, error: e } = await api.POST('/admin/imports/runs/{id}/commit', {
    params: { path: { id: run.value.id } },
  })
  committing.value = false
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Commit failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: `Imported ${data?.committed ?? 0} rows`, life: 3000 })
  run.value = null
  preview.value = []
  loadRuns()
}

const rowSev = (s?: string) =>
  s === 'create' ? 'success' : s === 'update' ? 'info' : s === 'error' ? 'danger' : 'secondary'
const runSev = (s?: string) => (s === 'committed' ? 'success' : s === 'validated' ? 'info' : 'secondary')
const fmt = (d?: string) => (d ? new Date(d).toLocaleString() : '—')

function summarize(d: unknown): string {
  if (!d || typeof d !== 'object') return ''
  return Object.entries(d as Record<string, unknown>)
    .slice(0, 4)
    .map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join('|') : String(v)}`)
    .join(' · ')
}

onMounted(() => {
  loadTargets()
  loadRuns()
})
</script>

<template>
  <div class="page">
    <PageHeader title="Import data" />
    <p class="muted">
      Upload a CSV or Excel file to create or update records in bulk. Choose a field to match on for
      upserts, clean values on the way in, and preview every row against its target's rules before you
      commit — nothing is saved until you do.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div class="toolbar">
      <Select
        v-model="target"
        :options="targetOptions"
        optionLabel="label"
        optionValue="value"
        placeholder="What are you importing?"
        class="target-select"
      />
      <Select
        v-model="matchField"
        :options="matchOptions"
        optionLabel="label"
        optionValue="value"
        showClear
        placeholder="Match on… (insert only)"
        class="match-select"
        :disabled="!target"
      />
      <MultiSelect
        v-model="normalize"
        :options="normalizeOptions"
        optionLabel="label"
        optionValue="value"
        display="chip"
        placeholder="Clean up"
        class="norm-select"
      />
      <Button icon="pi pi-download" label="Template" severity="secondary" outlined :disabled="!target" @click="downloadTemplate" />
      <Button icon="pi pi-upload" label="Upload" :loading="uploading" :disabled="!target" @click="pickFile" />
      <input ref="fileInput" type="file" accept=".csv,.xlsx,text/csv" hidden @change="onFile" />
    </div>
    <p v-if="selected" class="muted small">Columns: <code>{{ (selected.columns ?? []).join(', ') }}</code></p>

    <!-- Partner / API ingest contract -->
    <div v-if="selected" class="api-panel">
      <button type="button" class="api-toggle" @click="showApi = !showApi">
        <i :class="showApi ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" />
        Feed this in via API <span class="muted">— for partners &amp; integrations</span>
      </button>
      <div v-if="showApi" class="api-body">
        <p class="muted small mb">
          A supplier can push <strong>{{ selected.label }}</strong> straight in with a single call —
          no preview/commit step. Mint an
          <RouterLink :to="{ name: 'api-keys' }">API key</RouterLink> scoped to
          <code>import.ingest</code> and share the contract below. Valid rows apply; rejected rows
          come back with a reason.
        </p>
        <div class="endpoint">
          <Tag value="POST" severity="contrast" />
          <code class="url">{{ ingestUrl }}</code>
          <Button icon="pi pi-copy" text rounded size="small" title="Copy URL" @click="copy(ingestUrl)" />
        </div>
        <table class="fields">
          <thead>
            <tr><th>Field</th><th>Type</th><th>Required</th><th>Rules</th></tr>
          </thead>
          <tbody>
            <tr v-for="f in fields" :key="f.code">
              <td><code>{{ f.code }}</code></td>
              <td class="muted">{{ f.data_type }}</td>
              <td>
                <Tag v-if="f.required" value="required" severity="warn" />
                <span v-else class="muted">optional</span>
              </td>
              <td class="muted small">{{ f.rule || (f.options?.length ? f.options.join(' | ') : '—') }}</td>
            </tr>
          </tbody>
        </table>
        <div class="sample-head">
          <span class="muted small">Sample request</span>
          <Button icon="pi pi-copy" label="Copy" text size="small" @click="copy(curlSample)" />
        </div>
        <pre class="sample">{{ curlSample }}</pre>
      </div>
    </div>

    <!-- Dry-run preview -->
    <template v-if="run">
      <div class="stats">
        <div class="stat"><div class="sv">{{ run.total_rows ?? 0 }}</div><div class="sl">Rows</div></div>
        <div class="stat"><div class="sv ok">{{ run.create_rows ?? 0 }}</div><div class="sl">Create</div></div>
        <div class="stat"><div class="sv info">{{ run.update_rows ?? 0 }}</div><div class="sl">Update</div></div>
        <div class="stat"><div class="sv bad">{{ run.error_rows ?? 0 }}</div><div class="sl">Errors</div></div>
      </div>
      <div class="commitbar">
        <span class="muted">Preview — nothing saved yet{{ (run.error_rows ?? 0) > 0 ? '. Rows with errors are skipped on commit.' : '.' }}</span>
        <Button label="Commit import" icon="pi pi-check" :loading="committing" :disabled="!canCommit" @click="commit" />
      </div>
      <DataTable :value="preview" dataKey="row_number" stripedRows scrollable scrollHeight="50vh" class="mb">
        <Column field="row_number" header="#" style="width: 4rem" />
        <Column header="Outcome" style="width: 8rem">
          <template #body="{ data }"><Tag :value="data.status" :severity="rowSev(data.status)" /></template>
        </Column>
        <Column header="Detail">
          <template #body="{ data }">
            <span :class="{ bad: data.status === 'error' }">{{ data.status === 'error' ? data.message : summarize(data.data) }}</span>
          </template>
        </Column>
      </DataTable>
    </template>

    <h3>Recent imports</h3>
    <DataTable :value="runs" dataKey="id" stripedRows>
      <template #empty>No imports yet.</template>
      <Column field="target" header="Target" />
      <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="runSev(data.status)" /></template></Column>
      <Column header="Rows"><template #body="{ data }"><span class="muted small">{{ data.create_rows ?? 0 }}c · {{ data.update_rows ?? 0 }}u · {{ data.error_rows ?? 0 }}e</span></template></Column>
      <Column header="When"><template #body="{ data }"><span class="muted">{{ fmt(data.created_at) }}</span></template></Column>
    </DataTable>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.small { font-size: 0.82rem; }
.mb { margin-bottom: 1rem; }
h3 { margin: 1.75rem 0 0.5rem; }
.toolbar { display: flex; align-items: center; gap: 0.6rem; margin: 0.75rem 0 0.35rem; flex-wrap: wrap; }
.target-select { min-width: 16rem; }
.match-select { min-width: 13rem; }
.norm-select { min-width: 12rem; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.stats { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 0.75rem; margin: 1rem 0 0.5rem; max-width: 36rem; }
.stat { border: 1px solid var(--teggo-border, #e2e8f0); border-radius: var(--teggo-radius, 3px); background: var(--teggo-surface, #fff); padding: 0.7rem 0.9rem; }
.sv { font-size: 1.5rem; font-weight: 800; letter-spacing: -0.02em; }
.sv.ok { color: #15803d; }
.sv.info { color: #0e7490; }
.sv.bad { color: #b91c1c; }
.sl { color: var(--p-text-muted-color, #64748b); font-size: 0.8rem; }
.commitbar { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin: 0.25rem 0 0.75rem; }
.bad { color: #b91c1c; }

/* Partner / API ingest contract */
.api-panel { margin: 0.5rem 0 0.25rem; }
.api-toggle {
  display: inline-flex; align-items: center; gap: 0.5rem; background: none; border: 0;
  padding: 0.3rem 0; cursor: pointer; font: inherit; font-weight: 600; color: var(--p-text-color, #1e293b);
}
.api-toggle .pi { font-size: 0.75rem; }
.api-body {
  border: 1px solid var(--teggo-border, #e2e8f0); border-radius: var(--teggo-radius, 3px);
  background: var(--teggo-surface, #fff); padding: 0.9rem 1rem; margin-top: 0.4rem; max-width: 52rem;
}
.endpoint { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.85rem; }
.endpoint .url {
  flex: 1; overflow-x: auto; white-space: nowrap; padding: 0.35rem 0.55rem;
  background: var(--teggo-muted-bg, #f1f5f9); border-radius: var(--teggo-radius, 3px); font-size: 0.82rem;
}
.fields { width: 100%; border-collapse: collapse; font-size: 0.85rem; margin-bottom: 0.85rem; }
.fields th { text-align: left; font-weight: 600; padding: 0.3rem 0.5rem; border-bottom: 1px solid var(--teggo-border, #e2e8f0); }
.fields td { padding: 0.3rem 0.5rem; border-bottom: 1px solid var(--teggo-border, #f1f5f9); vertical-align: top; }
.sample-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.25rem; }
.sample {
  margin: 0; padding: 0.7rem 0.85rem; overflow-x: auto; font-size: 0.8rem; line-height: 1.5;
  background: #0f172a; color: #e2e8f0; border-radius: var(--teggo-radius, 3px);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
</style>
