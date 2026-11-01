// biome-ignore-all lint: Menu bar rows carry dynamic extension-owned actions into the host executor.
import {
  extensionMenuBarRenderer,
  menuBarStatusItemId,
  type NormalizedMenuBar,
  type NormalizedMenuBarRow,
  normalizeExtensionMenuBars,
} from '../extension-host/extension-menubar';
import type {
  MenuBarStatusItems,
  StatusItemMenuEntry,
  StatusItemSpec,
} from '../os/menubar-status-items';
import { error as logError, warn as logWarn } from '../platform/logger';
import {
  isPromiseTimeoutError,
  withAbortableTimeout,
} from '../platform/promise-timeout';

const SYNC_DEBOUNCE_MS = 150;

export type ExtensionMenuBarServiceDeps = {
  extensionModules: Map<string, any>;
  statusItems: Pick<MenuBarStatusItems, 'supported' | 'upsert' | 'retainOnly'>;
  createExtensionContext: (extension: any, command: any) => any;
  extensionRootActionFromItem: (entry: any, item: any) => any;
  runMenuAction: (rootAction: any) => unknown;
  renderTimeoutMs: number;
  isVisibleExtension?: (extension: any) => boolean;
};

type RenderedMenuBars = {
  extension: any;
  menuBars: NormalizedMenuBar[];
};

export function menuRowRunsWithoutView(action: any) {
  return (
    action?.dismissAfterRun === 'auto' ||
    action?.background === true ||
    action?.mode === 'background' ||
    action?.mode === 'noView'
  );
}

function menuBarEntry(extension: any) {
  return {
    extension,
    command: { id: 'menubar', title: extension.title || extension.id },
  };
}

