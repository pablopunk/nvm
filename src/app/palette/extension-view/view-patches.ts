import { allViewItems } from '../filtering';
import type { CommandItem, CommandView, CommandViewPatch } from '../model';

export type PatchCommandViewOptions = {
  preserveMissingItems?: boolean;
};

function commandItemFromPatch(
  existing: CommandItem | undefined,
  patch: Partial<CommandItem> & { id: string },
): CommandItem {
  return {
    ...(existing || ({} as CommandItem)),
    ...patch,
    title: String(patch.title || existing?.title || patch.id),
  };
}

function patchCommandItems(
  items: CommandItem[] | undefined,
  patches: NonNullable<CommandViewPatch['items']> = [],
  mode: CommandViewPatch['mode'] = 'patch',
  removeItemIds: string[] = [],
  options: PatchCommandViewOptions = {},
) {
  if (
    !Array.isArray(items) &&
    options.preserveMissingItems &&
    patches.length === 0 &&
    removeItemIds.length === 0
  )
    return items;
  let next = Array.isArray(items) ? items : [];
  if (removeItemIds.length) {
    const remove = new Set(removeItemIds);
    next = next.filter((item) => !remove.has(item.id));
  }
  if (patches.length === 0) return next;
  if (mode === 'replace')
    return patches.map((patch) => commandItemFromPatch(undefined, patch));
  const byId = new Map(next.map((item) => [item.id, item]));
  const patchedIds = new Set(patches.map((patch) => patch.id));
  const patchById = new Map(patches.map((patch) => [patch.id, patch]));
  const patchedItems = patches.map((patch) =>
    commandItemFromPatch(byId.get(patch.id), patch),
  );
  if (mode === 'prepend')
    return [
      ...patchedItems,
      ...next.filter((item) => !patchedIds.has(item.id)),
    ];
  if (mode === 'append')
    return [
      ...next.filter((item) => !patchedIds.has(item.id)),
      ...patchedItems,
    ];
  return next.map((item) =>
    patchedIds.has(item.id)
      ? commandItemFromPatch(item, patchById.get(item.id)!)
      : item,
  );
}

export function patchCommandView(
  current: CommandView,
  patch: CommandViewPatch,
  options: PatchCommandViewOptions = {},
): CommandView {
  const mode = patch.mode || current.refresh?.mode;
  const patches = patch.items || [];
  const removeItemIds = patch.removeItemIds || [];
  return {
    ...current,
    ...(patch.isLoading === undefined ? {} : { isLoading: patch.isLoading }),
    ...(patch.selectedItemId === undefined
      ? {}
      : { selectedItemId: patch.selectedItemId }),
    items: patchCommandItems(
      current.items,
      patches,
      mode,
      removeItemIds,
      options,
    ),
    sections: current.sections?.map((section) => ({
      ...section,
      items:
        patchCommandItems(
          section.items,
          patches,
          mode,
          removeItemIds,
          options,
        ) || [],
    })),
  };
}

export function selectionAfterViewPatch(
  current: CommandView,
  next: CommandView,
  patch: CommandViewPatch,
  selected: string,
) {
  const nextItems = allViewItems(next);
  if (nextItems.length === 0) return '';
  if (patch.selectedItemId !== undefined)
    return nextItems.some((item) => item.id === patch.selectedItemId)
      ? patch.selectedItemId
      : nextItems[0].id;
  if (
    next.selectedItemId &&
    nextItems.some((item) => item.id === next.selectedItemId)
  )
    return next.selectedItemId;
  if (selected && nextItems.some((item) => item.id === selected))
    return selected;
  const currentItems = allViewItems(current);
  const currentIndex = currentItems.findIndex((item) => item.id === selected);
  const fallbackIndex =
    currentIndex < 0 ? 0 : Math.min(currentIndex, nextItems.length - 1);
  return nextItems[fallbackIndex]?.id || '';
}
