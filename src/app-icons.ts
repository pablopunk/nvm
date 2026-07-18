const APP_ICON_PATH_SUFFIXES = ['.app', '.lnk'];

export function isAppIconPath(value: unknown): value is string {
  if (typeof value !== 'string') {
    return false;
  }
  const normalized = value.toLowerCase();
  return APP_ICON_PATH_SUFFIXES.some((suffix) => normalized.endsWith(suffix));
}
