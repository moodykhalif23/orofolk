<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useToast } from 'primevue/usetoast'
import Card from 'primevue/card'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import { api, errMessage } from '@/lib/client'
import { useProductOptions } from '@/composables/useRecordOptions'
import type { components } from '@teggo/api/schema'

type Quote = components['schemas']['QuoteDetail']
type CpqDefinition = components['schemas']['CpqDefinition']
type CpqGroup = components['schemas']['CpqGroup']

interface Line {
  product_id: number
  quantity: string
  unit: string
  unit_price: string
  discount: string
}

const route = useRoute()
const router = useRouter()
const toast = useToast()
const id = Number(route.params.id)

const { productOptions, productsLoaded, loadProducts } = useProductOptions()

const quote = ref<Quote | null>(null)
const lines = ref<Line[]>([])
const error = ref('')
const saving = ref(false)
const sending = ref(false)

const isFinal = computed(() => {
  const s = quote.value?.status
  return s === 'accepted' || s === 'declined' || s === 'expired'
})

function num(s: string) {
  const n = Number(s)
  return Number.isFinite(n) ? n : 0
}
function rowTotal(l: Line) {
  return (num(l.quantity) * num(l.unit_price) - num(l.discount)).toFixed(4)
}
const previewSubtotal = computed(() => lines.value.reduce((acc, l) => acc + Number(rowTotal(l)), 0).toFixed(4))

async function load() {
  error.value = ''
  const { data, error: err } = await api.GET('/admin/quotes/{id}', { params: { path: { id } } })
  if (err || !data) {
    error.value = errMessage(err, 'Quote not found')
    return
  }
  quote.value = data
  lines.value = data.items.map((i) => ({
    product_id: i.product_id,
    quantity: i.quantity,
    unit: i.unit,
    unit_price: i.unit_price,
    discount: i.discount,
  }))
}

function addLine() {
  lines.value.push({ product_id: 0, quantity: '1', unit: 'each', unit_price: '0', discount: '0' })
}
function removeLine(idx: number) {
  lines.value.splice(idx, 1)
}

async function saveLines() {
  saving.value = true
  const { data, error: err } = await api.PUT('/admin/quotes/{id}', {
    params: { path: { id } },
    body: { items: lines.value.map((l) => ({ ...l })) },
  })
  saving.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Save failed', detail: errMessage(err), life: 4000 })
    return
  }
  quote.value = data
  toast.add({ severity: 'success', summary: 'Lines saved', detail: `Subtotal ${data.subtotal}`, life: 2500 })
}

async function send() {
  sending.value = true
  const { data, error: err } = await api.POST('/admin/quotes/{id}/send', { params: { path: { id } }, body: {} })
  sending.value = false
  if (err || !data) {
    toast.add({ severity: 'error', summary: 'Send failed', detail: errMessage(err), life: 4000 })
    return
  }
  quote.value = data
  toast.add({ severity: 'success', summary: `Sent (v${data.version})`, life: 2500 })
}

function sev(s?: string) {
  return s === 'sent' || s === 'revised' ? 'info' : s === 'accepted' ? 'success' : s === 'declined' || s === 'expired' ? 'danger' : 'secondary'
}

// ---- CPQ: add a configured product line ----------------------------------

const cfgOpen = ref(false)
const cfgProductId = ref<number>(0)
const cfg = ref<CpqDefinition | null>(null)
const cfgLoading = ref(false)
const cfgQty = ref('1')
const cfgError = ref('')
const cfgErrors = ref<string[]>([])
const cfgAdding = ref(false)
// Per-group selection: a single option id for max_select<=1, else an id array.
const cfgSel = ref<Record<number, number | number[]>>({})

function isMulti(g: CpqGroup) {
  return (g.max_select ?? 1) > 1
}

function openConfigurator() {
  cfgProductId.value = 0
  cfg.value = null
  cfgQty.value = '1'
  cfgError.value = ''
  cfgErrors.value = []
  cfgSel.value = {}
  cfgOpen.value = true
}

async function loadConfig() {
  cfg.value = null
  cfgError.value = ''
  cfgErrors.value = []
  cfgSel.value = {}
  if (!cfgProductId.value) return
  cfgLoading.value = true
  const { data, error: err, response } = await api.GET('/admin/products/{id}/config', {
    params: { path: { id: cfgProductId.value } },
  })
  cfgLoading.value = false
  if (err || !data) {
    cfgError.value = response?.status === 404 ? 'This product is not configurable.' : errMessage(err)
    return
  }
  cfg.value = data
  // Seed defaults so the preview/validation start from a sensible state.
  for (const g of data.groups ?? []) {
    const defaults = (g.options ?? []).filter((o) => o.is_default).map((o) => o.id!)
    cfgSel.value[g.id!] = isMulti(g) ? defaults : (defaults[0] ?? 0)
  }
}

