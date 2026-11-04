/** Static presentation of the Clipboard History list; items and actions are per-render. */
export const ClipboardHistoryViewChrome = {
  type: 'list',
  id: 'clipboard-history',
  title: 'Clipboard History',
  presentation: 'side-preview',
  searchBarPlaceholder: 'Search Clipboard History',
  onSelectionChange: {
    type: 'loadClipboardLinkPreview',
    title: 'Load link preview',
  },
  emptyView: {
    title: 'No clipboard items found.',
    subtitle: 'Copy text or images and they will appear here.',
  },
  // Removal actions stay in Command-K; they are not search results.
  actionPanelVisibility: 'menu',
};
