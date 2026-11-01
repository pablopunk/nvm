import path from 'node:path';
import type {
  Menu as ElectronMenu,
  MenuItemConstructorOptions,
  NativeImage,
  Tray as ElectronTray,
} from 'electron';
import {
  DEFAULT_MENUBAR_ICON,
  isMenuBarIconName,
  type MenuBarIconName,
} from '../../shared/menubar-icons';

export type StatusItemMenuEntry =
  | { type: 'separator' }
  | { type: 'header'; label: string }
  | {
      type: 'item';
      label: string;
      sublabel?: string;
      icon?: MenuBarIconName;
      checked?: boolean;
      enabled?: boolean;
      accelerator?: string;
      submenu?: StatusItemMenuEntry[];
      onClick?: () => void;
    };

export type StatusItemSpec = {
  icon: MenuBarIconName;
  title?: string;
  tooltip: string;
  menu: StatusItemMenuEntry[];
};

export type StatusItemPlatform = 'darwin' | 'win32' | 'linux';

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
    nativeImage: { createFromPath(path: string): NativeImage };
    nativeTheme: {
      readonly shouldUseDarkColors: boolean;
      readonly shouldUseDarkColorsForSystemIntegratedUI?: boolean;
      on(event: 'updated', listener: () => void): unknown;
    };
  };
  /** Fires when the user opens an item's menu; refresh it in the background for the next open. */
  onMenuOpened?: (id: string) => void;
};

type LiveStatusItem = {
  tray: ElectronTray;
  spec: StatusItemSpec;
  /** Spec the native menu was built from; clicks resolve against it. */
  menuSpec: StatusItemSpec;
  appearanceSignature: string;
  menuSignature: string;
  menuOpen: boolean;
  menuRebuildPending: boolean;
  menuBuiltAt: number;
  menuRebuildTimer: ReturnType<typeof setTimeout> | null;
};

export function menuBarStatusItemsPlatform(
  platform: NodeJS.Platform,
): StatusItemPlatform | null {
  return platform === 'darwin' || platform === 'win32' || platform === 'linux'
    ? platform
    : null;
}

function signatureWithoutHandlers(value: unknown) {
  return JSON.stringify(value, (_key, entry) =>
    typeof entry === 'function' ? undefined : entry,
  );
}

export function statusItemAppearanceSignature(spec: StatusItemSpec) {
  return signatureWithoutHandlers([spec.icon, spec.title, spec.tooltip]);
}

export function statusItemMenuSignature(spec: StatusItemSpec) {
  return signatureWithoutHandlers(spec.menu);
}

