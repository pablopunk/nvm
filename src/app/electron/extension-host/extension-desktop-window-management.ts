import {
  type PartialWindowManagementOptions,
  resolveWindowManagementOptions,
} from '../os/window-management-options';
import type {
  DisplayInfo,
  Frame,
  WindowApplyResult,
  WindowDescriptor,
  WindowInvocation,
  WindowLayoutSpec,
  WindowListQuery,
  WindowManagementService,
} from '../os/window-management-types';
import {
  currentWindowInvocationScope,
  type WindowInvocationScope,
} from './window-invocation-scope';

interface DesktopWindowManagementDeps {
  service: WindowManagementService;
  resolveInvocation: (
    scope: WindowInvocationScope | undefined,
  ) => WindowInvocation | null;
}

interface ExtensionWindowQuery {
  appId?: string;
  appName?: string;
  title?: string;
}

interface ExtensionWindowHandleInput {
  id: string;
}

const MISSING_INVOCATION_MESSAGE =
  'Window commands run from the palette or a keyboard shortcut.';

const UNKNOWN_WINDOW_MESSAGE =
  'Pass a window handle from windows() or waitForWindow().';

const DEFAULT_WAIT_TIMEOUT_MS = 5000;

const MAX_WAIT_TIMEOUT_MS = 30_000;

const WAIT_POLL_INTERVAL_MS = 250;

async function applyWindowLayoutForCurrentScope(
  deps: DesktopWindowManagementDeps,
  spec: WindowLayoutSpec,
  options: PartialWindowManagementOptions = {},
) {
  const invocation = deps.resolveInvocation(currentWindowInvocationScope());
  if (!invocation) {
    return missingInvocationResult();
  }
  const result = await deps.service.apply(
    spec,
    resolveWindowManagementOptions(options),
    invocation,
  );
  return toExtensionApplyResult(result);
}

function missingInvocationResult() {
  return {
    ok: false,
    reason: 'no-target',
    message: MISSING_INVOCATION_MESSAGE,
  };
}

function unknownWindowResult() {
  return {
    ok: false,
    reason: 'unknown-window',
    message: UNKNOWN_WINDOW_MESSAGE,
  };
}

function pickFrame(frame: Frame): Frame {
  return {
    x: frame.x,
    y: frame.y,
    width: frame.width,
    height: frame.height,
  };
}

function toExtensionApplyResult(result: WindowApplyResult) {
  if (result.ok === false) {
    return { ok: false, reason: result.reason, message: result.message };
  }
  return {
    ok: true,
    outcome: result.outcome,
    changed: result.changed,
    frame: pickFrame(result.frame),
    ...(result.reason ? { reason: result.reason } : {}),
  };
}

function toExtensionDisplay(display: DisplayInfo) {
  return {
    id: display.id,
    isPrimary: display.isPrimary,
    frame: pickFrame(display.frame),
    workArea: pickFrame(display.workArea),
  };
}

function toExtensionWindowInfo(window: WindowDescriptor) {
  return {
    handle: { id: window.handle },
    appId: window.appId,
    appName: window.appName,
    title: window.title,
    frame: pickFrame(window.frame),
    displayId: window.displayId,
    isFullScreen: window.isFullScreen,
    isMinimized: window.isMinimized,
  };
}

type ExtensionWindowInfo = ReturnType<typeof toExtensionWindowInfo>;

function windowListQueryFrom(query?: ExtensionWindowQuery): WindowListQuery {
  return {
    ...(typeof query?.appId === 'string' ? { appId: query.appId } : {}),
    ...(typeof query?.appName === 'string' ? { appName: query.appName } : {}),
    ...(typeof query?.title === 'string' ? { title: query.title } : {}),
  };
}

function handleIdOf(window: unknown): string | null {
  if (typeof window !== 'object' || window === null) {
    return null;
  }
  const id = (window as Partial<ExtensionWindowHandleInput>).id;
  return typeof id === 'string' && id.length > 0 ? id : null;
}

