<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import { useAuthStore } from '@/stores/auth'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Endpoint = components['schemas']['WebhookEndpoint']
type Delivery = components['schemas']['WebhookDelivery']

const toast = useToast()
const auth = useAuthStore()
const endpoints = ref<Endpoint[]>([])
const error = ref('')

const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
// event_types is edited as a comma-separated string; empty = all events.
const form = reactive({ url: '', description: '', events: '', is_active: true })

// Secret reveal — the signing secret is returned exactly once, on create/rotate.
const revealOpen = ref(false)
const revealValue = ref('')
const revealTitle = ref('')

// Deliveries drawer.
const deliveriesOpen = ref(false)
const deliveriesFor = ref<Endpoint | null>(null)
const deliveries = ref<Delivery[]>([])
const deliveriesLoading = ref(false)

async function load() {
  error.value = ''
  const { data, error: e } = await api.GET('/admin/webhooks')
  if (e) {
    error.value = errMessage(e, 'Failed to load webhooks')
    return
  }
  endpoints.value = data?.items ?? []
}

function openCreate() {
  editingId.value = null
  Object.assign(form, { url: '', description: '', events: '', is_active: true })
  dialogOpen.value = true
}
function openEdit(e: Endpoint) {
  editingId.value = e.id!
  Object.assign(form, {
    url: e.url ?? '',
    description: e.description ?? '',
    events: (e.event_types ?? []).join(', '),
    is_active: e.is_active ?? true,
  })
  dialogOpen.value = true
}

function parseEvents(s: string): string[] {
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

async function save() {
  if (!form.url.trim()) return
  saving.value = true
  const body: components['schemas']['WebhookEndpointInput'] = {
    url: form.url.trim(),
    description: form.description.trim(),
    event_types: parseEvents(form.events),
    is_active: form.is_active,
  }
  const { data, error: e } = editingId.value
    ? await api.PUT('/admin/webhooks/{id}', { params: { path: { id: editingId.value } }, body })
    : await api.POST('/admin/webhooks', { body })
  saving.value = false
  if (e) {
    toast.add({ severity: 'error', summary: errMessage(e, 'Save failed'), life: 4000 })
    return
  }
  dialogOpen.value = false
  // Only a freshly-created endpoint returns its secret to reveal.
  const secret = (data as components['schemas']['WebhookEndpointCreated'] | undefined)?.secret
  if (!editingId.value && secret) reveal('Webhook created', secret)
  else toast.add({ severity: 'success', summary: 'Webhook saved', life: 2000 })
  load()
}

async function rotateSecret(e: Endpoint) {
  const { data, error: err } = await api.POST('/admin/webhooks/{id}/rotate-secret', {
    params: { path: { id: e.id! } },
  })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Rotate failed'), life: 4000 })
    return
  }
  reveal('Signing secret rotated', data?.secret ?? '')
}

async function remove(e: Endpoint) {
  if (!confirm(`Delete the webhook for "${e.url}"? It will stop receiving events.`)) return
  const { error: err } = await api.DELETE('/admin/webhooks/{id}', { params: { path: { id: e.id! } } })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Delete failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Webhook deleted', life: 2000 })
  load()
}

async function openDeliveries(e: Endpoint) {
  deliveriesFor.value = e
  deliveriesOpen.value = true
  deliveries.value = []
  deliveriesLoading.value = true
  const { data, error: err } = await api.GET('/admin/webhooks/{id}/deliveries', {
    params: { path: { id: e.id! } },
  })
  deliveriesLoading.value = false
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Failed to load deliveries'), life: 4000 })
    return
  }
  deliveries.value = data?.items ?? []
}

async function replay(d: Delivery) {
  const { error: err } = await api.POST('/admin/webhooks/deliveries/{id}/replay', {
    params: { path: { id: d.id! } },
  })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Replay failed'), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Re-queued for delivery', life: 2000 })
  if (deliveriesFor.value) openDeliveries(deliveriesFor.value)
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

