import path from 'node:path';
import type {
  Menu as ElectronMenu,
  MenuItemConstructorOptions,
  NativeImage,
  Tray as ElectronTray,
} from 'electron';
import {
  rememberStatusItemAnchor,
  type StatusItemAnchor,
  statusItemAnchorFrom,
} from './status-item-anchor';
import {
  glyphFollowsTheme,
  type StatusItemGlyph,
} from '../../shared/menubar-icons';
import {
  menuBarStatusItemsPlatform,
  menuEntryAtPath,
  menuRebuildCooldownMs,
  opensMenuOnPrimaryClick,
  type StatusItemMenuEntry,
  type StatusItemSpec,
  statusAreaIsDark,
  statusItemAppearanceSignature,
  statusItemIconFile,
  statusItemMenuEntries,
  statusItemMenuSignature,
  statusItemMenuTemplate,
  statusItemOwnsContextMenu,
  statusItemTitle,
  statusItemTooltip,
  statusItemUsesEmojiTitle,
  supportsStatusTitle,
  usesTemplateImages,
} from './menubar-status-item-model';

export type {
  StatusItemMenuEntry,
  StatusItemSpec,
} from './menubar-status-item-model';

export type MenuBarStatusItemsDeps = {
  platform: NodeJS.Platform;
  /** `XDG_CURRENT_DESKTOP`; some Linux panels stay dark whatever the app theme is. */
  desktopEnvironment?: string;
  iconsDir: string;
  electron: {
    Tray: new (image: NativeImage) => ElectronTray;
    Menu: {
      buildFromTemplate(template: MenuItemConstructorOptions[]): ElectronMenu;
    };
    nativeImage: {
      createFromPath(path: string): NativeImage;
      createEmpty(): NativeImage;
    };
    nativeTheme: {
      readonly shouldUseDarkColors: boolean;
      readonly shouldUseDarkColorsForSystemIntegratedUI?: boolean;
      on(event: 'updated', listener: () => void): unknown;
    };
  };
  /** Fires when the user opens an item's menu; refresh it in the background for the next open. */
  onMenuOpened?: (id: string) => void;
  /** Anchors primary clicks where trays report no icon bounds (Linux). */
  getCursorScreenPoint?: () => { x: number; y: number };
};

type LiveStatusItem = {
  tray: ElectronTray;
  spec: StatusItemSpec;
  /** Spec the native menu was built from; clicks resolve against it. */
  menuSpec: StatusItemSpec;
  appearanceSignature: string;
  menuSignature: string;
  menu: ElectronMenu | null;
  menuOpen: boolean;
  menuRebuildPending: boolean;
  menuBuiltAt: number;
  menuRebuildTimer: ReturnType<typeof setTimeout> | null;
};

