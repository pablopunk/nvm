import { Menu, nativeImage, nativeTheme, screen, Tray } from 'electron';
import type { NormalizedMenuBar } from '../extension-host/extension-menubar';
import type { createExtensionWindowManager } from '../extension-host/extension-window-manager';
import { createMenuBarStatusItems } from '../os/menubar-status-items';
import { bundledResourcePath } from '../platform/bundled-resource-path';
import { createMenuBarVisibility } from '../state/menubar-visibility';
import type { UserStateStore } from '../state/user-state-store';
import {
  createExtensionMenuBarService,
  type ExtensionMenuBarService,
  type ExtensionMenuBarServiceDeps,
} from './extension-menubar-service';

type ExtensionWindows = Pick<
  ReturnType<typeof createExtensionWindowManager>,
  'executeWindowAction' | 'isWindowVisible'
>;

export type AppMenuBarDeps = Omit<
  ExtensionMenuBarServiceDeps,
  'statusItems' | 'isVisibleExtension' | 'popovers'
> & {
  userStateStore: Pick<UserStateStore, 'get'>;
  scheduleSaveState: () => void;
  normalizeView: (view: any, entry: any) => any;
  windows: ExtensionWindows;
};

function popoverWindowId(extensionId: string, menuBarId: string) {
  return `menubar.${extensionId}.${menuBarId}`
    .replace(/[^A-Za-z0-9._:-]/g, '-')
    .slice(0, 128);
}

/** Popovers are host-owned extension windows: frameless, under the icon, closed on blur. */
function createMenuBarPopovers(deps: AppMenuBarDeps) {
  function popoverAction(
    type: string,
    extension: any,
    menuBar: NormalizedMenuBar,
  ) {
    const popover = menuBar.popover;
    if (!popover) return null;
    const id = popoverWindowId(extension.id, menuBar.id);
    const entry = {
      extension,
      command: { id: 'menubar', title: extension.title || extension.id },
    };
    return {
      type,
      windowId: id,
      ownerExtensionId: extension.id,
      view: deps.normalizeView(popover.view, entry),
      windowOptions: {
        id,
        title: menuBar.tooltip,
        position: 'menu-bar',
        chrome: 'none',
        titleBar: 'hidden',
        hideOnBlur: true,
        alwaysOnTop: true,
        visibleOnAllSpaces: true,
        width: popover.width,
        height: popover.height,
      },
    };
  }

  return {
    toggle(extension: any, menuBar: NormalizedMenuBar) {
      const action = popoverAction('toggleWindow', extension, menuBar);
      if (action) deps.windows.executeWindowAction(action);
    },
    updateIfOpen(extension: any, menuBar: NormalizedMenuBar) {
      const id = popoverWindowId(extension.id, menuBar.id);
      if (!deps.windows.isWindowVisible(id)) return;
      const action = popoverAction('updateWindow', extension, menuBar);
      if (action) deps.windows.executeWindowAction(action);
    },
  };
}

/** Status items need app readiness and bundled icons, so the service starts lazily. */
export function createAppMenuBar(deps: AppMenuBarDeps) {
  const visibility = createMenuBarVisibility(deps);
  let service: ExtensionMenuBarService | null = null;
  let starting: Promise<ExtensionMenuBarService> | null = null;

  async function startService() {
    const iconsDir = await bundledResourcePath('menubar-icons');
    const statusItems = createMenuBarStatusItems({
      platform: process.platform,
      desktopEnvironment: process.env.XDG_CURRENT_DESKTOP,
      iconsDir,
      electron: { Tray, Menu, nativeImage, nativeTheme },
      getCursorScreenPoint: () => screen.getCursorScreenPoint(),
      onMenuOpened: (id) => service?.refreshForStatusItem(id),
    });
    service = createExtensionMenuBarService({
      ...deps,
      statusItems,
      isVisibleExtension: (extension) => visibility.isShown(extension.id),
      popovers: createMenuBarPopovers(deps),
    });
    return service;
  }

  async function start() {
    starting ||= startService();
    await (await starting).syncAll();
  }

  return {
    visibility,
    start,
    scheduleSync: () => service?.scheduleSync(),
    refresh: (extensionId: string) => service?.refresh(extensionId),
  };
}

export type AppMenuBar = ReturnType<typeof createAppMenuBar>;
