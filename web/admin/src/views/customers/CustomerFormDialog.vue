<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useToast } from 'primevue/usetoast'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type Customer = components['schemas']['Customer']
type CustomerInput = components['schemas']['CustomerInput']
type CustomerGroup = components['schemas']['CustomerGroup']

const props = defineProps<{ open: boolean; customer: Customer | null }>()
const emit = defineEmits<{ 'update:open': [boolean]; saved: [] }>()

const toast = useToast()
const saving = ref(false)
const error = ref('')
const groups = ref<CustomerGroup[]>([])

interface FormState {
  name: string
  tax_id: string
  payment_terms_days: number
  credit_limit: string
  customer_group_id: number | null
}
const form = reactive<FormState>({
  name: '',
  tax_id: '',
  payment_terms_days: 0,
  credit_limit: '0',
  customer_group_id: null,
})

async function loadGroups() {
  const { data } = await api.GET('/admin/customer-groups')
  groups.value = data?.items ?? []
}

watch(
  () => props.open,
  (isOpen) => {
    if (!isOpen) return
    error.value = ''
    loadGroups()
    if (props.customer) {
      Object.assign(form, {
        name: props.customer.name,
        tax_id: props.customer.tax_id ?? '',
        payment_terms_days: props.customer.payment_terms_days,
        credit_limit: props.customer.credit_limit,
        customer_group_id: props.customer.customer_group_id ?? null,
      })
    } else {
      Object.assign(form, { name: '', tax_id: '', payment_terms_days: 0, credit_limit: '0', customer_group_id: null })
    }
  },
)

async function save() {
  error.value = ''
  if (!form.name.trim()) {
    error.value = 'Name is required'
    return
  }
  const body: CustomerInput = {
    name: form.name,
    tax_id: form.tax_id || null,
    payment_terms_days: form.payment_terms_days,
    credit_limit: form.credit_limit || '0',
    customer_group_id: form.customer_group_id,
  }
  saving.value = true
  const res = props.customer
    ? await api.PUT('/admin/customers/{id}', { params: { path: { id: props.customer.id } }, body })
    : await api.POST('/admin/customers', { body })
  saving.value = false
  if (res.error || !res.data) {
    error.value = errMessage(res.error, 'Save failed')
    return
  }
  toast.add({ severity: 'success', summary: props.customer ? 'Updated' : 'Created', detail: res.data.name, life: 2500 })
  emit('saved')
}
</script>

<template>
  <Dialog
    :visible="open"
    :header="customer ? 'Edit customer' : 'New customer'"
    modal
    :style="{ width: '520px' }"
    @update:visible="emit('update:open', $event)"
  >
    <form class="form" @submit.prevent="save">
      <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>
      <div class="field">
        <label>Name</label>
        <InputText v-model="form.name" fluid />
      </div>
      <div class="grid2">
        <div class="field">
          <label>Tax ID</label>
          <InputText v-model="form.tax_id" fluid />
        </div>
        <div class="field">
          <label>Group</label>
          <Select
            v-model="form.customer_group_id"
            :options="groups"
            optionLabel="name"
            optionValue="id"
            placeholder="— none —"
            showClear
            fluid
          />
        </div>
      </div>
      <div class="grid2">
        <div class="field">
          <label>Payment terms (days)</label>
          <InputNumber v-model="form.payment_terms_days" :min="0" fluid />
        </div>
        <div class="field">
          <label>Credit limit</label>
          <InputText v-model="form.credit_limit" fluid />
        </div>
      </div>
    </form>
    <template #footer>
      <Button label="Cancel" severity="secondary" text @click="emit('update:open', false)" />
      <Button label="Save" :loading="saving" @click="save" />
    </template>
  </Dialog>
</template>

<style scoped>
.form { display: flex; flex-direction: column; gap: 0.9rem; }
.grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 0.9rem; }
.field { display: flex; flex-direction: column; gap: 0.3rem; }
.field label { font-size: 0.8rem; font-weight: 600; }
</style>
