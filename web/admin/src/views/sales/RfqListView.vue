<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Rfq = components['schemas']['RfqSummary']

const router = useRouter()
const rows = ref<Rfq[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/rfqs')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load RFQs')
    return
  }
  rows.value = data.items ?? []
}

function sev(s: string) {
  return s === 'submitted' ? 'info' : s === 'quoted' ? 'warn' : s === 'accepted' ? 'success' : 'secondary'
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="RFQs" />
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <DataTable
      :value="rows"
      :loading="loading"
      dataKey="id"
      stripedRows
      paginator
      :rows="15"
      @rowClick="router.push({ name: 'rfq-detail', params: { id: $event.data.id } })"
      class="clickable"
    >
      <template #empty>
        <EmptyState icon="pi pi-inbox" title="No RFQs yet" message="Requests for quote submitted by customers land here, ready for you to turn into quotes." />
      </template>
      <Column field="id" header="ID" style="width: 5rem" />
      <Column header="Reference">
        <template #body="{ data }">{{ data.public_id.slice(0, 8) }}…</template>
      </Column>
      <Column header="Status">
        <template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template>
      </Column>
      <Column field="notes" header="Notes" />
    </DataTable>
  </div>
</template>

<style scoped>
.page h1 { margin: 0 0 1rem; }
.mb { margin-bottom: 1rem; }
.clickable :deep(tbody tr) { cursor: pointer; }
</style>
