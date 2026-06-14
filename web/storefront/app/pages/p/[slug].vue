<script setup lang="ts">
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Message from 'primevue/message'

const route = useRoute()
const router = useRouter()
const client = useClient()
const { isAuthenticated } = useAuth()
const locale = useLocale()

const { data: product, error } = await useAsyncData(
  () => `product-${route.params.slug}-${locale.value}`,
  async () => {
    const { data, error } = await client.GET('/storefront/products/{slug}', {
      params: {
        path: { slug: route.params.slug as string },
        query: locale.value ? { locale: locale.value } : {},
      },
    })
    if (error) throw createError({ statusCode: 404, statusMessage: 'Product not found' })
    return data
  },
  { watch: [locale] },
)

import type { components } from '@teggo/api/schema'
type Pricing = components['schemas']['ProductPricing']

const adding = ref(false)
const feedback = ref<{ severity: 'success' | 'warn' | 'error'; text: string } | null>(null)

// Contract pricing is buyer-specific, so it's only fetched for an authenticated
// session (client-side, where the session cookie is present).
const pricing = ref<Pricing | null>(null)
async function loadPricing() {
  if (!isAuthenticated.value) return
  const { data } = await client.GET('/storefront/products/{slug}/pricing', {
    params: { path: { slug: route.params.slug as string } },
  })
  pricing.value = data ?? null
}

type Availability = components['schemas']['WarehouseAvailability']
const availability = ref<Availability | null>(null)
async function loadAvailability() {
  const { data } = await client.GET('/storefront/products/{slug}/availability', {
    params: { path: { slug: route.params.slug as string } },
  })
  availability.value = data ?? null
}

// ---- Reviews ----
type ReviewItem = components['schemas']['ReviewItem']
const reviews = ref<{ average: string; total: number; items: ReviewItem[] } | null>(null)
async function loadReviews() {
  const { data } = await client.GET('/storefront/products/{slug}/reviews', {
    params: { path: { slug: route.params.slug as string } },
  })
  reviews.value = data ?? null
}

const reviewForm = reactive({ rating: 0, title: '', body: '' })
const reviewMsg = ref<{ severity: 'success' | 'warn' | 'error'; text: string } | null>(null)
const submittingReview = ref(false)
async function submitReview() {
  if (reviewForm.rating < 1) {
    reviewMsg.value = { severity: 'warn', text: 'Please choose a star rating.' }
    return
  }
  submittingReview.value = true
  const { error: err, response } = await client.POST('/storefront/products/{slug}/reviews', {
    params: { path: { slug: route.params.slug as string } },
    body: { rating: reviewForm.rating, title: reviewForm.title.trim(), body: reviewForm.body.trim() },
  })
  submittingReview.value = false
  if (!err) {
    reviewMsg.value = { severity: 'success', text: 'Thanks — your review is pending approval.' }
    reviewForm.rating = 0
    reviewForm.title = ''
    reviewForm.body = ''
    return
  }
  reviewMsg.value =
    response?.status === 403
      ? { severity: 'warn', text: 'You can review only products your company has received.' }
      : response?.status === 401
        ? { severity: 'warn', text: 'Please sign in to write a review.' }
        : { severity: 'error', text: 'Could not submit your review.' }
}

onMounted(() => {
  loadPricing()
  loadAvailability()
  loadReviews()
})

async function addToCart() {
  if (!product.value) return
  if (!isAuthenticated.value) {
    router.push({ path: '/login', query: { redirect: route.fullPath } })
    return
  }
  feedback.value = null
  adding.value = true
  const { error: err, response } = await client.POST('/storefront/cart/items', {
    body: { product_public_id: product.value.public_id, quantity: '1' },
  })
  adding.value = false
  if (!err) {
    feedback.value = { severity: 'success', text: 'Added to your cart.' }
    return
  }
  feedback.value =
    response?.status === 409
      ? { severity: 'warn', text: 'No price available — request a quote for this product.' }
      : { severity: 'error', text: 'Could not add to cart.' }
}

// Product attributes → a "Specifications" accordion (key/value pairs).
const specs = computed(() => {
  const a = (product.value?.attributes ?? {}) as Record<string, unknown>
  return Object.entries(a).filter(([, v]) => v !== null && v !== undefined && v !== '')
})

useSeoMeta({
  title: () => (product.value ? `${product.value.name} — Teggo Store` : 'Product'),
  description: () => product.value?.description ?? 'Product detail',
})
</script>

