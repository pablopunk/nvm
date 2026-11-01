import { Menu, nativeImage, nativeTheme, Tray } from 'electron';
import { createMenuBarStatusItems } from '../os/menubar-status-items';
import { bundledResourcePath } from '../platform/bundled-resource-path';
import { createMenuBarVisibility } from '../state/menubar-visibility';
import type { UserStateStore } from '../state/user-state-store';
import {
  createExtensionMenuBarService,
  type ExtensionMenuBarService,
  type ExtensionMenuBarServiceDeps,
} from './extension-menubar-service';

export type AppMenuBarDeps = Omit<
  ExtensionMenuBarServiceDeps,
  'statusItems' | 'isVisibleExtension'
> & {
  userStateStore: Pick<UserStateStore, 'get'>;
  scheduleSaveState: () => void;
};

/** Status items need app readiness and bundled icons, so the service starts lazily. */
export function createAppMenuBar(deps: AppMenuBarDeps) {
  const visibility = createMenuBarVisibility(deps);
  let service: ExtensionMenuBarService | null = null;
  let starting: Promise<ExtensionMenuBarService> | null = null;

  async function startService() {
    const iconsDir = await bundledResourcePath('menubar-icons');
    const statusItems = createMenuBarStatusItems({
      platform: process.platform,
      iconsDir,
      electron: { Tray, Menu, nativeImage, nativeTheme },
      onMenuOpened: (id) => service?.refreshForStatusItem(id),
    });
    service = createExtensionMenuBarService({
      ...deps,
      statusItems,
      isVisibleExtension: (extension) => visibility.isShown(extension.id),
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