export function createMenuBarStatusItems(deps: MenuBarStatusItemsDeps) {
  const platform = menuBarStatusItemsPlatform(deps.platform);
  const { Tray, Menu, nativeImage, nativeTheme } = deps.electron;
  const live = new Map<string, LiveStatusItem>();
  const images = new Map<string, NativeImage>();

  function loadImage(icon: StatusItemGlyph, darkBackground: boolean) {
    if (!platform) throw new Error('Status items are not supported');
    if (statusItemUsesEmojiTitle(icon, platform))
      return nativeImage.createEmpty();
    const file = statusItemIconFile(icon, platform, darkBackground);
    const cached = images.get(file);
    if (cached) return cached;
    const image = nativeImage.createFromPath(path.join(deps.iconsDir, file));
    if (usesTemplateImages(platform) && glyphFollowsTheme(icon))
      image.setTemplateImage(true);
    images.set(file, image);
    return image;
  }

  function statusImage(icon: StatusItemGlyph) {
    if (!platform) throw new Error('Status items are not supported');
    return loadImage(
      icon,
      statusAreaIsDark({
        platform,
        desktopEnvironment: deps.desktopEnvironment,
        shouldUseDarkColors: nativeTheme.shouldUseDarkColors,
        shouldUseDarkColorsForSystemIntegratedUI:
          nativeTheme.shouldUseDarkColorsForSystemIntegratedUI,
      }),
    );
  }

  function menuItemImage(icon: StatusItemGlyph) {
    return loadImage(icon, false);
  }

  function markMenuOpen(id: string) {
    const item = live.get(id);
    if (item) item.menuOpen = true;
    deps.onMenuOpened?.(id);
  }

  function applyPendingAfterMenuCloses(id: string) {
    const item = live.get(id);
    if (!item) return;
    item.menuOpen = false;
    if (!item.menuRebuildPending) return;
    item.menuRebuildPending = false;
    applyMenu(id, item);
  }

  function menuEntriesFor(id: string, spec: StatusItemSpec) {
    if (!platform) return spec.menu;
    return statusItemMenuEntries(spec, platform, () => runPrimaryClick(id));
  }

  function runPrimaryClick(id: string, bounds?: Partial<StatusItemAnchor>) {
    const cursor = deps.getCursorScreenPoint?.() ?? { x: 0, y: 0 };
    rememberStatusItemAnchor(statusItemAnchorFrom(bounds, cursor));
    live.get(id)?.spec.onPrimaryClick?.();
  }

  function handlePrimaryClick(id: string, bounds?: Partial<StatusItemAnchor>) {
    const item = live.get(id);
    if (!(item && platform)) return;
    if (item.spec.onPrimaryClick) {
      runPrimaryClick(id, bounds);
      return;
    }
    if (!opensMenuOnPrimaryClick(platform)) return;
    deps.onMenuOpened?.(id);
    item.tray.popUpContextMenu();
  }

  function handleSecondaryClick(id: string) {
    const item = live.get(id);
    if (!(item?.menu && platform)) return;
    if (statusItemOwnsContextMenu(item.menuSpec, platform)) return;
    if (!item.menuSpec.menu.length) return;
    deps.onMenuOpened?.(id);
    item.tray.popUpContextMenu(item.menu);
  }

  function clickCurrentEntry(id: string, path: number[]) {
    const item = live.get(id);
    const entries = item ? menuEntriesFor(id, item.menuSpec) : [];
    const entry = menuEntryAtPath(entries, path);
    if (entry?.type === 'item') entry.onClick?.();
  }

  /** Clicks resolve against the latest spec, so unchanged menus never need rebuilding. */
  function withLiveClickHandlers(
    id: string,
    entries: StatusItemMenuEntry[],
    path: number[] = [],
  ): StatusItemMenuEntry[] {
    return entries.map((entry, index) => {
      if (entry.type !== 'item') return entry;
      const entryPath = [...path, index];
      return {
        ...entry,
        ...(entry.submenu
          ? { submenu: withLiveClickHandlers(id, entry.submenu, entryPath) }
          : {}),
        ...(entry.onClick
          ? { onClick: () => clickCurrentEntry(id, entryPath) }
          : {}),
      };
    });
  }

  function buildMenu(id: string, spec: StatusItemSpec) {
    if (!platform) throw new Error('Status items are not supported');
    const menu = Menu.buildFromTemplate(
      statusItemMenuTemplate(
        withLiveClickHandlers(id, menuEntriesFor(id, spec)),
        platform,
        menuItemImage,
      ),
    );
    menu.on('menu-will-show', () => markMenuOpen(id));
    menu.on('menu-will-close', () =>
      setTimeout(() => applyPendingAfterMenuCloses(id), 0),
    );
    return menu;
  }

  function applyAppearance(item: LiveStatusItem) {
    if (!platform) return;
    const { tray, spec } = item;
    tray.setImage(statusImage(spec.icon));
    if (supportsStatusTitle(platform))
      tray.setTitle(statusItemTitle(spec, platform), {
        fontType: 'monospacedDigit',
      });
    tray.setToolTip(statusItemTooltip(spec, platform));
  }

  /** Rebuilding replaces an open native menu, so callers rebuild only when rows change. */
  function applyMenu(id: string, item: LiveStatusItem) {
    if (item.menuRebuildTimer) clearTimeout(item.menuRebuildTimer);
    item.menuRebuildTimer = null;
    item.menuBuiltAt = Date.now();
    item.menuSpec = item.spec;
    item.menuSignature = statusItemMenuSignature(item.spec);
    item.menu = buildMenu(id, item.spec);
    item.tray.setContextMenu(
      platform && statusItemOwnsContextMenu(item.spec, platform)
        ? item.menu
        : null,
    );
  }

  function createTray(id: string, spec: StatusItemSpec) {
    const tray = new Tray(statusImage(spec.icon));
    tray.on('click', (_event, bounds) => handlePrimaryClick(id, bounds));
    tray.on('right-click', () => handleSecondaryClick(id));
    return tray;
  }

  function updateExisting(
    id: string,
    item: LiveStatusItem,
    spec: StatusItemSpec,
  ) {
    item.spec = spec;
    const appearanceSignature = statusItemAppearanceSignature(spec);
    if (item.appearanceSignature !== appearanceSignature) {
      item.appearanceSignature = appearanceSignature;
      applyAppearance(item);
    }
    if (item.menuSignature === statusItemMenuSignature(spec)) {
      item.menuSpec = spec;
      return;
    }
    if (item.menuOpen) item.menuRebuildPending = true;
    else scheduleMenuRebuild(id, item);
  }

  function scheduleMenuRebuild(id: string, item: LiveStatusItem) {
    if (!platform || item.menuRebuildTimer) return;
    const wait =
      item.menuBuiltAt + menuRebuildCooldownMs(platform) - Date.now();
    if (wait <= 0) {
      applyMenu(id, item);
      return;
    }
    item.menuRebuildTimer = setTimeout(() => {
      if (live.get(id) === item) applyMenu(id, item);
    }, wait);
    item.menuRebuildTimer.unref?.();
  }

  function upsert(id: string, spec: StatusItemSpec) {
    if (!platform) return false;
    const existing = live.get(id);
    if (existing) {
      updateExisting(id, existing, spec);
      return true;
    }
    const item: LiveStatusItem = {
      tray: createTray(id, spec),
      spec,
      menuSpec: spec,
      appearanceSignature: statusItemAppearanceSignature(spec),
      menuSignature: '',
      menu: null,
      menuOpen: false,
      menuRebuildPending: false,
      menuBuiltAt: 0,
      menuRebuildTimer: null,
    };
    live.set(id, item);
    applyAppearance(item);
    applyMenu(id, item);
    return true;
  }

  function remove(id: string) {
    const item = live.get(id);
    if (!item) return;
    live.delete(id);
    if (item.menuRebuildTimer) clearTimeout(item.menuRebuildTimer);
    item.tray.destroy();
  }

  function retainOnly(ids: Iterable<string>) {
    const keep = new Set(ids);
    for (const id of Array.from(live.keys())) if (!keep.has(id)) remove(id);
  }

  function refreshImagesForTheme() {
    for (const item of live.values())
      item.tray.setImage(statusImage(item.spec.icon));
  }

  if (platform && !usesTemplateImages(platform))
    nativeTheme.on('updated', refreshImagesForTheme);

  return {
    supported: Boolean(platform),
    upsert,
    remove,
    retainOnly,
    ids: () => Array.from(live.keys()),
  };
}

export type MenuBarStatusItems = ReturnType<typeof createMenuBarStatusItems>;
