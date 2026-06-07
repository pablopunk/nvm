import type { CommandItem, CommandView, CommandViewPatch } from './model'

export type PatchCommandViewOptions = {
  preserveMissingItems?: boolean
}

function patchCommandItems(items: CommandItem[] | undefined, patches: NonNullable<CommandViewPatch['items']> = [], mode: CommandViewPatch['mode'] = 'patch', removeItemIds: string[] = [], options: PatchCommandViewOptions = {}) {
  if (!Array.isArray(items)) return options.preserveMissingItems ? items : []
  let next = items
  if (removeItemIds.length) {
    const remove = new Set(removeItemIds)
    next = next.filter((item) => !remove.has(item.id))
  }
  if (patches.length === 0) return next
  if (mode === 'replace') return patches as CommandItem[]
  const byId = new Map(next.map((item) => [item.id, item]))
  const patchedIds = new Set(patches.map((patch) => patch.id))
  const patchById = new Map(patches.map((patch) => [patch.id, patch]))
  const patchedItems = patches.map((patch) => ({ ...(byId.get(patch.id) || {} as CommandItem), ...patch }))
  if (mode === 'prepend') return [...patchedItems, ...next.filter((item) => !patchedIds.has(item.id))]
  if (mode === 'append') return [...next.filter((item) => !patchedIds.has(item.id)), ...patchedItems]
  return next.map((item) => patchedIds.has(item.id) ? { ...item, ...patchById.get(item.id) } : item)
}

export function patchCommandView(current: CommandView, patch: CommandViewPatch, options: PatchCommandViewOptions = {}): CommandView {
  const mode = patch.mode || current.refresh?.mode
  const patches = patch.items || []
  const removeItemIds = patch.removeItemIds || []
  return {
    ...current,
    ...(patch.isLoading === undefined ? {} : { isLoading: patch.isLoading }),
    ...(patch.selectedItemId === undefined ? {} : { selectedItemId: patch.selectedItemId }),
    items: patchCommandItems(current.items, patches, mode, removeItemIds, options),
    sections: current.sections?.map((section) => ({ ...section, items: patchCommandItems(section.items, patches, mode, removeItemIds, options) || [] })),
  }
}
