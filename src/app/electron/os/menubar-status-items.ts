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
  signature: string;
  menuOpen: boolean;
  pendingSpec: StatusItemSpec | null;
};

export function menuBarStatusItemsPlatform(
  platform: NodeJS.Platform,
): StatusItemPlatform | null {
  return platform === 'darwin' || platform === 'win32' || platform === 'linux'
    ? platform
    : null;
}

export function statusItemSpecSignature(spec: StatusItemSpec) {
  return JSON.stringify(spec, (_key, value) =>
    typeof value === 'function' ? undefined : value,
  );
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
    const pending = item.pendingSpec;
    item.pendingSpec = null;
    if (pending) upsert(id, pending);
  }

  function buildMenu(id: string, spec: StatusItemSpec) {
    if (!platform) throw new Error('Status items are not supported');
    const menu = Menu.buildFromTemplate(
      statusItemMenuTemplate(spec.menu, platform, menuItemImage),
    );
    menu.on('menu-will-show', () => markMenuOpen(id));
    menu.on('menu-will-close', () =>
      setTimeout(() => applyPendingAfterMenuCloses(id), 0),
    );
    return menu;
  }

  function applySpec(id: string, item: LiveStatusItem) {
    if (!platform) return;
    const { tray, spec } = item;
    tray.setImage(statusImage(spec.icon));
    if (supportsStatusTitle(platform))
      tray.setTitle(spec.title || '', { fontType: 'monospacedDigit' });
    tray.setToolTip(statusItemTooltip(spec, platform));
    tray.setContextMenu(buildMenu(id, spec));
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

  function upsert(id: string, spec: StatusItemSpec) {
    if (!platform) return false;
    const signature = statusItemSpecSignature(spec);
    const existing = live.get(id);
    if (existing?.menuOpen) {
      existing.pendingSpec = spec;
      return true;
    }
    if (existing) {
      existing.spec = spec;
      if (existing.signature === signature) {
        existing.tray.setContextMenu(buildMenu(id, spec));
        return true;
      }
      existing.signature = signature;
      applySpec(id, existing);
      return true;
    }
    const item = {
      tray: createTray(id, spec),
      spec,
      signature,
      menuOpen: false,
      pendingSpec: null,
    };
    live.set(id, item);
    applySpec(id, item);
    return true;
  }

  function remove(id: string) {
    const item = live.get(id);
    if (!item) return;
    live.delete(id);
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
