import type { UserStateStore } from './user-state-store';

export type MenuBarVisibility = {
  isShown(extensionId: string): boolean;
  setShown(extensionId: string, shown: boolean): void;
};

/** Menu bar items are shown by default; the user hides them per extension from the palette. */
export function createMenuBarVisibility(deps: {
  userStateStore: Pick<UserStateStore, 'get'>;
  scheduleSaveState: () => void;
}): MenuBarVisibility {
  function hiddenExtensions(): Record<string, boolean> {
    const state = deps.userStateStore.get();
    if (!state.menuBarHidden || typeof state.menuBarHidden !== 'object')
      state.menuBarHidden = {};
    return state.menuBarHidden;
  }

  return {
    isShown: (extensionId) => hiddenExtensions()[extensionId] !== true,
    setShown(extensionId, shown) {
      const hidden = hiddenExtensions();
      if (shown) delete hidden[extensionId];
      else hidden[extensionId] = true;
      deps.scheduleSaveState();
    },
  };
}
