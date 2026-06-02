<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Select from 'primevue/select'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Order = components['schemas']['OrderDetail']

// Mirrors the backend order state machine (sales/handler.go orderTransitions).
const TRANSITIONS: Record<string, string[]> = {
  pending: ['confirmed', 'on_hold', 'cancelled'],
  confirmed: ['processing', 'on_hold', 'cancelled'],
  processing: ['shipped', 'on_hold', 'cancelled'],
  shipped: ['delivered'],
  delivered: ['closed'],
  on_hold: ['confirmed', 'cancelled'],
}

const route = useRoute()
const router = useRouter()
const toast = useToast()
const id = Number(route.params.id)

const order = ref<Order | null>(null)
const error = ref('')
const nextStatus = ref<string | null>(null)
const note = ref('')
const applying = ref(false)

const nextOptions = computed(() => (order.value ? TRANSITIONS[order.value.status] ?? [] : []))

async function load() {
  error.value = ''
  const { data, error: err } = await api.GET('/admin/orders/{id}', { params: { path: { id } } })
  if (err || !data) {
    error.value = errMessage(err, 'Order not found')
    return
  }
  order.value = data
  nextStatus.value = null
  note.value = ''
}

async function applyStatus() {
  if (!nextStatus.value) return
  applying.value = true
  const { data, error: err } = await api.PATCH('/admin/orders/{id}/status', {
    params: { path: { id } },
    body: { status: nextStatus.value, note: note.value || null },
  })
  applying.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Transition failed', detail: errMessage(err), life: 4000 })
    return
  }
  order.value = data
  nextStatus.value = null
  note.value = ''
  toast.add({ severity: 'success', summary: `Status → ${data.status}`, life: 2500 })
}

function sev(s: string) {
  if (s === 'cancelled') return 'danger'
  if (s === 'delivered' || s === 'closed') return 'success'
  if (s === 'pending' || s === 'on_hold') return 'warn'
  return 'info'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <Button icon="pi pi-arrow-left" label="Orders" text severity="secondary" @click="router.push({ name: 'orders' })" />
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <template v-if="order">
      <div class="head">
        <h1>Order <span class="muted">{{ order.public_id.slice(0, 8) }}…</span> <Tag :value="order.status" :severity="sev(order.status)" /></h1>
        <div class="total">{{ order.grand_total }} {{ order.currency }}</div>
      </div>

      <div class="grid">
        <Card class="items">
          <template #title>Items</template>
          <template #content>
            <DataTable :value="order.items" dataKey="id" stripedRows>
              <template #empty>No items.</template>
              <Column field="sku" header="SKU" />
              <Column field="name" header="Product" />
              <Column field="quantity" header="Qty" />
              <Column field="unit_price" header="Unit price" />
              <Column field="row_total" header="Row total" />
            </DataTable>
          </template>
        </Card>

        <Card class="statuscard">
          <template #title>Change status</template>
          <template #content>
            <p v-if="!nextOptions.length" class="muted">No further transitions from <strong>{{ order.status }}</strong>.</p>
            <div v-else class="statusform">
              <Select v-model="nextStatus" :options="nextOptions" placeholder="Next status" fluid />
              <InputText v-model="note" placeholder="Note (optional)" fluid />
              <Button label="Apply" icon="pi pi-check" :disabled="!nextStatus" :loading="applying" @click="applyStatus" />
            </div>
          </template>
        </Card>
      </div>
    </template>
  </div>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; margin: 0.5rem 0 1rem; }
.head h1 { margin: 0; display: flex; align-items: center; gap: 0.6rem; font-size: 1.4rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 1rem; }
.total { font-size: 1.3rem; font-weight: 700; font-variant-numeric: tabular-nums; }
.mb { margin-bottom: 1rem; }
.grid { display: grid; grid-template-columns: 2fr 1fr; gap: 1rem; }
@media (max-width: 820px) { .grid { grid-template-columns: 1fr; } }
.statusform { display: flex; flex-direction: column; gap: 0.75rem; }
</style>
