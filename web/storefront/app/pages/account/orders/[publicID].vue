<script setup lang="ts">
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'
import type { components } from '@teggo/api/schema'

definePageMeta({ middleware: 'auth' })

type Order = components['schemas']['OrderDetail']

const route = useRoute()
const router = useRouter()
const client = useClient()
const publicID = route.params.publicID as string

const { data: order, error } = await useAsyncData(`order-${publicID}`, async () => {
  const { data, error } = await client.GET('/storefront/orders/{publicID}', { params: { path: { publicID } } })
  if (error || !data) throw createError({ statusCode: 404, statusMessage: 'Order not found' })
  return data
})

useSeoMeta({ title: () => (order.value ? `Order ${order.value.public_id.slice(0, 8)} — Teggo` : 'Order') })

const reordering = ref(false)
const reorderNotice = ref('')

async function reorder() {
  reordering.value = true
  reorderNotice.value = ''
  const { data, error: err } = await client.POST('/storefront/cart/reorder', {
    body: { order_public_id: publicID },
  })
  reordering.value = false
  if (err || !data) {
    reorderNotice.value = 'Could not add these items to your cart.'
    return
  }
  const skipped = data.skipped_skus ?? []
  if (skipped.length) {
    reorderNotice.value = `Added to cart. ${skipped.length} item${skipped.length > 1 ? 's' : ''} skipped (price on request): ${skipped.join(', ')}.`
  } else {
    router.push('/cart')
  }
}

function sev(s?: string) {
  if (s === 'cancelled') return 'danger'
  if (s === 'delivered' || s === 'closed') return 'success'
  if (s === 'pending' || s === 'on_hold') return 'warn'
  return 'info'
}
</script>

<template>
  <section class="wrap">
    <Button icon="pi pi-arrow-left" label="My orders" text severity="secondary" @click="router.push('/account/orders')" />
    <Message v-if="error" severity="error" :closable="false">Order not found.</Message>
    <Message v-if="reorderNotice" severity="info" :closable="true" class="mb">{{ reorderNotice }}</Message>

    <template v-if="order">
      <div class="head">
        <h1>Order <span class="muted">{{ order.public_id.slice(0, 8) }}…</span> <Tag :value="order.status" :severity="sev(order.status)" /></h1>
        <div class="actions">
          <Button label="Reorder" icon="pi pi-replay" outlined :loading="reordering" @click="reorder" />
          <div class="total">{{ order.grand_total }} {{ order.currency }}</div>
        </div>
      </div>

      <DataTable :value="order.items" dataKey="id" stripedRows>
        <template #empty>No items.</template>
        <Column field="name" header="Product" />
        <Column field="sku" header="SKU" />
        <Column field="quantity" header="Qty" />
        <Column field="unit_price" header="Unit price" />
        <Column field="row_total" header="Row total" />
      </DataTable>
    </template>
  </section>
</template>

<style scoped>
.wrap { max-width: 820px; }
.head { display: flex; align-items: center; justify-content: space-between; margin: 0.5rem 0 1rem; }
.head h1 { margin: 0; display: flex; align-items: center; gap: 0.6rem; }
.actions { display: flex; align-items: center; gap: 1.25rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-weight: 400; font-size: 1rem; }
.total { font-size: 1.3rem; font-weight: 700; }
.mb { margin-bottom: 1rem; }
</style>