async function listExtensionDisplays(deps: DesktopWindowManagementDeps) {
  const result = await deps.service.listDisplays();
  return result.ok === false ? [] : result.displays.map(toExtensionDisplay);
}

async function listExtensionWindows(
  deps: DesktopWindowManagementDeps,
  query?: ExtensionWindowQuery,
) {
  const result = await deps.service.listWindows(windowListQueryFrom(query));
  return result.ok === false ? [] : result.windows.map(toExtensionWindowInfo);
}

function clampWaitTimeoutMs(timeoutMs: number | undefined) {
  if (typeof timeoutMs !== 'number' || !Number.isFinite(timeoutMs)) {
    return DEFAULT_WAIT_TIMEOUT_MS;
  }
  return Math.min(Math.max(timeoutMs, 0), MAX_WAIT_TIMEOUT_MS);
}

function waitMilliseconds(milliseconds: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, milliseconds));
}

function waitForExtensionWindow(
  deps: DesktopWindowManagementDeps,
  query?: ExtensionWindowQuery,
  options?: { timeoutMs?: number },
) {
  const deadline = Date.now() + clampWaitTimeoutMs(options?.timeoutMs);
  return pollForExtensionWindow(deps, query, deadline);
}

async function pollForExtensionWindow(
  deps: DesktopWindowManagementDeps,
  query: ExtensionWindowQuery | undefined,
  deadline: number,
): Promise<ExtensionWindowInfo | null> {
  const [firstMatch] = await listExtensionWindows(deps, query);
  if (firstMatch || Date.now() >= deadline) {
    return firstMatch ?? null;
  }
  await waitMilliseconds(
    Math.min(WAIT_POLL_INTERVAL_MS, deadline - Date.now()),
  );
  return pollForExtensionWindow(deps, query, deadline);
}

async function applyLayoutToExtensionWindow(
  deps: DesktopWindowManagementDeps,
  window: ExtensionWindowHandleInput,
  spec: WindowLayoutSpec,
  options: PartialWindowManagementOptions = {},
) {
  const handleId = handleIdOf(window);
  if (handleId === null) {
    return unknownWindowResult();
  }
  const result = await deps.service.applyToWindow(
    handleId,
    spec,
    resolveWindowManagementOptions(options),
  );
  return toExtensionApplyResult(result);
}

async function focusExtensionWindow(
  deps: DesktopWindowManagementDeps,
  window: ExtensionWindowHandleInput,
) {
  const handleId = handleIdOf(window);
  if (handleId === null) {
    return unknownWindowResult();
  }
  const result = await deps.service.focusWindow(handleId);
  if (result.ok === false) {
    return { ok: false, reason: result.reason, message: result.message };
  }
  return { ok: true };
}

function createDesktopWindowManagement(
  deps: DesktopWindowManagementDeps | undefined,
) {
  if (!deps?.service.available()) {
    return;
  }
  return {
    apply: (spec: WindowLayoutSpec, options?: PartialWindowManagementOptions) =>
      applyWindowLayoutForCurrentScope(deps, spec, options),
    displays: () => listExtensionDisplays(deps),
    windows: (query?: ExtensionWindowQuery) =>
      listExtensionWindows(deps, query),
    waitForWindow: (
      query: ExtensionWindowQuery,
      options?: { timeoutMs?: number },
    ) => waitForExtensionWindow(deps, query, options),
    applyTo: (
      window: ExtensionWindowHandleInput,
      spec: WindowLayoutSpec,
      options?: PartialWindowManagementOptions,
    ) => applyLayoutToExtensionWindow(deps, window, spec, options),
    focus: (window: ExtensionWindowHandleInput) =>
      focusExtensionWindow(deps, window),
  };
}

export type { DesktopWindowManagementDeps, ExtensionWindowQuery };
export { clampWaitTimeoutMs, createDesktopWindowManagement };
