import { windowAccessFailure } from './window-management-access';
import { unwrapEnvelope } from './window-management-envelopes';
import type { WindowHandleRegistry } from './window-management-handles';
import type {
  DisplayListResult,
  MacosWindowManager,
  RunningApp,
  WindowDescriptor,
  WindowInfo,
  WindowListQuery,
  WindowListResult,
} from './window-management-types';

type WindowLister = Pick<
  MacosWindowManager,
  'permissionState' | 'displays' | 'runningApps' | 'windows'
>;

function matchesAppId(app: RunningApp, appId: string | undefined) {
  return (
    appId === undefined || app.bundleId.toLowerCase() === appId.toLowerCase()
  );
}

function matchesAppName(app: RunningApp, appName: string | undefined) {
  return !appName || app.name.toLowerCase().includes(appName.toLowerCase());
}

function matchesTitle(window: WindowInfo, title: string | undefined) {
  return !title || window.title.toLowerCase().includes(title.toLowerCase());
}

function describeListedWindow(
  handles: WindowHandleRegistry,
  app: RunningApp,
  window: WindowInfo,
): WindowDescriptor {
  const handle = handles.issue({
    pid: app.pid,
    appId: app.bundleId,
    windowKey: window.windowKey,
  });
  return {
    handle,
    appId: app.bundleId,
    appName: app.name,
    title: window.title,
    frame: { ...window.frame },
    displayId: window.displayId,
    isFullScreen: window.isFullScreen,
    isMinimized: window.isMinimized,
  };
}

async function listWindowsOfApp(
  manager: WindowLister,
  handles: WindowHandleRegistry,
  app: RunningApp,
  query: WindowListQuery,
): Promise<WindowDescriptor[]> {
  const listed = unwrapEnvelope(await manager.windows(app.pid));
  if (listed.ok === false) {
    return [];
  }
  return listed.value
    .filter((window) => matchesTitle(window, query.title))
    .map((window) => describeListedWindow(handles, app, window));
}

export const MAX_LISTED_APPS = 40;

export async function listDisplaysFrom(
  manager: WindowLister,
): Promise<DisplayListResult> {
  const denied = await windowAccessFailure(manager);
  if (denied) {
    return denied;
  }
  const displays = unwrapEnvelope(await manager.displays());
  if (displays.ok === false) {
    return displays.failure;
  }
  return { ok: true, displays: displays.value };
}

export async function listWindowsFrom(
  manager: WindowLister,
  handles: WindowHandleRegistry,
  query: WindowListQuery,
): Promise<WindowListResult> {
  const denied = await windowAccessFailure(manager);
  if (denied) {
    return denied;
  }
  const apps = unwrapEnvelope(await manager.runningApps());
  if (apps.ok === false) {
    return apps.failure;
  }
  const matchingApps = apps.value
    .filter(
      (app) =>
        matchesAppId(app, query.appId) && matchesAppName(app, query.appName),
    )
    .slice(0, MAX_LISTED_APPS);
  const windows: WindowDescriptor[] = [];
  for (const app of matchingApps) {
    // biome-ignore lint/performance/noAwaitInLoops: apps are read one at a time so a slow app cannot fan out helper processes
    windows.push(...(await listWindowsOfApp(manager, handles, app, query)));
  }
  return { ok: true, windows };
}
