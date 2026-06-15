<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import MultiSelect from 'primevue/multiselect'
import DatePicker from 'primevue/datepicker'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import { useAuthStore } from '@/stores/auth'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type ApiKey = components['schemas']['ApiKey']

const toast = useToast()
const auth = useAuthStore()
const keys = ref<ApiKey[]>([])
const error = ref('')

const dialogOpen = ref(false)
const saving = ref(false)
const form = reactive({ name: '', scopes: [] as string[], expires_at: null as Date | null })

// A key may only be granted scopes the signed-in user already holds — the same
// subset rule the server enforces, surfaced here so the picker can't over-grant.
const scopeOptions = computed(() => [...auth.permissions].sort().map((p) => ({ label: p, value: p })))

// Secret reveal — the raw key is returned exactly once, on create/rotate.
const revealOpen = ref(false)
const revealValue = ref('')
const revealTitle = ref('')

async function load() {
  error.value = ''
  const { data, error: e } = await api.GET('/admin/api-keys')
  if (e) {
    error.value = errMessage(e, 'Failed to load API keys')
    return
  }
  keys.value = data?.items ?? []
}

function openCreate() {
  Object.assign(form, { name: '', scopes: [], expires_at: null })
  dialogOpen.value = true
}

async function create() {
  if (!form.name.trim()) return
  saving.value = true
  const body: components['schemas']['ApiKeyInput'] = {
    name: form.name.trim(),
    scopes: form.scopes,
    ...(form.expires_at ? { expires_at: form.expires_at.toISOString() } : {}),
  }
  const { data, error: e } = await api.POST('/admin/api-keys', { body })
  saving.value = false
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Create failed'), life: 4000 })
    return
  }
  dialogOpen.value = false
  reveal('API key created', data?.key ?? '')
  load()
}

async function rotate(k: ApiKey) {
  const { data, error: e } = await api.POST('/admin/api-keys/{id}/rotate', {
    params: { path: { id: k.id! } },
  })
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Rotate failed'), life: 4000 })
    return
  }
  reveal('API key rotated', data?.key ?? '')
  load()
}

async function revoke(k: ApiKey) {
  if (!confirm(`Revoke "${k.name}"? Any integration using it stops working immediately.`)) return
  const { error: e } = await api.DELETE('/admin/api-keys/{id}', { params: { path: { id: k.id! } } })
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Revoke failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Key revoked', life: 2000 })
  load()
}

function reveal(title: string, value: string) {
  revealTitle.value = title
  revealValue.value = value
  revealOpen.value = true
}

async function copyReveal() {
  try {
    await navigator.clipboard.writeText(revealValue.value)
    toast.add({ severity: 'success', summary: 'Copied to clipboard', life: 1500 })
  } catch {
    /* clipboard blocked — the value is selectable in the field */
  }
}

const statusSev = (s?: string) => (s === 'active' ? 'success' : s === 'revoked' ? 'danger' : 'warn')
const fmt = (d?: string) => (d ? new Date(d).toLocaleString() : '—')

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="API keys">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button
          v-if="auth.can('apikey.manage')"
          icon="pi pi-plus"
          label="New API key"
          @click="openCreate"
        />
      </template>
    </PageHeader>
    <p class="muted">
      Programmatic keys let integrations call the admin API with
      <code>Authorization: Bearer tgk_…</code> instead of a user login. The secret is shown only
      once — store it somewhere safe. A key can do exactly what its scopes allow.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="keys" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState
          icon="pi pi-key"
          title="No API keys yet"
          message="Create a scoped key so an external system can talk to your store programmatically."
        >
          <Button
            v-if="auth.can('apikey.manage')"
            icon="pi pi-plus"
            label="New API key"
            @click="openCreate"
          />
        </EmptyState>
      </template>
      <Column field="name" header="Name" />
      <Column header="Key">
        <template #body="{ data }"><code>{{ data.prefix }}…</code></template>
      </Column>
      <Column header="Scopes">
        <template #body="{ data }">
          <span class="muted">{{ (data.scopes ?? []).length }} scope{{ (data.scopes ?? []).length === 1 ? '' : 's' }}</span>
        </template>
      </Column>
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.status" :severity="statusSev(data.status)" /></template>
      </Column>
      <Column header="Last used">
        <template #body="{ data }"><span class="muted">{{ fmt(data.last_used_at) }}</span></template>
      </Column>
      <Column header="Expires">
        <template #body="{ data }"><span class="muted">{{ fmt(data.expires_at) }}</span></template>
      </Column>
      <Column v-if="auth.can('apikey.manage')" header="" style="width: 7rem">
        <template #body="{ data }">
          <Button
            v-if="data.status !== 'revoked'"
            icon="pi pi-sync"
            text
            rounded
            size="small"
            title="Rotate — issues a new secret, invalidates the old one"
            @click="rotate(data)"
          />
          <Button
            v-if="data.status !== 'revoked'"
            icon="pi pi-trash"
            text
            rounded
            size="small"
            severity="danger"
            title="Revoke"
            @click="revoke(data)"
          />
        </template>
      </Column>
    </DataTable>

    <!-- Create -->
    <Dialog v-model:visible="dialogOpen" modal header="New API key" :style="{ width: '34rem' }">
      <div class="field">
        <label>Name</label>
        <InputText v-model="form.name" placeholder="e.g. Zapier integration" autofocus />
      </div>
      <div class="field">
        <label>Scopes</label>
        <MultiSelect
          v-model="form.scopes"
          :options="scopeOptions"
          optionLabel="label"
          optionValue="value"
          filter
          display="chip"
          placeholder="Select what this key may do"
        />
        <small class="muted">Only your own permissions are offered. A key can never exceed your access.</small>
      </div>
      <div class="field">
        <label>Expires <span class="muted">(optional)</span></label>
        <DatePicker v-model="form.expires_at" showTime hourFormat="24" showButtonBar :minDate="new Date()" />
        <small class="muted">Leave empty for a key that never expires.</small>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Create key" :loading="saving" :disabled="!form.name.trim()" @click="create" />
      </template>
    </Dialog>

    <!-- Reveal (shown exactly once) -->
    <Dialog v-model:visible="revealOpen" modal :header="revealTitle" :style="{ width: '34rem' }" :closable="true">
      <Message severity="warn" :closable="false" class="mb">
        Copy this secret now — it will not be shown again.
      </Message>
      <div class="reveal">
        <InputText :value="revealValue" readonly class="reveal-input" />
        <Button icon="pi pi-copy" label="Copy" @click="copyReveal" />
      </div>
      <template #footer>
        <Button label="Done" @click="revealOpen = false" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.reveal { display: flex; gap: 0.5rem; align-items: center; }
.reveal-input { flex: 1; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
