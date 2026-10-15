const ITEM_FOREGROUND_COLORS = new Set([
  'yellow',
  'blue',
  'purple',
  'green',
  'red',
  'orange',
  'pink',
]);

export function normalizeItemAppearance(appearance) {
  const foreground = appearance?.foreground;
  const background = appearance?.background;
  const prominence = appearance?.prominence;
  const normalized = {
    ...(ITEM_FOREGROUND_COLORS.has(foreground) ? { foreground } : {}),
    ...(background === 'accent' ? { background } : {}),
    ...(prominence === 'primary' ? { prominence } : {}),
  };
  return Object.keys(normalized).length ? normalized : undefined;
}