const statusSev = (s?: string) => (s === 'success' ? 'success' : s === 'failed' ? 'danger' : 'secondary')
const fmt = (d?: string) => (d ? new Date(d).toLocaleString() : '—')

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Webhooks">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button
          v-if="auth.can('webhook.manage')"
          icon="pi pi-plus"
          label="New webhook"
          @click="openCreate"
        />
      </template>
    </PageHeader>
    <p class="muted">
      Subscribe an external URL to your store's events. Each delivery is a signed
      <code>POST</code> (verify the <code>X-Teggo-Signature</code> HMAC with the endpoint secret).
      This is also how you connect Zapier, n8n or Make. Leave events empty to receive everything.
    </p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="endpoints" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState
          icon="pi pi-send"
          title="No webhooks yet"
          message="Point an endpoint at your automation tool or service to receive events as they happen."
        >
          <Button
            v-if="auth.can('webhook.manage')"
            icon="pi pi-plus"
            label="New webhook"
            @click="openCreate"
          />
        </EmptyState>
      </template>
      <Column field="url" header="Endpoint" />
      <Column header="Events">
        <template #body="{ data }">
          <span v-if="(data.event_types ?? []).length" class="muted">{{ data.event_types.join(', ') }}</span>
          <Tag v-else value="all events" severity="info" />
        </template>
      </Column>
      <Column header="Active">
        <template #body="{ data }">
          <Tag :value="data.is_active ? 'active' : 'off'" :severity="data.is_active ? 'success' : 'secondary'" />
        </template>
      </Column>
      <Column header="" style="width: 11rem">
        <template #body="{ data }">
          <Button icon="pi pi-history" text rounded size="small" title="Delivery log" @click="openDeliveries(data)" />
          <template v-if="auth.can('webhook.manage')">
            <Button icon="pi pi-pencil" text rounded size="small" title="Edit" @click="openEdit(data)" />
            <Button icon="pi pi-key" text rounded size="small" title="Rotate signing secret" @click="rotateSecret(data)" />
            <Button icon="pi pi-trash" text rounded size="small" severity="danger" title="Delete" @click="remove(data)" />
          </template>
        </template>
      </Column>
    </DataTable>

    <!-- Create / edit -->
    <Dialog v-model:visible="dialogOpen" modal :header="editingId ? 'Edit webhook' : 'New webhook'" :style="{ width: '36rem' }">
      <div class="field">
        <label>Endpoint URL</label>
        <InputText v-model="form.url" placeholder="https://hooks.example.com/teggo" autofocus />
      </div>
      <div class="field">
        <label>Description <span class="muted">(optional)</span></label>
        <InputText v-model="form.description" placeholder="What this endpoint is for" />
      </div>
      <div class="field">
        <label>Events <span class="muted">(comma-separated; blank = all)</span></label>
        <Textarea v-model="form.events" rows="2" placeholder="order.status_changed, quote.created" />
        <small class="muted">Subscribe to specific domain events, or leave empty to receive every event.</small>
      </div>
      <div class="field-row">
        <ToggleSwitch v-model="form.is_active" inputId="wh-active" />
        <label for="wh-active">Active</label>
      </div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" :disabled="!form.url.trim()" @click="save" />
      </template>
    </Dialog>

    <!-- Reveal secret (shown exactly once) -->
    <Dialog v-model:visible="revealOpen" modal :header="revealTitle" :style="{ width: '34rem' }">
      <Message severity="warn" :closable="false" class="mb">
        Copy this signing secret now — it will not be shown again. Use it to verify the
        <code>X-Teggo-Signature</code> header on incoming deliveries.
      </Message>
      <div class="reveal">
        <InputText :value="revealValue" readonly class="reveal-input" />
        <Button icon="pi pi-copy" label="Copy" @click="copyReveal" />
      </div>
      <template #footer>
        <Button label="Done" @click="revealOpen = false" />
      </template>
    </Dialog>

    <!-- Delivery log -->
    <Dialog
      v-model:visible="deliveriesOpen"
      modal
      :header="`Deliveries — ${deliveriesFor?.url ?? ''}`"
      :style="{ width: '52rem' }"
    >
      <DataTable :value="deliveries" dataKey="id" stripedRows scrollable scrollHeight="420px" :loading="deliveriesLoading">
        <template #empty>No deliveries recorded yet.</template>
        <Column field="event_type" header="Event" />
        <Column header="Status">
          <template #body="{ data }"><Tag :value="data.status" :severity="statusSev(data.status)" /></template>
        </Column>
        <Column field="attempt" header="Attempt" style="width: 6rem" />
        <Column field="response_status" header="HTTP" style="width: 5rem" />
        <Column header="Error">
          <template #body="{ data }"><span class="muted">{{ data.error || '—' }}</span></template>
        </Column>
        <Column header="When">
          <template #body="{ data }"><span class="muted">{{ fmt(data.created_at) }}</span></template>
        </Column>
        <Column v-if="auth.can('webhook.manage')" header="" style="width: 6rem">
          <template #body="{ data }">
            <Button icon="pi pi-replay" text rounded size="small" title="Replay this delivery" @click="replay(data)" />
          </template>
        </Column>
      </DataTable>
    </Dialog>
  </div>
</template>

<style scoped>
.muted { color: var(--p-text-muted-color, #64748b); }
.mb { margin-bottom: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.field-row { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.5rem; }
.field-row label { font-size: 0.9rem; font-weight: 600; }
.reveal { display: flex; gap: 0.5rem; align-items: center; }
.reveal-input { flex: 1; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
