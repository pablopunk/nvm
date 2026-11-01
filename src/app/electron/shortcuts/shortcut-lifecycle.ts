export type ShortcutLifecycleAction = {
  background?: boolean;
  mode?: string;
  rootAction?: unknown;
  persistentAction?: unknown;
  [key: string]: unknown;
};

export type ShortcutLifecycle = Pick<
  ShortcutLifecycleAction,
  'background' | 'mode'
>;

export function shortcutActionLifecycle(
  action: unknown,
): ShortcutLifecycle | undefined {
  const visited = new Set<object>();

  function resolve(candidate: unknown): ShortcutLifecycle | undefined {
    if (!candidate || typeof candidate !== 'object') return;
    if (visited.has(candidate)) return;
    visited.add(candidate);
    const current = candidate as ShortcutLifecycleAction;
    if (current.background || current.mode)
      return {
        background: current.background,
        mode: current.mode,
      };
    return resolve(current.persistentAction) || resolve(current.rootAction);
  }

  return resolve(action);
}

export function shortcutActionOpensPalette(action: unknown): boolean {
  const current = action as ShortcutLifecycleAction | undefined;
  const rootAction = current?.rootAction as { type?: unknown } | undefined;
  return current?.type === 'openPalette' || rootAction?.type === 'openPalette';
}

export function shortcutActionRunsWithoutView(action: unknown): boolean {
  const lifecycle = shortcutActionLifecycle(action);
  return Boolean(
    lifecycle?.background ||
      lifecycle?.mode === 'background' ||
      lifecycle?.mode === 'noView',
  );
}

export function withInheritedShortcutLifecycle<
  T extends ShortcutLifecycleAction,
>(action: T, inheritedFrom: unknown): T {
  const lifecycle = shortcutActionLifecycle(inheritedFrom);
  if (!lifecycle) return action;
  const background = action.background || lifecycle.background;
  const mode = action.mode || lifecycle.mode;
  return {
    ...action,
    ...(background !== undefined ? { background } : {}),
    ...(mode !== undefined ? { mode } : {}),
  };
}
