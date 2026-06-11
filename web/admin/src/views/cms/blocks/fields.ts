// Helpers for the block inspector: read/write block.props by dotted path
// (e.g. 'cta.label'), and create fresh block instances from the registry.
import { BLOCK_REGISTRY, type Block } from '@teggo/blocks'

export function getPath(obj: Record<string, any>, path: string): any {
  return path.split('.').reduce<any>((acc, k) => (acc == null ? acc : acc[k]), obj)
}

export function setPath(obj: Record<string, any>, path: string, value: any): void {
  const keys = path.split('.')
  let cur = obj
  for (let i = 0; i < keys.length - 1; i++) {
    const k = keys[i]
    if (cur[k] == null || typeof cur[k] !== 'object') cur[k] = {}
    cur = cur[k]
  }
  cur[keys[keys.length - 1]] = value
}

export function newBlockId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `b-${Math.floor(Math.random() * 1e9).toString(36)}`
}

export function makeBlock(type: string): Block {
  const def = BLOCK_REGISTRY[type]
  return { type, id: newBlockId(), props: def ? def.defaultProps() : {} }
}
