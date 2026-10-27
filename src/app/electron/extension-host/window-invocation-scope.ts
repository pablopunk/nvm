import { AsyncLocalStorage } from 'node:async_hooks';
import type {
  WindowFocusTarget,
  WindowInvocation,
} from '../os/window-management-types';

type WindowInvocationScope =
  | { source: 'palette' }
  | { source: 'shortcut'; target: WindowFocusTarget | null };

const windowInvocationScopes = new AsyncLocalStorage<WindowInvocationScope>();

export function runInWindowInvocationScope<Result>(
  scope: WindowInvocationScope,
  task: () => Result,
): Result {
  return windowInvocationScopes.run(scope, task);
}

export function currentWindowInvocationScope():
  | WindowInvocationScope
  | undefined {
  return windowInvocationScopes.getStore();
}

export function windowInvocationFromScope(
  scope: WindowInvocationScope | undefined,
  paletteFocusTarget: () => WindowFocusTarget | null,
): WindowInvocation | null {
  if (!scope) {
    return null;
  }
  if (scope.source === 'shortcut') {
    return { source: 'shortcut', target: scope.target };
  }
  return { source: 'palette', target: paletteFocusTarget() };
}

export type { WindowInvocationScope };
