<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'

type Rule = components['schemas']['PriceAdjustmentRule']

const rules = ref<Rule[]>([])
const groups = ref<{ id: number; name: string }[]>([])
const loading = ref(false)
const error = ref('')
const toast = useToast()

const dialogOpen = ref(false)
const saving = ref(false)
const formError = ref('')
const form = reactive({
  name: '',
  customer_group_id: null as number | null,
  attribute_key: '',
  attribute_value: '',
  adjustment_type: 'percent',
  adjustment_value: '',
  priority: 0,
})
const types = [
  { label: 'Percent (e.g. -10 = −10%)', value: 'percent' },
  { label: 'Amount (signed currency delta)', value: 'amount' },
]

async function load() {
  loading.value = true
  error.value = ''
  const { data, error: err } = await api.GET('/admin/price-adjustment-rules')
  const { data: gdata } = await api.GET('/admin/customer-groups')
  loading.value = false
  if (err || !data) {
    error.value = errMessage(err, 'Failed to load rules')
    return
  }
  rules.value = data.items ?? []
  groups.value = (gdata?.items ?? []).map((x) => ({ id: x.id, name: x.name }))
}

function groupName(id?: number | null) {
  return id ? (groups.value.find((g) => g.id === id)?.name ?? `#${id}`) : 'All buyers'
}
function scope(r: Rule) {
  return r.attribute_key ? `${r.attribute_key}=${r.attribute_value}` : 'All products'
}
function effect(r: Rule) {
  return r.adjustment_type === 'percent' ? `${r.adjustment_value}%` : `${r.adjustment_value}`
}

function openCreate() {
  Object.assign(form, { name: '', customer_group_id: null, attribute_key: '', attribute_value: '', adjustment_type: 'percent', adjustment_value: '', priority: 0 })
  formError.value = ''
  dialogOpen.value = true
}

async function save() {
  if (!form.name || form.adjustment_value === '') {
    formError.value = 'Name and adjustment value are required.'
    return
  }
  const hasKey = form.attribute_key.trim() !== ''
  const hasVal = form.attribute_value.trim() !== ''
  if (hasKey !== hasVal) {
    formError.value = 'Attribute key and value must be set together (or both blank).'
    return
  }
  saving.value = true
  const { error: err } = await api.POST('/admin/price-adjustment-rules', {
    body: {
      name: form.name,
      customer_group_id: form.customer_group_id,
      attribute_key: hasKey ? form.attribute_key.trim() : null,
      attribute_value: hasVal ? form.attribute_value.trim() : null,
      adjustment_type: form.adjustment_type as Rule['adjustment_type'],
      adjustment_value: String(form.adjustment_value),
      priority: form.priority,
      is_active: true,
    },
  })
  saving.value = false
  if (err) {
    formError.value = errMessage(err, 'Save failed')
    return
  }
  toast.add({ severity: 'success', summary: 'Rule created', life: 2500 })
  dialogOpen.value = false
  load()
}

async function remove(r: Rule) {
  const { error: err } = await api.DELETE('/admin/price-adjustment-rules/{id}', { params: { path: { id: r.id } } })
  if (err) {
    toast.add({ severity: 'error', summary: 'Delete failed', detail: errMessage(err), life: 4000 })
    return
  }
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <PageHeader title="Price rules" :meta="rules.length">
      <template #actions>
        <Button icon="pi pi-refresh" severity="secondary" text @click="load" />
        <Button icon="pi pi-plus" label="New rule" @click="openCreate" />
      </template>
    </PageHeader>
    <p class="muted">Adjustment rules tweak the resolved price by a percent or fixed amount, scoped by customer group and/or a product attribute. The highest-priority matching rule applies; with none, prices are unchanged.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <DataTable :value="rules" :loading="loading" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-sliders-h" title="No price rules" message="Without rules, prices follow your price lists as-is. Add a rule for volume breaks or customer-specific discounts.">
          <Button icon="pi pi-plus" label="New rule" @click="openCreate" />
        </EmptyState>
      </template>
      <Column field="name" header="Name" />
      <Column header="Applies to"><template #body="{ data }">{{ groupName(data.customer_group_id) }}</template></Column>
      <Column header="When"><template #body="{ data }">{{ scope(data) }}</template></Column>
      <Column header="Adjustment"><template #body="{ data }"><Tag :value="effect(data)" :severity="String(data.adjustment_value).startsWith('-') ? 'success' : 'warn'" /></template></Column>
      <Column field="priority" header="Priority" />
      <Column header="" style="width: 5rem">
        <template #body="{ data }"><Button icon="pi pi-trash" text rounded severity="danger" @click="remove(data)" /></template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="dialogOpen" modal header="New price rule" :style="{ width: '34rem' }">
      <Message v-if="formError" severity="error" :closable="false" class="mb">{{ formError }}</Message>
      <div class="field"><label>Name</label><InputText v-model="form.name" /></div>
      <div class="field">
        <label>Customer group <span class="muted">(blank = all buyers)</span></label>
        <Select v-model="form.customer_group_id" :options="groups" optionLabel="name" optionValue="id" placeholder="All buyers" showClear />
      </div>
      <div class="grid2">
        <div class="field"><label>Attribute key <span class="muted">(optional)</span></label><InputText v-model="form.attribute_key" placeholder="e.g. brand" /></div>
        <div class="field"><label>Attribute value</label><InputText v-model="form.attribute_value" placeholder="e.g. acme" /></div>
      </div>
      <div class="grid2">
        <div class="field"><label>Type</label><Select v-model="form.adjustment_type" :options="types" optionLabel="label" optionValue="value" /></div>
        <div class="field"><label>Value</label><InputText v-model="form.adjustment_value" placeholder="-10" /></div>
      </div>
      <div class="field"><label>Priority <span class="muted">(higher wins)</span></label><InputNumber v-model="form.priority" :min="0" showButtons /></div>
      <template #footer>
        <Button label="Cancel" severity="secondary" text @click="dialogOpen = false" />
        <Button label="Save" :loading="saving" @click="save" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.mb { margin-bottom: 1rem; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 1rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
</style>
