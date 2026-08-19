import type { SetStateAction } from 'react';
import { useState } from 'react';
import type { CommandView } from './model';

export type NavigationMode = 'root' | 'push' | 'replace' | 'pop';

export type NavigationState = {
  view: CommandView | null;
  backStack: CommandView[];
};

export function nextNavigationState(
  current: NavigationState,
  nextView: CommandView,
  navigation: Exclude<NavigationMode, 'pop'> = 'replace',
): NavigationState {
  if (navigation === 'root') return { view: nextView, backStack: [] };
  if (navigation === 'push' && current.view)
    return { view: nextView, backStack: [...current.backStack, current.view] };
  return { view: nextView, backStack: current.backStack };
}

export function previousNavigationState(current: NavigationState): {
  state: NavigationState;
  didPop: boolean;
} {
  const next = [...current.backStack];
  const previous = next.pop() || null;
  return {
    state: { view: previous, backStack: next },
    didPop: Boolean(previous),
  };
}

export function useExtensionNavigation() {
  const [state, setState] = useState<NavigationState>({
    view: null,
    backStack: [],
  });
  const { view, backStack } = state;

  function setView(nextView: SetStateAction<CommandView | null>) {
    setState((current) => ({
      ...current,
      view: typeof nextView === 'function' ? nextView(current.view) : nextView,
    }));
  }

  function setBackStack(nextBackStack: SetStateAction<CommandView[]>) {
    setState((current) => ({
      ...current,
      backStack:
        typeof nextBackStack === 'function'
          ? nextBackStack(current.backStack)
          : nextBackStack,
    }));
  }

  function showView(
    nextView: CommandView,
    navigation: Exclude<NavigationMode, 'pop'> = 'replace',
  ) {
    setState((current) => nextNavigationState(current, nextView, navigation));
  }

  function popView() {
    let didPop = false;
    setState((current) => {
      const next = previousNavigationState(current);
      didPop = next.didPop;
      return next.state;
    });
    return didPop;
  }

  function clearView() {
    setState({ view: null, backStack: [] });
  }

  return {
    view,
    setView,
    backStack,
    setBackStack,
    showView,
    popView,
    clearView,
  };
}
