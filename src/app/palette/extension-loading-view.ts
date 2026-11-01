import type { CommandView } from './model';

export function extensionLoadingView(title = 'Loading…'): CommandView {
  return {
    type: 'list',
    id: `extension-loading:${title}`,
    title,
    searchBarPlaceholder: title,
    contentSizing: 'fit',
    isLoading: true,
    items: [],
  };
}
