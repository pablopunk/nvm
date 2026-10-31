interface ListItemClassNameInput {
  disabled?: boolean;
}

const WRAP_SUBTITLE_VIEW_IDS = new Set(['os-permissions']);

export function extensionListItemClassName(
  viewId: string | undefined,
  item: ListItemClassNameInput,
) {
  return [
    'extensionListItem',
    (item.disabled || WRAP_SUBTITLE_VIEW_IDS.has(viewId ?? '')) &&
      'extensionListItemWrapSubtitle',
    item.disabled && 'extensionListItemContext',
  ]
    .filter(Boolean)
    .join(' ');
}
