<script setup lang="ts">
import draggable from 'vuedraggable'
import { BlockRenderer, BLOCK_REGISTRY, type Block } from '@teggo/blocks'

const props = defineProps<{ blocks: Block[]; selectedId: string | null }>()
const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'duplicate', index: number): void
  (e: 'remove', index: number): void
}>()

function label(b: Block) {
  return BLOCK_REGISTRY[b.type]?.label ?? b.type
}

// Fired when a block is dropped in from the palette — select the new block.
function onAdd(e: { newIndex?: number }) {
  const b = e.newIndex != null ? props.blocks[e.newIndex] : undefined
  if (b?.id) emit('select', b.id)
}
</script>

<template>
  <div class="canvas">
    <!-- :list mutates the array in place (reorder + accept palette drops). -->
    <draggable
      :list="blocks"
      item-key="id"
      handle=".drag"
      :group="{ name: 'blocks', pull: true, put: true }"
      :animation="150"
      class="stack"
      @add="onAdd"
    >
      <template #item="{ element, index }">
        <div
          class="card"
          :class="{ selected: element.id === selectedId }"
          @click="emit('select', element.id)"
        >
          <div class="bar">
            <i class="pi pi-bars drag" title="Drag to reorder" @click.stop />
            <span class="kind">{{ label(element) }}</span>
            <span class="spacer" />
            <button type="button" class="icon" title="Duplicate" @click.stop="emit('duplicate', index)">
              <i class="pi pi-copy" />
            </button>
            <button type="button" class="icon danger" title="Delete" @click.stop="emit('remove', index)">
              <i class="pi pi-trash" />
            </button>
          </div>
          <!-- WYSIWYG: same renderer the storefront uses. Inert in the builder. -->
          <div class="preview">
            <BlockRenderer :blocks="[element]" :preview="true" />
          </div>
        </div>
      </template>
      <template #footer>
        <p v-if="!blocks.length" class="empty muted">
          Drag a block here, or click one in the palette.
        </p>
      </template>
    </draggable>
  </div>
</template>

<style scoped>
.canvas { min-height: 12rem; }
.stack { display: flex; flex-direction: column; gap: 0.85rem; min-height: 6rem; }
.empty { text-align: center; padding: 2.5rem 1rem; border: 1.5px dashed var(--p-surface-300, #cbd5e1); border-radius: 10px; }
.muted { color: var(--p-text-muted-color, #64748b); }

.card {
  border: 1.5px solid var(--p-surface-200, #e2e8f0); border-radius: 10px;
  overflow: hidden; background: var(--p-surface-0, #fff); cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.card:hover { border-color: var(--p-surface-300, #cbd5e1); }
.card.selected { border-color: var(--p-primary-color, #3b82f6); box-shadow: 0 0 0 2px var(--p-primary-100, #dbeafe); }

.bar {
  display: flex; align-items: center; gap: 0.6rem;
  padding: 0.4rem 0.6rem; background: var(--p-surface-50, #f8fafc);
  border-bottom: 1px solid var(--p-surface-200, #e2e8f0); font-size: 0.85rem;
}
.bar .kind { font-weight: 600; }
.bar .spacer { flex: 1; }
.drag { cursor: grab; color: var(--p-text-muted-color, #94a3b8); }
.drag:active { cursor: grabbing; }
.icon { border: 0; background: transparent; cursor: pointer; color: var(--p-text-muted-color, #64748b); padding: 0.25rem; border-radius: 6px; }
.icon:hover { background: var(--p-surface-200, #e2e8f0); }
.icon.danger:hover { color: var(--p-red-500, #ef4444); }

/* Inert preview: clicks select the card rather than following links. */
.preview { padding: 1rem; pointer-events: none; }
.preview :deep(.block:last-child) { margin-bottom: 0; }
</style>
