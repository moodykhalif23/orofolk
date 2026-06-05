<script setup lang="ts">
import Button from 'primevue/button'
import Textarea from 'primevue/textarea'
import Message from 'primevue/message'

definePageMeta({ middleware: 'auth' })
useSeoMeta({ title: 'Quick order — Teggo Store' })

const client = useClient()
const router = useRouter()

// Buyers paste one "SKU, quantity" per line (quantity optional, defaults to 1).
const raw = ref('')
const busy = ref(false)
const error = ref('')
const result = ref<{ added: number; not_found_skus: string[]; price_on_request: string[] } | null>(null)

function parseLines() {
  return raw.value
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .map((l) => {
      const parts = l.split(/[,\t;]+/).map((s) => s.trim())
      const sku = parts[0] ?? ''
      const qty = parts[1]
      return { sku, quantity: qty && qty !== '' ? qty : '1' }
    })
    .filter((l) => l.sku !== '')
}

async function submit() {
  error.value = ''
  result.value = null
  const lines = parseLines()
  if (!lines.length) {
    error.value = 'Enter at least one SKU.'
    return
  }
  busy.value = true
  const { data, error: err } = await client.POST('/storefront/cart/bulk', { body: { lines } })
  busy.value = false
  if (err || !data) {
    error.value = 'Could not process your quick order.'
    return
  }
  result.value = {
    added: data.added,
    not_found_skus: data.not_found_skus ?? [],
    price_on_request: data.price_on_request ?? [],
  }
}
</script>

<template>
  <section class="wrap">
    <h1 class="title">Quick order</h1>
    <p class="muted">Paste one SKU per line. Add a quantity after a comma — for example <code>WIDGET-01, 10</code>. Blank quantities default to 1.</p>

    <Message v-if="error" severity="error" :closable="false" class="mb">{{ error }}</Message>

    <Textarea v-model="raw" rows="10" class="box" placeholder="WIDGET-01, 10&#10;BOLT-22, 250&#10;CABLE-3M" :disabled="busy" />

    <div class="actions">
      <Button label="Add to cart" icon="pi pi-cart-plus" :loading="busy" @click="submit" />
    </div>

    <template v-if="result">
      <Message severity="success" :closable="false" class="mb">
        {{ result.added }} item{{ result.added === 1 ? '' : 's' }} added to your cart.
      </Message>
      <Message v-if="result.not_found_skus.length" severity="warn" :closable="false" class="mb">
        Unknown SKUs (skipped): {{ result.not_found_skus.join(', ') }}
      </Message>
      <Message v-if="result.price_on_request.length" severity="warn" :closable="false" class="mb">
        Price on request (skipped — request a quote): {{ result.price_on_request.join(', ') }}
      </Message>
      <Button v-if="result.added" label="Go to cart" icon="pi pi-arrow-right" iconPos="right" outlined @click="router.push('/cart')" />
    </template>
  </section>
</template>

<style scoped>
.wrap { max-width: 720px; }
.title { margin: 0 0 0.5rem; }
.muted { color: var(--p-text-muted-color, #64748b); margin-bottom: 1rem; }
.muted code { background: var(--p-surface-100, #f1f5f9); padding: 0.1rem 0.35rem; border-radius: 4px; }
.box { width: 100%; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.actions { margin: 1rem 0 1.5rem; }
.mb { margin-bottom: 1rem; }
</style>
