<script setup lang="ts">
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import Message from 'primevue/message'

definePageMeta({ middleware: 'auth' })
useSeoMeta({ title: 'My RFQs — Teggo Store' })

const client = useClient()
const { data, error } = await useAsyncData('my-rfqs', async () => {
  const { data, error } = await client.GET('/storefront/rfqs')
  if (error) throw createError({ statusCode: 502, statusMessage: 'Could not load RFQs' })
  return data
})

function sev(s: string) {
  return s === 'quoted' ? 'warn' : s === 'accepted' ? 'success' : s === 'submitted' ? 'info' : 'secondary'
}
</script>

<template>
  <section>
    <PageHeader title="My requests for quote">
      <template #actions>
        <NuxtLink to="/rfq"><Button label="New request" icon="pi pi-plus" /></NuxtLink>
      </template>
    </PageHeader>
    <Message v-if="error" severity="error" :closable="false">Could not load your RFQs.</Message>
    <DataTable v-else :value="data?.items ?? []" dataKey="id" stripedRows>
      <template #empty>
        <EmptyState icon="pi pi-inbox" title="No requests yet" message="Send a request for quote and we'll get back to you with pricing here.">
          <NuxtLink to="/rfq"><Button label="New request" icon="pi pi-plus" /></NuxtLink>
        </EmptyState>
      </template>
      <Column header="Reference"><template #body="{ data }">{{ data.public_id.slice(0, 8) }}…</template></Column>
      <Column header="Status"><template #body="{ data }"><Tag :value="data.status" :severity="sev(data.status)" /></template></Column>
      <Column field="notes" header="Notes" />
    </DataTable>
  </section>
</template>

