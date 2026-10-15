const VIEW_TYPES = new Set([
  'list',
  'grid',
  'preview',
  'chat',
  'form',
  'editor',
  'progress',
  'webview',
  'camera',
]);

export function isView(value) {
  return Boolean(value?.type && VIEW_TYPES.has(value.type));
}
