<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type CustomerGroup = components['schemas']['CustomerGroup']

const rows = ref<CustomerGroup[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const name = ref('')
const toast = useToast()

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/customer-groups')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load groups')
    return
  }
  rows.value = data.items ?? []
}

function openCreate() {
  name.value = ''
  dialogOpen.value = true
}

async function save() {
  saving.value = true
  const { error: err } = await api.POST('/admin/customer-groups', { body: { name: name.value } })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  dialogOpen.value = false
  toast.add({ severity: 'success', summary: 'Group created', life: 2000 })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="header">
      <h1>Customer groups</h1>
      <Button icon="pi pi-plus" label="New group" @click="openCreate" />
    </div>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <DataTable :value="rows" :loading="loading" dataKey="id" stripedRows>
      <template #empty>No groups yet.</template>
      <Column field="id" header="ID" style="width: 5rem" />
      <Column field="name" header="Name" sortable />
    </DataTable>

    <Dialog v-model:visible="dialogOpen" header="New customer group" modal :style="{ width: '400px' }">
      <div class="field"><label>Name</label><InputText v-model="name" fluid /></div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1rem; }
.header h1 { margin: 0; }
.mb { margin-bottom: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
</style>
