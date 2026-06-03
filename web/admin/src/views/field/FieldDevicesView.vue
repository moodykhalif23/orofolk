<script setup lang="ts">
import { onMounted, ref } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

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
    <div class="header">
      <h1>Field devices <span class="muted">offline sync</span></h1>
      <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
    </div>
    <p class="muted">Sales-rep devices that sync a scoped local subset (their customers, catalog, pricing, and own drafts) via the cursor-based delta protocol.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="devices" :loading="loading" dataKey="id" stripedRows>
      <template #empty>No devices have synced yet.</template>
      <Column field="user_email" header="Rep" />
      <Column field="device_uuid" header="Device" />
      <Column field="platform" header="Platform" />
      <Column field="last_sync_cursor" header="Cursor" />
      <Column field="last_seen_at" header="Last seen" />
    </DataTable>
  </div>
</template>

<style scoped>
.header { display: flex; align-items: center; justify-content: space-between; }
.header h1 { margin: 0; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; }
.mb { margin-bottom: 1rem; }
</style>
