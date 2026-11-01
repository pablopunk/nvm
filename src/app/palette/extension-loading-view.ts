import type { CommandView } from './model';

const LOADING_VIEW_ID_PREFIX = 'extension-loading:';

export function isExtensionLoadingView(view: CommandView) {
  return view.id.startsWith(LOADING_VIEW_ID_PREFIX);
}

export function extensionLoadingView(title = 'Loading…'): CommandView {
  return {
    type: 'list',
    id: `${LOADING_VIEW_ID_PREFIX}${title}`,
    title,
    searchBarPlaceholder: title,
    contentSizing: 'fit',
    isLoading: true,
    items: [],
  };
}