function toggleMulti(groupId: number, optionId: number, checked: boolean) {
  const cur = (cfgSel.value[groupId] as number[]) ?? []
  cfgSel.value[groupId] = checked ? [...cur, optionId] : cur.filter((x) => x !== optionId)
}

const cfgSelections = computed<number[]>(() => {
  const out: number[] = []
  for (const v of Object.values(cfgSel.value)) {
    if (Array.isArray(v)) out.push(...v)
    else if (v) out.push(v)
  }
  return out
})

// Client-side price estimate; the server is authoritative on Add.
const cfgPreview = computed(() => {
  if (!cfg.value) return '0'
  let unit = num(cfg.value.base_price ?? '0')
  const byId = new Map<number, number>()
  for (const g of cfg.value.groups ?? []) for (const o of g.options ?? []) byId.set(o.id!, num(o.price_delta ?? '0'))
  for (const id of cfgSelections.value) unit += byId.get(id) ?? 0
  return (unit * num(cfgQty.value)).toFixed(4)
})

async function addConfigured() {
  cfgAdding.value = true
  cfgError.value = ''
  cfgErrors.value = []
  const { error: err, response } = await api.POST('/admin/quotes/{id}/configured-lines', {
    params: { path: { id } },
    body: { product_id: cfgProductId.value, quantity: cfgQty.value, selections: cfgSelections.value },
  })
  cfgAdding.value = false
  if (err) {
    // The server returns the specific validation failures (min/max/required/rules).
    const body = err as { errors?: string[]; error?: { message?: string } }
    if (response?.status === 422 && Array.isArray(body.errors)) cfgErrors.value = body.errors
    else cfgError.value = errMessage(err, 'Could not add configured line')
    return
  }
  cfgOpen.value = false
  await load() // the configured line now appears among the quote's lines
  toast.add({ severity: 'success', summary: 'Configured line added', life: 2500 })
}

onMounted(() => {
  load()
  loadProducts()
})
</script>

<template>
  <div class="page">
    <Button icon="pi pi-arrow-left" label="Quotes" text severity="secondary" @click="router.push({ name: 'quotes' })" />
    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <template v-if="quote">
      <div class="head">
        <h1>Quote #{{ quote.id }} <Tag :value="quote.status" :severity="sev(quote.status)" /> <span class="muted">v{{ quote.version }} · {{ quote.currency }}</span></h1>
        <div class="actions">
          <Button label="Save lines" icon="pi pi-save" severity="secondary" :loading="saving" :disabled="isFinal" @click="saveLines" />
          <Button label="Send" icon="pi pi-send" :loading="sending" :disabled="isFinal" @click="send" />
        </div>
      </div>

      <Message v-if="isFinal" severity="warn" :closable="false" class="mb">
        This quote is {{ quote.status }} and can no longer be edited.
      </Message>

      <Card>
        <template #title>
          <div class="linehead">
            <span>Line items</span>
            <div class="lineactions">
              <Button label="Add configured product" icon="pi pi-sliders-h" size="small" text :disabled="isFinal" @click="openConfigurator" />
              <Button label="Add line" icon="pi pi-plus" size="small" text :disabled="isFinal" @click="addLine" />
            </div>
          </div>
        </template>
        <template #content>
          <div class="table-scroll">
          <table class="lines">
            <thead>
              <tr><th>Product</th><th>Qty</th><th>Unit</th><th>Unit price</th><th>Discount</th><th class="r">Row total</th><th></th></tr>
            </thead>
            <tbody>
              <tr v-for="(l, idx) in lines" :key="idx">
                <td>
                  <Select
                    v-model="l.product_id"
                    :options="productOptions"
                    optionLabel="label"
                    optionValue="id"
                    filter
                    filterPlaceholder="Search products…"
                    placeholder="Select a product"
                    :emptyMessage="productsLoaded ? 'No products' : 'Loading…'"
                    :disabled="isFinal"
                    class="prodsel"
                  />
                </td>
                <td><InputText v-model="l.quantity" :disabled="isFinal" class="sm" /></td>
                <td><InputText v-model="l.unit" :disabled="isFinal" class="sm" /></td>
                <td><InputText v-model="l.unit_price" :disabled="isFinal" class="sm" /></td>
                <td><InputText v-model="l.discount" :disabled="isFinal" class="sm" /></td>
                <td class="r tabular">{{ rowTotal(l) }}</td>
                <td><Button icon="pi pi-times" text rounded severity="danger" :disabled="isFinal" @click="removeLine(idx)" /></td>
              </tr>
              <tr v-if="!lines.length"><td colspan="7" class="empty">No lines — add one.</td></tr>
            </tbody>
          </table>
          </div>

          <div class="totals">
            <span class="muted">Preview subtotal (saved server-side on Save):</span>
            <strong class="tabular">{{ previewSubtotal }}</strong>
            <span class="muted">· stored: {{ quote.subtotal }}</span>
          </div>
        </template>
      </Card>
    </template>

    <Dialog v-model:visible="cfgOpen" modal header="Add a configured product" :style="{ width: '34rem' }">
      <div class="field">
        <label>Product</label>
        <Select
          v-model="cfgProductId"
          :options="productOptions"
          optionLabel="label"
          optionValue="id"
          filter
          placeholder="Select a configurable product"
          @change="loadConfig"
        />
      </div>

      <Message v-if="cfgError" severity="warn" :closable="false" class="mb">{{ cfgError }}</Message>
      <div v-if="cfgLoading" class="muted">Loading configuration…</div>

      <template v-if="cfg">
        <div v-for="g in cfg.groups ?? []" :key="g.id" class="group">
          <div class="glabel">
            {{ g.name }}
            <span v-if="g.required" class="req">required</span>
            <span class="muted sub">{{ isMulti(g) ? `choose ${g.min_select ?? 0}–${g.max_select}` : 'choose one' }}</span>
          </div>

          <Select
            v-if="!isMulti(g)"
            :modelValue="cfgSel[g.id!]"
            :options="(g.options ?? []).map((o) => ({ label: `${o.name} (${Number(o.price_delta) >= 0 ? '+' : ''}${o.price_delta})`, value: o.id }))"
            optionLabel="label"
            optionValue="value"
            :placeholder="g.required ? 'Select an option' : 'None'"
            :showClear="!g.required"
            class="full"
            @update:modelValue="cfgSel[g.id!] = $event"
          />
          <div v-else class="opts">
            <label v-for="o in g.options ?? []" :key="o.id" class="opt">
              <Checkbox
                :modelValue="(cfgSel[g.id!] as number[])?.includes(o.id!)"
                binary
                @update:modelValue="toggleMulti(g.id!, o.id!, $event as boolean)"
              />
              <span>{{ o.name }} <span class="muted">({{ Number(o.price_delta) >= 0 ? '+' : '' }}{{ o.price_delta }})</span></span>
            </label>
          </div>
        </div>

        <div class="field qtyrow">
          <label>Quantity</label>
          <InputText v-model="cfgQty" class="sm" />
        </div>

        <Message v-if="cfgErrors.length" severity="error" :closable="false" class="mb">
          <ul class="errs"><li v-for="(e, i) in cfgErrors" :key="i">{{ e }}</li></ul>
        </Message>

        <div class="cfgtotal">
          Estimated: <strong class="tabular">{{ cfgPreview }} {{ cfg.currency }}</strong>
          <span class="muted sub">· final price validated on Add</span>
        </div>
      </template>

      <template #footer>
        <Button label="Cancel" text :disabled="cfgAdding" @click="cfgOpen = false" />
        <Button label="Add to quote" icon="pi pi-plus" :loading="cfgAdding" :disabled="!cfg" @click="addConfigured" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.head { display: flex; align-items: center; justify-content: space-between; margin: 0.5rem 0 1rem; gap: 1rem; }
