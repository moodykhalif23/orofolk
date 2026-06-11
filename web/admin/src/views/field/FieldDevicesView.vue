<script setup lang="ts">
import { onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type FieldDevice = components['schemas']['FieldDevice']

const devices = ref<FieldDevice[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/field/devices')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load devices')
    return
  }
  devices.value = data.items ?? []
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Field devices" meta="offline sync">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
      </template>
    </PageHeader>
    <p class="muted">Sales-rep devices that sync a scoped local subset (their customers, catalog, pricing, and own drafts) via the cursor-based delta protocol.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="devices" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-mobile" title="No devices have synced" message="Sales-rep devices appear here once they sync their scoped offline data set." />
      </template>
      <Column field="user_email" header="Rep" />
      <Column field="device_uuid" header="Device" />
      <Column field="platform" header="Platform" />
      <Column field="last_sync_cursor" header="Cursor" />
      <Column field="last_seen_at" header="Last seen" />
    </DataTable>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
</style>
