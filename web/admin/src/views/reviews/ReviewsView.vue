<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Rating from 'primevue/rating'
import SelectButton from 'primevue/selectbutton'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Review = components['schemas']['AdminReview']
type Status = 'pending' | 'approved' | 'rejected'

const toast = useToast()
const rows = ref<Review[]>([])
const loading = ref(false)
const error = ref('')
const status = ref<Status>('pending')
const statusOptions = [
  { label: 'Pending', value: 'pending' },
  { label: 'Approved', value: 'approved' },
  { label: 'Rejected', value: 'rejected' },
]

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: e } = await api.GET('/admin/reviews', { params: { query: { status: status.value } } })
  loading.value = false
  if (e || !data) {
    error.value = errMessage(e, 'Failed to load reviews')
    return
  }
  rows.value = data.items ?? []
}

async function moderate(r: Review, next: 'approved' | 'rejected') {
  const { error: e } = await api.PATCH('/admin/reviews/{id}', {
    params: { path: { id: r.id } },
    body: { status: next },
  })
  if (e) {
    toast.add({ severity: 'error', summary: 'Failed', detail: errMessage(e), life: 4000 })
    return
  }
  toast.add({ severity: 'success', summary: next === 'approved' ? 'Approved' : 'Rejected', life: 2000 })
  load()
}

function sev(s: string): 'success' | 'danger' | 'warn' {
  return s === 'approved' ? 'success' : s === 'rejected' ? 'danger' : 'warn'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Reviews" :meta="rows.length">
      <template #actions>
        <SelectButton
          v-model="status"
          :options="statusOptions"
          option-label="label"
          option-value="value"
          :allow-empty="false"
          @update:model-value="load"
        />
      </template>
    </PageHeader>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="rows" :loading="loading" data-key="id" striped-rows>
      <template #empty>
        <EmptyState icon="pi pi-star" title="Nothing to moderate" message="Verified-purchase buyer reviews awaiting your decision appear here." />
      </template>
      <Column header="Product"><template #body="{ data }">{{ data.product_name }}</template></Column>
      <Column header="Rating" style="width: 9rem">
        <template #body="{ data }"><Rating :model-value="data.rating" readonly /></template>
      </Column>
      <Column header="Review">
        <template #body="{ data }">
          <div class="rv-title">{{ data.title || '—' }}</div>
          <div v-if="data.body" class="rv-body">{{ data.body }}</div>
          <Tag v-if="data.verified" value="Verified purchase" severity="success" class="rv-vtag" />
        </template>
      </Column>
      <Column header="Author"><template #body="{ data }">{{ data.author }}</template></Column>
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template>
      </Column>
      <Column header="" style="width: 13rem">
        <template #body="{ data }">
          <div v-if="data.status === 'pending'" class="rv-actions">
            <Button label="Approve" icon="pi pi-check" size="small" @click="moderate(data, 'approved')" />
            <Button label="Reject" icon="pi pi-times" size="small" severity="danger" outlined @click="moderate(data, 'rejected')" />
          </div>
        </template>
      </Column>
    </DataTable>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.rv-title { font-weight: 600; }
.rv-body { color: var(--p-text-muted-color, #64748b); font-size: 0.9rem; max-width: 28rem; }
.rv-vtag { margin-top: 0.3rem; }
.rv-actions { display: flex; gap: 0.4rem; }
</style>
