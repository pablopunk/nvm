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
    isShown: (extensionId) =>
      menuBarShownInState(deps.userStateStore.get(), extensionId),
    setShown(extensionId, shown) {
      const hidden = hiddenExtensions();
      if (shown) delete hidden[extensionId];
      else hidden[extensionId] = true;
      deps.scheduleSaveState();
    },
  };
}

export const APP_MENU_BAR_EXTENSION_ID = 'nevermind.menubar';
export const MENU_BAR_TOGGLE_ACTION_ID = 'menu-bar-visibility';

export function menuBarSettingRowId(extensionId: string) {
  return `setting:menu-bar:${extensionId}`;
}

/** The Settings row runs the same host toggle, so both surfaces share one state. */
export function menuBarSettingRowPatch(extensionId: string, shown: boolean) {
  const primaryAction = {
    type: 'runExtensionRegisteredAction',
    title: shown ? 'Turn Off' : 'Turn On',
    extensionId,
    registeredActionId: MENU_BAR_TOGGLE_ACTION_ID,
  };
  return {
    id: menuBarSettingRowId(extensionId),
    accessories: [{ text: shown ? 'On' : 'Off' }],
    primaryAction,
    actionPanel: { sections: [{ actions: [primaryAction] }] },
  };
}

export function menuBarShownInState(
  state: Record<string, any>,
  extensionId: string,
) {
  return state.menuBarHidden?.[extensionId] !== true;
}

export function appMenuBarSettingItem(state: Record<string, any>) {
  return {
    title: 'Show Menu Bar Icon',
    subtitle: 'Keep Nevermind in the menu bar so you can always open it',
    icon: 'panel-top',
    ...menuBarSettingRowPatch(
      APP_MENU_BAR_EXTENSION_ID,
      menuBarShownInState(state, APP_MENU_BAR_EXTENSION_ID),
    ),
  };
}
