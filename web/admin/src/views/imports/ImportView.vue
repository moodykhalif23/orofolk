<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useAuthStore } from '@/stores/auth'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'

type ImportTarget = components['schemas']['ImportTarget']
type ImportRun = components['schemas']['ImportRun']
type ImportRow = components['schemas']['ImportRow']

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

const targetOptions = computed(() => targets.value.map((t) => ({ label: t.label, value: t.key })))
const selected = computed(() => targets.value.find((t) => t.key === target.value) ?? null)
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
    const res = await fetch(`${apiBase}/admin/imports?target=${encodeURIComponent(target.value)}`, {
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
      Upload a CSV to create or update records in bulk. Every row is validated against its target's
      rules first, so you preview exactly what will happen — then commit. Nothing is saved until you do.
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
      <Button icon="pi pi-download" label="Template" severity="secondary" outlined :disabled="!target" @click="downloadTemplate" />
      <Button icon="pi pi-upload" label="Upload CSV" :loading="uploading" :disabled="!target" @click="pickFile" />
      <input ref="fileInput" type="file" accept=".csv,text/csv" hidden @change="onFile" />
    </div>
    <p v-if="selected" class="muted small">Columns: <code>{{ (selected.columns ?? []).join(', ') }}</code></p>

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
</style>
