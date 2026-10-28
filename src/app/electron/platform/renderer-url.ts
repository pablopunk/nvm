export function resolveRendererUrl(
  environment: Record<string, string | undefined>,
  isPackaged: boolean,
): string | undefined {
  if (isPackaged) return undefined;
  return environment.ELECTRON_RENDERER_URL?.trim() || undefined;
}