function usesTemplateImages(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

function supportsStatusTitle(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

function supportsMenuSublabels(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

function supportsMenuAccelerators(platform: StatusItemPlatform) {
  return platform !== 'linux';
}

function supportsMenuItemIcons(platform: StatusItemPlatform) {
  return platform === 'darwin';
}

const LINUX_MENU_REBUILD_COOLDOWN_MS = 5000;

/** Linux trays report no menu open/close, and a rebuild replaces an open menu. */
export function menuRebuildCooldownMs(platform: StatusItemPlatform) {
  return platform === 'linux' ? LINUX_MENU_REBUILD_COOLDOWN_MS : 0;
}

function opensMenuOnPrimaryClick(platform: StatusItemPlatform) {
  return platform === 'win32';
}

export function statusItemIconFile(
  icon: MenuBarIconName,
  platform: StatusItemPlatform,
  darkBackground: boolean,
) {
  const name = isMenuBarIconName(icon) ? icon : DEFAULT_MENUBAR_ICON;
  return !usesTemplateImages(platform) && darkBackground
    ? `${name}-white.png`
    : `${name}.png`;
}

const ALWAYS_DARK_LINUX_PANELS = /gnome|unity|ubuntu|pantheon|budgie/i;

/** Tray glyphs must contrast with the panel, which does not always follow the app theme. */
export function statusAreaIsDark(input: {
  platform: StatusItemPlatform;
  desktopEnvironment?: string;
  shouldUseDarkColors: boolean;
  shouldUseDarkColorsForSystemIntegratedUI?: boolean;
}) {
  if (input.platform === 'win32')
    return (
      input.shouldUseDarkColorsForSystemIntegratedUI ??
      input.shouldUseDarkColors
    );
  if (input.platform === 'linux')
    return (
      ALWAYS_DARK_LINUX_PANELS.test(input.desktopEnvironment || '') ||
      input.shouldUseDarkColors
    );
  return input.shouldUseDarkColors;
}

export function statusItemTooltip(
  spec: StatusItemSpec,
  platform: StatusItemPlatform,
) {
  if (supportsStatusTitle(platform) || !spec.title) return spec.tooltip;
  return `${spec.tooltip} — ${spec.title}`;
}

export function statusItemMenuTemplate(
  entries: StatusItemMenuEntry[],
  platform: StatusItemPlatform,
  menuIcon: (icon: MenuBarIconName) => NativeImage,
): MenuItemConstructorOptions[] {
  return entries.map((entry): MenuItemConstructorOptions => {
    if (entry.type === 'separator') return { type: 'separator' };
    if (entry.type === 'header') return { label: entry.label, enabled: false };
    const label =
      entry.sublabel && !supportsMenuSublabels(platform)
        ? `${entry.label} — ${entry.sublabel}`
        : entry.label;
    return {
      label,
      ...(entry.sublabel && supportsMenuSublabels(platform)
        ? { sublabel: entry.sublabel }
        : {}),
      ...(entry.checked !== undefined
        ? { type: 'checkbox' as const, checked: entry.checked }
        : {}),
      enabled: entry.enabled !== false,
      ...(entry.accelerator && supportsMenuAccelerators(platform)
        ? { accelerator: entry.accelerator, registerAccelerator: false }
        : {}),
      ...(entry.icon && supportsMenuItemIcons(platform)
        ? { icon: menuIcon(entry.icon) }
        : {}),
      ...(entry.submenu?.length
        ? { submenu: statusItemMenuTemplate(entry.submenu, platform, menuIcon) }
        : {}),
      ...(entry.onClick ? { click: entry.onClick } : {}),
    };
  });
}

export function menuEntryAtPath(
  entries: StatusItemMenuEntry[],
  path: number[],
): StatusItemMenuEntry | undefined {
  let list: StatusItemMenuEntry[] | undefined = entries;
  let entry: StatusItemMenuEntry | undefined;
  for (const index of path) {
    entry = list?.[index];
    list = entry?.type === 'item' ? entry.submenu : undefined;
  }
  return entry;
}

export function createMenuBarStatusItems(deps: MenuBarStatusItemsDeps) {
  const platform = menuBarStatusItemsPlatform(deps.platform);
  const { Tray, Menu, nativeImage, nativeTheme } = deps.electron;
  const live = new Map<string, LiveStatusItem>();
  const images = new Map<string, NativeImage>();

  function loadImage(icon: MenuBarIconName, darkBackground: boolean) {
    if (!platform) throw new Error('Status items are not supported');
    const file = statusItemIconFile(icon, platform, darkBackground);
    const cached = images.get(file);
    if (cached) return cached;
    const image = nativeImage.createFromPath(path.join(deps.iconsDir, file));
    if (usesTemplateImages(platform)) image.setTemplateImage(true);
    images.set(file, image);
    return image;
  }

  function statusImage(icon: MenuBarIconName) {
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

  function menuItemImage(icon: MenuBarIconName) {
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

  function clickCurrentEntry(id: string, path: number[]) {
    const entry = menuEntryAtPath(live.get(id)?.menuSpec.menu ?? [], path);
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
        withLiveClickHandlers(id, spec.menu),
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
      tray.setTitle(spec.title || '', { fontType: 'monospacedDigit' });
    tray.setToolTip(statusItemTooltip(spec, platform));
  }

  /** Rebuilding replaces an open native menu, so callers rebuild only when rows change. */
  function applyMenu(id: string, item: LiveStatusItem) {
    if (item.menuRebuildTimer) clearTimeout(item.menuRebuildTimer);
    item.menuRebuildTimer = null;
    item.menuBuiltAt = Date.now();
    item.menuSpec = item.spec;
    item.menuSignature = statusItemMenuSignature(item.spec);
    item.tray.setContextMenu(buildMenu(id, item.spec));
  }

  function createTray(id: string, spec: StatusItemSpec) {
    const tray = new Tray(statusImage(spec.icon));
    if (platform && opensMenuOnPrimaryClick(platform))
      tray.on('click', () => {
        deps.onMenuOpened?.(id);
        tray.popUpContextMenu();
      });
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
