<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type PendingProduct = components['schemas']['PendingProduct']

const items = ref<PendingProduct[]>([])
const loading = ref(false)
const error = ref('')
const busy = ref<number | null>(null)
const toast = useToast()

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/products/pending')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load the moderation queue')
    return
  }
  items.value = data.items ?? []
}

async function moderate(p: PendingProduct, verb: 'approve' | 'reject') {
  busy.value = p.id
  const path = `/admin/products/{id}/${verb}` as '/admin/products/{id}/approve'
  const { error: err } = await api.POST(path, { params: { path: { id: p.id } } })
  busy.value = null
  if (err) {
    toast.add({ severity: 'error', summary: `${verb} failed`, detail: errMessage(err), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: `Product ${verb === 'approve' ? 'approved' : 'rejected'}`, life: 2000 })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Catalog moderation" :meta="items.length">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
      </template>
    </PageHeader>
    <p class="muted">Vendor-submitted listings awaiting approval. Approved products become visible and buyable on the storefront; rejected ones stay hidden.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="items" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-check-circle" title="All caught up" message="No vendor listings are waiting for moderation right now." />
      </template>
      <Column field="sku" header="SKU" />
      <Column field="name" header="Name" />
      <Column field="vendor_id" header="Vendor" />
      <Column header="" style="width: 16rem">
        <template #body="{ data }">
          <div class="rowact">
            <Button label="Approve" icon="pi pi-check" size="small" :loading="busy === data.id" @click="moderate(data, 'approve')" />
            <Button label="Reject" icon="pi pi-times" size="small" severity="danger" outlined :loading="busy === data.id" @click="moderate(data, 'reject')" />
          </div>
        </template>
      </Column>
    </DataTable>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.rowact { display: flex; gap: 0.5rem; }
</style>
