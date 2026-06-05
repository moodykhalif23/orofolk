<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useToast } from 'primevue/usetoast'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useCustomerOptions } from '@/composables/useRecordOptions'
import type { components } from '@teggo/api/schema'

type AdminProduct = components['schemas']['AdminProduct']
type ProductInput = components['schemas']['ProductInput']
type VisRule = components['schemas']['CatalogVisibilityRule']

const props = defineProps<{ open: boolean; product: AdminProduct | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; saved: [] }>()

const toast = useToast()
const saving = ref(false)
const error = ref('')
const attrsText = ref('{}')

// Per-customer catalog visibility (only for an existing product).
const { customers, customersLoaded, loadCustomers } = useCustomerOptions()
const visRules = ref<VisRule[]>([])
const visCustomer = ref<number | null>(null)
const custName = (id?: number | null) => (id ? (customers.value.find((c) => c.id === id)?.name ?? `#${id}`) : '—')

async function loadVisibility() {
  if (!props.product) return
  const { data } = await api.GET('/admin/products/{id}/visibility', { params: { path: { id: props.product.id } } })
  visRules.value = data?.items ?? []
}
async function addVisibility() {
  if (!props.product || !visCustomer.value) return
  const { error: err } = await api.POST('/admin/products/{id}/visibility', {
    params: { path: { id: props.product.id } },
    body: { customer_id: visCustomer.value, visible: false },
  })
  if (err) {
    toast.add({ severity: 'error', summary: 'Could not add rule', detail: errMessage(err), life: 3500 })
    return
  }
  visCustomer.value = null
  loadVisibility()
}
async function removeVisibility(id: number) {
  const { error: err } = await api.DELETE('/admin/catalog-visibility/{id}', { params: { path: { id } } })
  if (!err) loadVisibility()
}

const types = ['simple', 'configurable', 'kit', 'digital']
const statuses = ['draft', 'active', 'disabled']

const form = reactive<ProductInput>({
  sku: '',
  name: '',
  slug: '',
  type: 'simple',
  status: 'draft',
  unit: 'each',
  description: '',
})

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    error.value = ''
    if (props.product) {
      Object.assign(form, {
        sku: props.product.sku,
        name: props.product.name,
        slug: props.product.slug,
        type: props.product.type,
        status: props.product.status,
        unit: props.product.unit,
        description: props.product.description ?? '',
      })
      attrsText.value = JSON.stringify(props.product.attributes ?? {}, null, 2)
      loadCustomers()
      loadVisibility()
    } else {
      visRules.value = []
      Object.assign(form, { sku: '', name: '', slug: '', type: 'simple', status: 'draft', unit: 'each', description: '' })
      attrsText.value = '{}'
    }
  },
)

function close() {
  emit('update:open', false)
}

async function save() {
  error.value = ''
  let attributes: Record<string, unknown> = {}
  try {
    attributes = attrsText.value.trim() ? JSON.parse(attrsText.value) : {}
  } catch {
    error.value = 'Attributes must be valid JSON'
    return
  }
  const body: ProductInput = { ...form, attributes }
  saving.value = true
  const res = props.product
    ? await api.PUT('/admin/products/{id}', { params: { path: { id: props.product.id } }, body })
    : await api.POST('/admin/products', { body })
  saving.value = false
  if (res.error || !res.data) {
    error.value = errMessage(res.error, 'Save failed')
    return
  }
  toast.add({ severity: 'success', summary: props.product ? 'Updated' : 'Created', detail: res.data.name, life: 2500 })
  emit('saved')
}
</script>

<template>
  <Dialog
    :visible="open"
    :header="product ? 'Edit product' : 'New product'"
    modal
    :style="{ width: '560px' }"
    @update:visible="emit('update:open', $event)"
  >
    <form class="form" @submit.prevent="save">
      <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>
      <div class="grid2">
        <div class="field">
          <label>SKU</label>
          <InputText v-model="form.sku" fluid />
        </div>
        <div class="field">
          <label>Slug</label>
          <InputText v-model="form.slug" fluid />
        </div>
      </div>
      <div class="field">
        <label>Name</label>
        <InputText v-model="form.name" fluid />
      </div>
      <div class="grid2">
        <div class="field">
          <label>Type</label>
          <Select v-model="form.type" :options="types" fluid />
        </div>
        <div class="field">
          <label>Status</label>
          <Select v-model="form.status" :options="statuses" fluid />
        </div>
      </div>
      <div class="field">
        <label>Unit</label>
        <InputText v-model="form.unit" fluid />
      </div>
      <div class="field">
        <label>Description</label>
        <Textarea v-model="form.description" rows="2" fluid />
      </div>
      <div class="field">
        <label>Attributes (JSON)</label>
        <Textarea v-model="attrsText" rows="4" fluid class="mono" />
      </div>

      <div v-if="product" class="field vis">
        <label>Catalog visibility <span class="hint">(hide this product from specific customers)</span></label>
        <ul class="vis-list">
          <li v-for="r in visRules" :key="r.id">
            <span>{{ r.visible ? 'Visible to' : 'Hidden from' }} {{ r.customer_id ? custName(r.customer_id) : (r.customer_group_id ? `group #${r.customer_group_id}` : '—') }}</span>
            <Button icon="pi pi-times" text rounded size="small" severity="danger" @click="removeVisibility(r.id)" />
          </li>
          <li v-if="!visRules.length" class="muted">Visible to everyone.</li>
        </ul>
        <div class="vis-add">
          <Select
            v-model="visCustomer"
            :options="customers"
            optionLabel="name"
            optionValue="id"
            filter
            placeholder="Hide from customer…"
            :emptyMessage="customersLoaded ? 'No customers' : 'Loading…'"
            class="grow"
          />
          <Button label="Hide" icon="pi pi-eye-slash" size="small" :disabled="!visCustomer" @click="addVisibility" />
        </div>
      </div>
    </form>
    <template #footer>
      <Button label="Cancel" severity="secondary" text @click="close" />
      <Button label="Save" :loading="saving" @click="save" />
    </template>
  </Dialog>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}
.grid2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.9rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}
.field label {
  font-size: 0.8rem;
  font-weight: 600;
}
.mono :deep(textarea) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.85rem;
}
.vis { border-top: 1px solid var(--p-surface-200, #e2e8f0); padding-top: 0.8rem; }
.hint { font-weight: 400; color: var(--p-text-muted-color, #64748b); font-size: 0.75rem; }
.vis-list { list-style: none; margin: 0 0 0.5rem; padding: 0; display: flex; flex-direction: column; gap: 0.25rem; }
.vis-list li { display: flex; align-items: center; justify-content: space-between; font-size: 0.85rem; }
.vis-list li.muted { color: var(--p-text-muted-color, #64748b); }
.vis-add { display: flex; gap: 0.5rem; align-items: center; }
.grow { flex: 1; }
</style>