export function createExtensionMenuBarService(
  deps: ExtensionMenuBarServiceDeps,
) {
  const rendered = new Map<string, RenderedMenuBars>();
  const inFlight = new Map<string, Promise<void>>();
  const statusItemOwners = new Map<string, string>();
  let syncTimer: ReturnType<typeof setTimeout> | undefined;

  function menuBarExtensions() {
    return Array.from(deps.extensionModules.values()).filter(
      (extension) =>
        extensionMenuBarRenderer(extension) &&
        (deps.isVisibleExtension?.(extension) ?? true),
    );
  }

  function menuEntriesForRows(
    state: RenderedMenuBars,
    menuBarId: string,
    rows: NormalizedMenuBarRow[],
    prefix: string,
  ): StatusItemMenuEntry[] {
    return rows.map((row) => {
      const rowPath = prefix ? `${prefix}/${row.id}` : row.id;
      const runnable = Boolean(row.action) && !row.items;
      return {
        type: 'item' as const,
        label: row.title,
        ...(row.subtitle ? { sublabel: row.subtitle } : {}),
        ...(row.icon ? { icon: row.icon } : {}),
        ...(row.checked !== undefined ? { checked: row.checked } : {}),
        ...(row.shortcut ? { accelerator: row.shortcut } : {}),
        enabled: !row.disabled && (runnable || Boolean(row.items)),
        ...(row.items
          ? {
              submenu: menuEntriesForRows(state, menuBarId, row.items, rowPath),
            }
          : {}),
        ...(runnable
          ? {
              onClick: () =>
                runRowAction(state.extension, menuBarId, rowPath, row),
            }
          : {}),
      };
    });
  }

  function statusItemSpec(
    state: RenderedMenuBars,
    menuBar: NormalizedMenuBar,
  ): StatusItemSpec {
    const menu: StatusItemMenuEntry[] = [];
    menuBar.sections.forEach((section, index) => {
      if (index > 0) menu.push({ type: 'separator' });
      if (section.title) menu.push({ type: 'header', label: section.title });
      menu.push(...menuEntriesForRows(state, menuBar.id, section.items, ''));
    });
    return {
      icon: menuBar.icon,
      ...(menuBar.title ? { title: menuBar.title } : {}),
      tooltip: menuBar.tooltip,
      menu,
    };
  }

  function logActionFailure(extension: any, error: unknown) {
    logError('menubar.action.failed', error, {
      source: 'host',
      scope: 'extension',
      extensionId: extension?.id,
    });
  }

  /** Binds lazily so frequent refreshes never register handlers that are never clicked. */
  function runRowAction(
    extension: any,
    menuBarId: string,
    rowPath: string,
    row: NormalizedMenuBarRow,
  ) {
    try {
      const rootAction = deps.extensionRootActionFromItem(
        menuBarEntry(extension),
        {
          id: `menubar:${menuBarId}:${rowPath}`,
          title: row.title,
          primaryAction: row.action,
          ...(menuRowRunsWithoutView(row.action) ? { mode: 'noView' } : {}),
        },
      );
      if (!rootAction) return;
      void Promise.resolve(deps.runMenuAction(rootAction)).catch((error) =>
        logActionFailure(extension, error),
      );
    } catch (error) {
      logActionFailure(extension, error);
    }
  }

  function applyStatusItems() {
    const live: string[] = [];
    statusItemOwners.clear();
    for (const extension of menuBarExtensions()) {
      const state = rendered.get(extension.id);
      if (!state || state.extension !== extension) continue;
      for (const menuBar of state.menuBars) {
        const id = menuBarStatusItemId(extension.id, menuBar.id);
        if (!deps.statusItems.upsert(id, statusItemSpec(state, menuBar)))
          continue;
        live.push(id);
        statusItemOwners.set(id, extension.id);
      }
    }
    deps.statusItems.retainOnly(live);
  }

  function logIssues(
    extension: any,
    issues: Array<{ message: string; menuBarId?: string }>,
  ) {
    for (const issue of issues)
      logWarn('menubar.descriptor.invalid', issue, {
        source: 'host',
        scope: 'extension',
        extensionId: extension.id,
      });
  }

  async function renderExtension(extension: any) {
    const render = extensionMenuBarRenderer(extension);
    if (!render) return;
    try {
      const ctx = deps.createExtensionContext(
        extension,
        menuBarEntry(extension).command,
      );
      const result = await withAbortableTimeout(
        render.call(extension.menuBar, ctx),
        deps.renderTimeoutMs,
      );
      if (deps.extensionModules.get(extension.id) !== extension) return;
      const { menuBars, issues } = normalizeExtensionMenuBars(
        extension,
        result,
      );
      logIssues(extension, issues);
      rendered.set(extension.id, { extension, menuBars });
    } catch (error) {
      logError(
        isPromiseTimeoutError(error)
          ? 'menubar.render.timeout'
          : 'menubar.render.failed',
        error,
        { source: 'host', scope: 'extension', extensionId: extension.id },
      );
    }
  }

  function refresh(extensionId: string) {
    if (!deps.statusItems.supported) return Promise.resolve();
    const current = inFlight.get(extensionId);
    if (current) return current;
    const extension = deps.extensionModules.get(extensionId);
    if (!extension) return Promise.resolve();
    const pending = renderExtension(extension)
      .then(applyStatusItems)
      .finally(() => inFlight.delete(extensionId));
    inFlight.set(extensionId, pending);
    return pending;
  }

  async function syncAll() {
    if (!deps.statusItems.supported) return;
    const extensions = menuBarExtensions();
    const liveIds = new Set(extensions.map((extension) => extension.id));
    for (const id of Array.from(rendered.keys()))
      if (!liveIds.has(id)) rendered.delete(id);
    await Promise.all(extensions.map((extension) => refresh(extension.id)));
    applyStatusItems();
  }

  function scheduleSync() {
    if (syncTimer) clearTimeout(syncTimer);
    syncTimer = setTimeout(() => {
      syncTimer = undefined;
      void syncAll();
    }, SYNC_DEBOUNCE_MS);
    syncTimer.unref?.();
  }

  function refreshForStatusItem(statusItemId: string) {
    const extensionId = statusItemOwners.get(statusItemId);
    if (extensionId) void refresh(extensionId);
  }

  return { refresh, syncAll, scheduleSync, refreshForStatusItem };
}

export type ExtensionMenuBarService = ReturnType<
  typeof createExtensionMenuBarService
>;
