interface ListItemClassNameInput {
  disabled?: boolean;
  contextual?: boolean;
}

function isContextualRow(item: ListItemClassNameInput) {
  return Boolean(item.disabled && item.contextual);
}

const WRAP_SUBTITLE_VIEW_IDS = new Set(['os-permissions']);

export function extensionListItemClassName(
  viewId: string | undefined,
  item: ListItemClassNameInput,
) {
  return [
    'extensionListItem',
    (isContextualRow(item) || WRAP_SUBTITLE_VIEW_IDS.has(viewId ?? '')) &&
      'extensionListItemWrapSubtitle',
    isContextualRow(item) && 'extensionListItemContext',
  ]
    .filter(Boolean)
    .join(' ');
}
