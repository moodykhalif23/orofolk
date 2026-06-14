<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import SkeletonDetail from '@/components/SkeletonDetail.vue'

type Rfq = components['schemas']['RfqDetail']

const route = useRoute()
const router = useRouter()
const toast = useToast()
const id = Number(route.params.id)

const rfq = ref<Rfq | null>(null)
const error = ref('')
const creating = ref(false)

async function load() {
  error.value = ''
  const { data, error: err } = await api.GET('/admin/rfqs/{id}', { params: { path: { id } } })
  if (err || !data) {
    error.value = errMessage(err, 'RFQ not found')
    return
  }
  rfq.value = data
}

async function createQuote() {
  creating.value = true
  const { data, error: err } = await api.POST('/admin/rfqs/{id}/quote', { params: { path: { id } } })
  creating.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errMessage(err), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: 'Quote drafted from RFQ', life: 2500 })
  router.push({ name: 'quote-editor', params: { id: data.id } })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <AppBreadcrumb :items="[{ label: 'RFQs', route: { name: 'rfqs' } }, { label: rfq ? `RFQ ${rfq.public_id.slice(0, 8)}…` : 'RFQ' }]" />
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <SkeletonDetail v-if="!rfq && !error" :cards="2" />

    <template v-if="rfq">
      <div class="head">
        <h1>RFQ <span class="muted">{{ rfq.public_id.slice(0, 8) }}…</span> <Tag :value="rfq.status" /></h1>
        <Button
          v-if="rfq.status === 'submitted'"
          label="Create quote"
          icon="pi pi-file-edit"
          :loading="creating"
          @click="createQuote"
        />
      </div>

      <Message v-if="rfq.notes" severity="secondary" :closable="false" class="mb">{{ rfq.notes }}</Message>

      <Card>
        <template #title>Requested items</template>
        <template #content>
          <DataTable :value="rfq.items" dataKey="id" stripedRows>
            <template #empty>No items.</template>
            <Column field="sku" header="SKU" />
            <Column field="name" header="Product" />
            <Column field="quantity" header="Qty" />
            <Column field="unit" header="Unit" />
            <Column header="Target price">
              <template #body="{ data }">{{ data.target_price ?? '—' }}</template>
            </Column>
          </DataTable>
        </template>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; margin: 0.5rem 0 1rem; }
.head h1 { margin: 0; display: flex; align-items: center; gap: 0.6rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 1rem; }
.mb { margin-bottom: 1rem; }
</style>