<template>
  <section>
    <StoreBreadcrumb :items="[{ label: 'Catalog', to: '/c/all' }, { label: product?.name ?? 'Product' }]" />
    <Message v-if="error" severity="error" :closable="false">
      Product not found, or the API is unavailable.
    </Message>

    <article v-else-if="product" class="detail">
      <div class="gallery">
        <Galleria
          v-if="product.images?.length"
          :value="product.images"
          :numVisible="4"
          :showThumbnails="product.images.length > 1"
          :showItemNavigators="product.images.length > 1"
          :showIndicators="false"
          container-class="pdp-gallery"
        >
          <template #item="{ item }">
            <Image :src="item.url" :alt="item.alt ?? product.name" preview image-class="gal-main" />
          </template>
          <template #thumbnail="{ item }">
            <img :src="item.url" :alt="item.alt ?? ''" class="gal-thumb" />
          </template>
        </Galleria>
        <div v-else class="placeholder"><i class="pi pi-image" /></div>
      </div>
      <div class="info">
        <span class="sku">{{ product.sku }}</span>
        <h1>{{ product.name }}</h1>
        <p v-if="product.sold_by" class="soldby">Sold by <strong>{{ product.sold_by }}</strong></p>
        <Tag :value="product.status" severity="secondary" />
        <div v-if="reviews && reviews.total > 0" class="rating-summary">
          <Rating :model-value="Number(reviews.average)" readonly />
          <span class="rating-num">{{ reviews.average }}</span>
          <a href="#reviews" class="rating-count">{{ reviews.total }} review{{ reviews.total === 1 ? '' : 's' }}</a>
        </div>
        <p v-if="product.description" class="desc">{{ product.description }}</p>

        <Accordion v-if="specs.length" value="0" class="specs">
          <AccordionPanel value="0">
            <AccordionHeader>Specifications</AccordionHeader>
            <AccordionContent>
              <table class="spec-table">
                <tbody>
                  <tr v-for="[k, v] in specs" :key="k">
                    <th>{{ k }}</th>
                    <td>{{ v }}</td>
                  </tr>
                </tbody>
              </table>
            </AccordionContent>
          </AccordionPanel>
        </Accordion>

        <div v-if="pricing" class="pricing">
          <h3>Your pricing</h3>
          <p v-if="pricing.price_on_request" class="por">Price on request — add to a quote and our team will respond.</p>
          <table v-else class="tiers">
            <thead><tr><th>Quantity</th><th>Unit price</th></tr></thead>
            <tbody>
              <tr v-for="(t, i) in pricing.tiers" :key="i">
                <td>{{ t.min_quantity }}+ <span class="unit">/ {{ t.unit }}</span></td>
                <td>{{ t.value }} {{ pricing.currency }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="availability && availability.warehouses.length" class="avail">
          <h3>Availability</h3>
          <ul>
            <li v-for="wloc in availability.warehouses" :key="wloc.warehouse_id">
              <span>{{ wloc.warehouse_name }}</span>
              <span :class="Number(wloc.available) > 0 ? 'in' : 'out'">{{ Number(wloc.available) > 0 ? `${wloc.available} available` : 'Out of stock' }}</span>
            </li>
          </ul>
        </div>

        <Message v-if="feedback" :severity="feedback.severity" :closable="false" class="feedback">{{ feedback.text }}</Message>
        <div class="actions">
          <Button label="Add to cart" icon="pi pi-shopping-cart" :loading="adding" @click="addToCart" />
          <NuxtLink to="/rfq">
            <Button label="Request a quote" icon="pi pi-file-edit" severity="secondary" outlined />
          </NuxtLink>
        </div>
      </div>
    </article>

    <section v-if="product" id="reviews" class="reviews">
      <h2>Reviews <span v-if="reviews" class="muted">({{ reviews.total }})</span></h2>

      <div v-if="isAuthenticated" class="review-form">
        <h3>Write a review</h3>
        <Message v-if="reviewMsg" :severity="reviewMsg.severity" :closable="false" class="rf-msg">{{ reviewMsg.text }}</Message>
        <div class="rf-rate">
          <label>Your rating</label>
          <Rating v-model="reviewForm.rating" />
        </div>
        <InputText v-model="reviewForm.title" placeholder="Title (optional)" class="rf-input" />
        <Textarea v-model="reviewForm.body" rows="3" placeholder="Share your experience (optional)" class="rf-input" />
        <Button label="Submit review" icon="pi pi-send" :loading="submittingReview" class="rf-submit" @click="submitReview" />
      </div>
      <p v-else class="muted">Sign in to write a review.</p>

      <div v-if="reviews && reviews.items.length" class="review-list">
        <article v-for="rv in reviews.items" :key="rv.id" class="review">
          <div class="review-head">
            <Rating :model-value="rv.rating" readonly />
            <span class="review-author">{{ rv.author }}</span>
            <Tag v-if="rv.verified" value="Verified purchase" severity="success" />
          </div>
          <h4 v-if="rv.title" class="review-title">{{ rv.title }}</h4>
          <p v-if="rv.body" class="review-body">{{ rv.body }}</p>
        </article>
      </div>
      <p v-else class="muted">No reviews yet — be the first to review this product.</p>
    </section>
  </section>
</template>

<style scoped>
.detail {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
}
@media (max-width: 720px) {
  .detail {
    grid-template-columns: 1fr;
  }
}
.placeholder {
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 3rem;
  color: var(--p-surface-300, #cbd5e1);
  background: var(--p-surface-100, #f1f5f9);
  border-radius: 12px;
}
.gallery :deep(.pdp-gallery) { width: 100%; }
.gallery :deep(.p-image) { width: 100%; display: block; }
.gallery :deep(.gal-main) {
  width: 100%;
  height: auto;
  display: block;
  border-radius: 12px;
  background: var(--p-surface-100, #f1f5f9);
}
.gal-thumb {
  width: 56px;
  height: 56px;
  object-fit: cover;
  border-radius: 6px;
  display: block;
}
.specs { margin: 1.25rem 0; }
.spec-table { width: 100%; border-collapse: collapse; font-size: 0.9rem; }
.spec-table th {
  text-align: left;
  font-weight: 600;
  padding: 0.45rem 0.9rem 0.45rem 0;
  color: var(--p-text-muted-color, #64748b);
  white-space: nowrap;
  vertical-align: top;
  text-transform: capitalize;
}
.spec-table td { padding: 0.45rem 0; }
.spec-table tr + tr th,
.spec-table tr + tr td { border-top: 1px solid var(--p-surface-100, #f1f5f9); }
.sku {
  font-size: 0.85rem;
  color: var(--p-text-muted-color, #64748b);
}
.soldby {
  margin: 0.25rem 0 0;
  font-size: 0.9rem;
  color: var(--p-text-muted-color, #64748b);
}
.info h1 {
  margin: 0.25rem 0 0.75rem;
}
.desc {
  margin: 1rem 0;
  line-height: 1.6;
}
.pricing {
  margin: 1.25rem 0;
  padding: 1rem;
  border: 1px solid var(--p-surface-200, #e2e8f0);
  border-radius: 10px;
  background: var(--p-surface-0, #fff);
}
.pricing h3 { margin: 0 0 0.6rem; font-size: 0.95rem; }
.por { margin: 0; color: var(--p-text-muted-color, #64748b); }
.avail { margin: 1.25rem 0; }
.avail h3 { margin: 0 0 0.5rem; font-size: 0.95rem; }
.avail ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.3rem; }
.avail li { display: flex; justify-content: space-between; gap: 1rem; font-size: 0.9rem; max-width: 22rem; }
.avail .in { color: var(--p-green-600, #16a34a); font-weight: 600; }
.avail .out { color: var(--p-text-muted-color, #64748b); }
.tiers { width: 100%; border-collapse: collapse; font-variant-numeric: tabular-nums; }
.tiers th { text-align: left; font-size: 0.78rem; color: var(--p-text-muted-color, #64748b); font-weight: 600; padding-bottom: 0.35rem; }
.tiers td { padding: 0.3rem 0; border-top: 1px solid var(--p-surface-100, #f1f5f9); }
.tiers .unit { color: var(--p-text-muted-color, #64748b); font-size: 0.8rem; }
.actions {
  display: flex;
  gap: 0.75rem;
  margin-top: 1.5rem;
  flex-wrap: wrap;
}
.rating-summary { display: flex; align-items: center; gap: 0.6rem; margin: 0.5rem 0; }
.rating-num { font-weight: 700; }
.rating-count { color: var(--p-primary-color, #6366f1); text-decoration: none; font-size: 0.9rem; }
.reviews { margin-top: 2.5rem; border-top: 1px solid var(--p-surface-200, #e2e8f0); padding-top: 1.5rem; }
.reviews h2 { margin: 0 0 1rem; }
.muted { color: var(--p-text-muted-color, #64748b); }
.review-form {
  background: var(--p-surface-50, #f8fafc);
  border: 1px solid var(--p-surface-200, #e2e8f0);
  border-radius: 10px;
  padding: 1.1rem 1.25rem;
  margin-bottom: 1.5rem;
  max-width: 36rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.review-form h3 { margin: 0; font-size: 1rem; }
.rf-rate { display: flex; align-items: center; gap: 0.6rem; }
.rf-rate label { font-size: 0.9rem; font-weight: 600; }
.rf-input { width: 100%; }
.rf-submit { align-self: flex-start; }
.rf-msg { margin: 0; }
.review-list { display: flex; flex-direction: column; gap: 1.25rem; }
.review { border-bottom: 1px solid var(--p-surface-100, #f1f5f9); padding-bottom: 1rem; }
.review-head { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; }
.review-author { font-weight: 600; font-size: 0.9rem; }
.review-title { margin: 0.4rem 0 0.2rem; font-size: 1rem; }
.review-body { margin: 0; color: var(--p-text-color, #334155); line-height: 1.55; }
</style>
