<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import type { components } from '@teggo/api/schema'

type TaxRate = components['schemas']['TaxRate']
type ShippingRate = components['schemas']['ShippingRate']

const toast = useToast()
const taxRates = ref<TaxRate[]>([])
const shipRates = ref<ShippingRate[]>([])
const error = ref('')

const taxForm = reactive({ country: '', tax_class: 'standard', rate: '', name: '' })
const shipForm = reactive({ country: '', service: '', carrier: 'local', amount: '', free_over: '' })

async function load() {
  error.value = ''
  const { data: t, error: te } = await api.GET('/admin/tax-rates')
  const { data: s } = await api.GET('/admin/shipping-rates')
  if (te) {
    error.value = errMessage(te, 'Failed to load')
    return
  }
  taxRates.value = t?.items ?? []
  shipRates.value = s?.items ?? []
}

async function addTax() {
  if (taxForm.country.length !== 2 || !taxForm.name) {
    toast.add({ severity: 'warn', summary: '2-letter country + name required', life: 3000 })
    return
  }
  const { error: err } = await api.POST('/admin/tax-rates', { body: { ...taxForm, rate: taxForm.rate || '0' } })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Save failed'), life: 4000 })
    return
  }
  taxForm.rate = ''
  taxForm.name = ''
  load()
}

async function delTax(id: number) {
  await api.DELETE('/admin/tax-rates/{id}', { params: { path: { id } } })
  load()
}

async function addShip() {
  if (shipForm.country.length !== 2 || !shipForm.service) {
    toast.add({ severity: 'warn', summary: '2-letter country + service required', life: 3000 })
    return
  }
  const body: components['schemas']['ShippingRateInput'] = {
    country: shipForm.country,
    service: shipForm.service,
    carrier: shipForm.carrier || 'local',
    amount: shipForm.amount || '0',
    is_active: true,
  }
  if (shipForm.free_over) body.free_over = shipForm.free_over
  const { error: err } = await api.POST('/admin/shipping-rates', { body })
  if (err) {
    toast.add({ severity: 'error', summary: errMessage(err, 'Save failed'), life: 4000 })
    return
  }
  shipForm.amount = ''
  shipForm.free_over = ''
  shipForm.service = ''
  load()
}

async function delShip(id: number) {
  await api.DELETE('/admin/shipping-rates/{id}', { params: { path: { id } } })
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <h1>Tax &amp; Shipping</h1>
    <p class="muted">Rules-based local providers: per-(country, tax-class) VAT and table-rate shipping (free over a threshold). Tax snapshots onto order lines at checkout; shipping rates feed the cart and shipment labels.</p>
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <div class="cols">
      <section>
        <h3>VAT rates</h3>
        <DataTable :value="taxRates" dataKey="id" stripedRows class="mb">
          <template #empty>No tax rates.</template>
          <Column field="country" header="Country" />
          <Column field="tax_class" header="Class" />
          <Column header="Rate"><template #body="{ data }">{{ (Number(data.rate) * 100).toFixed(2) }}%</template></Column>
          <Column field="name" header="Name" />
          <Column header="" style="width:3rem"><template #body="{ data }"><Button icon="pi pi-trash" text rounded severity="danger" size="small" @click="delTax(data.id)" /></template></Column>
        </DataTable>
        <div class="mrow">
          <InputText v-model="taxForm.country" placeholder="KE" maxlength="2" style="width:4rem" />
          <InputText v-model="taxForm.tax_class" placeholder="class" style="width:7rem" />
          <InputText v-model="taxForm.rate" placeholder="0.16" style="width:6rem" />
          <InputText v-model="taxForm.name" placeholder="name" />
          <Button icon="pi pi-plus" label="Add" size="small" @click="addTax" />
        </div>
        <p class="muted small">Rate is a fraction (0.16 = 16%). Use a class of <code>exempt</code> with rate 0 for exemptions.</p>
      </section>

      <section>
        <h3>Shipping rates</h3>
        <DataTable :value="shipRates" dataKey="id" stripedRows class="mb">
          <template #empty>No shipping rates.</template>
          <Column field="country" header="Country" />
          <Column field="service" header="Service" />
          <Column field="carrier" header="Carrier" />
          <Column field="amount" header="Amount" />
          <Column header="Free over"><template #body="{ data }">{{ data.free_over ?? '—' }}</template></Column>
          <Column header="" style="width:3rem"><template #body="{ data }"><Button icon="pi pi-trash" text rounded severity="danger" size="small" @click="delShip(data.id)" /></template></Column>
        </DataTable>
        <div class="mrow">
          <InputText v-model="shipForm.country" placeholder="KE" maxlength="2" style="width:4rem" />
          <InputText v-model="shipForm.service" placeholder="standard" style="width:7rem" />
          <InputText v-model="shipForm.amount" placeholder="10.00" style="width:6rem" />
          <InputText v-model="shipForm.free_over" placeholder="free over" style="width:7rem" />
          <Button icon="pi pi-plus" label="Add" size="small" @click="addShip" />
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
h1 { margin: 0 0 0.25rem; }
.muted { color: var(--p-text-muted-color, #64748b); }
.small { font-size: 0.85rem; }
.mb { margin-bottom: 0.75rem; }
.cols { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; margin-top: 1rem; }
.mrow { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; }
h3 { margin: 0 0 0.5rem; }
</style>