.head h1 { margin: 0; display: flex; align-items: center; gap: 0.6rem; font-size: 1.4rem; }
.actions { display: flex; gap: 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 0.95rem; }
.mb { margin-bottom: 1rem; }
.linehead { display: flex; align-items: center; justify-content: space-between; }
.lineactions { display: flex; gap: 0.25rem; }
.field { display: flex; flex-direction: column; gap: 0.35rem; margin-bottom: 0.9rem; }
.field label { font-size: 0.85rem; font-weight: 600; }
.full { width: 100%; }
.group { padding: 0.6rem 0; border-top: 1px solid var(--p-surface-200, #e2e8f0); }
.glabel { font-weight: 600; font-size: 0.9rem; margin-bottom: 0.4rem; display: flex; align-items: center; gap: 0.5rem; }
.req { font-size: 0.7rem; color: var(--p-red-500, #ef4444); border: 1px solid currentColor; border-radius: 3px; padding: 0 0.25rem; }
.sub { font-weight: 400; font-size: 0.8rem; }
.opts { display: flex; flex-direction: column; gap: 0.4rem; }
.opt { display: flex; align-items: center; gap: 0.5rem; cursor: pointer; }
.qtyrow { margin-top: 0.8rem; }
.errs { margin: 0; padding-left: 1.1rem; }
.cfgtotal { margin-top: 0.8rem; font-size: 1.05rem; }
.lines { width: 100%; min-width: 38rem; border-collapse: collapse; }
.lines th { text-align: left; font-size: 0.78rem; color: var(--p-text-muted-color, #64748b); padding: 0.3rem 0.5rem; }
.lines td { padding: 0.3rem 0.5rem; }
.lines .r { text-align: right; }
.sm :deep(input), .sm { width: 7rem; }
.prodsel { width: 16rem; }
.tabular { font-variant-numeric: tabular-nums; }
.empty { text-align: center; color: var(--p-text-muted-color, #64748b); padding: 1rem; }
.totals { display: flex; align-items: baseline; gap: 0.6rem; justify-content: flex-end; margin-top: 1rem; font-size: 1.05rem; }
</style>
