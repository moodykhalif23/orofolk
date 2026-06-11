<script setup lang="ts">
import draggable from 'vuedraggable'
import { BLOCK_REGISTRY, BLOCK_TYPES } from '@teggo/blocks'
import { makeBlock } from './fields'

defineEmits<{ (e: 'add', type: string): void }>()

// Static descriptors for the draggable source list.
const items = BLOCK_TYPES.map((type) => ({ type }))

// When an item is dragged into the canvas, clone it into a real Block.
function cloneToBlock(item: { type: string }) {
  return makeBlock(item.type)
}
</script>

<template>
  <div class="palette">
    <h3>Add a block</h3>
    <draggable
      :list="items"
      :group="{ name: 'blocks', pull: 'clone', put: false }"
      :clone="cloneToBlock"
      :sort="false"
      item-key="type"
      class="list"
    >
      <template #item="{ element }">
        <button type="button" class="palette-item" @click="$emit('add', element.type)">
          <i :class="BLOCK_REGISTRY[element.type].icon" />
          <span>{{ BLOCK_REGISTRY[element.type].label }}</span>
          <i class="pi pi-plus add" />
        </button>
      </template>
    </draggable>
    <small class="muted">Click or drag a block onto the canvas.</small>
  </div>
</template>

<style scoped>
.palette { display: flex; flex-direction: column; gap: 0.5rem; }
.palette h3 { margin: 0 0 0.25rem; font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.04em; color: var(--p-text-muted-color, #64748b); }
.list { display: flex; flex-direction: column; gap: 0.5rem; }
.palette-item {
  display: flex; align-items: center; gap: 0.6rem;
  width: 100%; padding: 0.6rem 0.75rem;
  border: 1px solid var(--p-surface-200, #e2e8f0); border-radius: 8px;
  background: var(--p-surface-0, #fff); cursor: grab; text-align: left;
  font: inherit; color: inherit; transition: border-color 0.15s, background 0.15s;
}
.palette-item:hover { border-color: var(--p-primary-color, #3b82f6); background: var(--p-surface-50, #f8fafc); }
.palette-item:active { cursor: grabbing; }
.palette-item > span { flex: 1; font-weight: 500; }
.palette-item .add { color: var(--p-text-muted-color, #94a3b8); font-size: 0.8rem; }
.muted { color: var(--p-text-muted-color, #64748b); font-size: 0.78rem; }
</style>
