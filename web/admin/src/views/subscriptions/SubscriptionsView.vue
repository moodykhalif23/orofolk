<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Subscription = components['schemas']['Subscription']

const rows = ref<Subscription[]>([])
const loading = ref(false)
const error = ref('')
const toast = useToast()
const confirm = useConfirm()

const detail = ref<Subscription | null>(null)
const detailOpen = ref(false)

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/subscriptions')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load subscriptions')
    return
  }
  rows.value = data.items ?? []
}

function statusSeverity(s: string) {
  return s === 'active' ? 'success' : s === 'paused' ? 'warn' : 'secondary'
}

async function openDetail(s: Subscription) {
  const { data } = await api.GET('/admin/subscriptions/{id}', { params: { path: { id: s.id } } })
  detail.value = data ?? s
  detailOpen.value = true
}

async function setStatus(s: Subscription, status: 'active' | 'paused' | 'cancelled') {
  const { error: err } = await api.POST('/admin/subscriptions/{id}/status', {
    params: { path: { id: s.id } },
    body: { status },
  })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Update failed'), life: 4000 })
    return
  }
  load()
}

function confirmCancel(s: Subscription) {
  confirm.require({
    message: 'Cancel this subscription? It will stop creating orders.',
    header: 'Cancel subscription',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Keep', severity: 'secondary', outlined: true },
    acceptProps: { label: 'Cancel subscription', severity: 'danger' },
    accept: () => setStatus(s, 'cancelled'),
  })
}

async function runNow(s: Subscription) {
  const { data, error: err } = await api.POST('/admin/subscriptions/{id}/run', { params: { path: { id: s.id } } })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Run failed'), life: 4000 })
    return
  }
  toast.add({
    severity: data?.order_created ? 'success' : 'warn',
    summary: data?.order_created ? 'Order created' : 'No order created',
    detail: data?.order_created ? undefined : 'No current price for the subscription items.',
    life: 3500,
  })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Subscriptions" :meta="rows.length">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
      </template>
    </PageHeader>
    <p class="muted mb">
      Recurring &amp; standing orders. A daily job turns due subscriptions into orders, priced from
      the customer's current price list. Buyers can pause, skip, or cancel from their account.
    </p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="rows" :loading="loading" dataKey="id" stripedRows paginator :rows="15">
      <template #empty>
        <EmptyState icon="pi pi-sync" title="No subscriptions yet" message="When a customer sets up a recurring order, it appears here." />
      </template>
      <Column field="id" header="ID" style="width: 4rem" />
      <Column header="Name"><template #body="{ data }">{{ data.name || '—' }}</template></Column>
      <Column field="customer_id" header="Customer" />
      <Column field="cadence" header="Cadence" />
      <Column field="next_run_date" header="Next run" />
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.status" :severity="statusSeverity(data.status)" /></template>
      </Column>
      <Column header="" style="width: 12rem">
        <template #body="{ data }">
          <Button icon="pi pi-eye" severity="secondary" text rounded @click="openDetail(data)" />
          <Button v-if="data.status === 'active'" icon="pi pi-play" severity="secondary" text rounded title="Run now" @click="runNow(data)" />
          <Button v-if="data.status === 'active'" icon="pi pi-pause" severity="secondary" text rounded title="Pause" @click="setStatus(data, 'paused')" />
          <Button v-if="data.status === 'paused'" icon="pi pi-play-circle" severity="secondary" text rounded title="Resume" @click="setStatus(data, 'active')" />
          <Button v-if="data.status !== 'cancelled'" icon="pi pi-times" severity="danger" text rounded title="Cancel" @click="confirmCancel(data)" />
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="detailOpen" modal header="Subscription" :style="{ width: '42rem' }">
      <template v-if="detail">
        <div class="meta mb">
          <span><strong>{{ detail.name || `Subscription #${detail.id}` }}</strong></span>
          <Tag :value="detail.status" :severity="statusSeverity(detail.status)" />
          <span class="muted">{{ detail.cadence }} · next {{ detail.next_run_date }} · {{ detail.currency }}</span>
        </div>

        <h4>Items</h4>
        <DataTable :value="detail.items ?? []" dataKey="id" class="mb" stripedRows>
          <template #empty>No items.</template>
          <Column field="sku" header="SKU" />
          <Column field="name" header="Product" />
          <Column field="quantity" header="Qty" />
          <Column field="unit" header="Unit" />
        </DataTable>

        <h4>Recent runs</h4>
        <DataTable :value="detail.runs ?? []" dataKey="id" stripedRows>
          <template #empty>No runs yet.</template>
          <Column field="run_date" header="Date" />
          <Column header="Result">
            <template #body="{ data }">
              <Tag :value="data.status" :severity="data.status === 'success' ? 'success' : data.status === 'failed' ? 'danger' : 'secondary'" />
            </template>
          </Column>
          <Column field="order_id" header="Order">
            <template #body="{ data }">{{ data.order_id ?? '—' }}</template>
          </Column>
          <Column field="note" header="Note"><template #body="{ data }">{{ data.note ?? '' }}</template></Column>
        </DataTable>
      </template>
      <template #footer>
        <Button label="Close" severity="secondary" text @click="detailOpen = false" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.meta { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; }
h4 { margin: 0.5rem 0 0.5rem; font-size: 0.9rem; }
</style>
