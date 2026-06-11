<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Category = components['schemas']['Category']

const rows = ref<Category[]>([])
const loading = ref(false)
const error = ref('')
const dialogOpen = ref(false)
const saving = ref(false)
const toast = useToast()

const form = reactive({ name: '', slug: '', sort_order: 0 })

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/categories')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load categories')
    return
  }
  rows.value = data.items ?? []
}

function openCreate() {
  Object.assign(form, { name: '', slug: '', sort_order: 0 })
  dialogOpen.value = true
}

async function save() {
  saving.value = true
  const { error: err } = await api.POST('/admin/categories', { body: { ...form } })
  saving.value = false
  if (err) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  dialogOpen.value = false
  toast.add({ severity: 'success', summary: 'Category created', life: 2000 })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Categories">
      <template #actions>
        <Button icon="pi pi-plus" label="New category" @click="openCreate" />
      </template>
    </PageHeader>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>
    <DataTable :value="rows" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-sitemap" title="No categories yet" message="Organize your catalog into categories so buyers can browse and filter products.">
          <Button icon="pi pi-plus" label="New category" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="id" header="ID" style="width: 5rem" />
      <Column field="name" header="Name" sortable />
      <Column field="slug" header="Slug" />
      <Column field="parent_id" header="Parent" />
      <Column field="sort_order" header="Sort" />
    </DataTable>

    <Dialog v-model:visible="dialogOpen" header="New category" modal :style="{ width: '440px' }">
      <form class="form" @submit.prevent="save">
        <div class="field"><label>Name</label><InputText v-model="form.name" fluid /></div>
        <div class="field"><label>Slug</label><InputText v-model="form.slug" fluid /></div>
        <div class="field"><label>Sort order</label><InputNumber v-model="form.sort_order" fluid /></div>
      </form>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
</style>
