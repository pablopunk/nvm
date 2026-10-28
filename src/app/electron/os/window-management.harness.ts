import { createWindowManagementService } from './window-management';
import type {
  DisplayInfo,
  EngineInput,
  Frame,
  HelperEnvelope,
  MacosWindowManager,
  SetFrameData,
  SetFrameRequest,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocation,
  WindowInvocationPorts,
  WindowLayoutSpec,
  WindowPermissionState,
} from './window-management-types';

const MENU_BAR_HEIGHT = 25;
const SCREEN_WIDTH = 1440;
const SCREEN_HEIGHT = 900;
const WORK_AREA_HEIGHT = SCREEN_HEIGHT - MENU_BAR_HEIGHT;
const ORIGINAL_X = 100;
const ORIGINAL_Y = 100;
const ORIGINAL_WIDTH = 600;
const ORIGINAL_HEIGHT = 400;
const FINDER_PID = 42;
const OTHER_PID_VALUE = 43;
const UNRELATED_PID = 99;

const WORK_AREA: Frame = {
  x: 0,
  y: MENU_BAR_HEIGHT,
  width: SCREEN_WIDTH,
  height: WORK_AREA_HEIGHT,
};
const DISPLAY: DisplayInfo = {
  id: 'main',
  isPrimary: true,
  frame: { x: 0, y: 0, width: SCREEN_WIDTH, height: SCREEN_HEIGHT },
  workArea: WORK_AREA,
};
const FINDER: WindowFocusTarget = {
  bundleId: 'com.apple.finder',
  pid: FINDER_PID,
};
const OTHER_APP: WindowFocusTarget = {
  bundleId: 'com.other.app',
  pid: OTHER_PID_VALUE,
};
const UNRELATED_FRONTMOST: WindowFocusTarget = {
  bundleId: 'com.apple.Terminal',
  pid: UNRELATED_PID,
};
const ORIGINAL_FRAME: Frame = {
  x: ORIGINAL_X,
  y: ORIGINAL_Y,
  width: ORIGINAL_WIDTH,
  height: ORIGINAL_HEIGHT,
};
const LEFT_HALF: WindowLayoutSpec = {
  kind: 'fraction',
  x: 0,
  y: 0,
  width: 0.5,
  height: 1,
};
const REQUESTED_LEFT_HALF: Frame = {
  x: 0,
  y: MENU_BAR_HEIGHT,
  width: SCREEN_WIDTH / 2,
  height: WORK_AREA_HEIGHT,
};
const NO_OPTIONS = { gap: 0 };

function paletteInvocation(
  target: WindowFocusTarget | null = FINDER,
): WindowInvocation {
  return { source: 'palette', target };
}

function shortcutInvocation(
  target: WindowFocusTarget | null = FINDER,
): WindowInvocation {
  return { source: 'shortcut', target };
}

function ok<Data>(data: Data): HelperEnvelope<Data> {
  return { ok: true, code: 'ok', data };
}

function fail<Data>(
  code: HelperEnvelope<Data>['code'],
  message: string = code,
): HelperEnvelope<Data> {
  return { ok: false, code, message };
}

function writeOk(
  frame: Frame,
  outcome: SetFrameData['outcome'] = 'exact',
  reason?: string,
): HelperEnvelope<SetFrameData> {
  return ok({ outcome, changed: true, frame, ...(reason ? { reason } : {}) });
}

function focusedWindow(overrides: Partial<WindowInfo> = {}): WindowInfo {
  return {
    bundleId: FINDER.bundleId,
    windowKey: '42:7',
    title: 'Notes',
    frame: ORIGINAL_FRAME,
    displayId: 'main',
    isFullScreen: false,
    isMinimized: false,
    isMovable: true,
    isResizable: true,
    ...overrides,
  };
}

function bundleIdForPid(pid: number) {
  return pid === OTHER_APP.pid ? OTHER_APP.bundleId : FINDER.bundleId;
}

const engine = {
  computeTargetFrame({ spec, workArea, currentFrame }: EngineInput): Frame {
    if (spec.kind === 'center') {
      return {
        x: workArea.x + (workArea.width - currentFrame.width) / 2,
        y: workArea.y + (workArea.height - currentFrame.height) / 2,
        width: currentFrame.width,
        height: currentFrame.height,
      };
    }
    return {
      x: workArea.x + spec.x * workArea.width,
      y: workArea.y + spec.y * workArea.height,
      width: spec.width * workArea.width,
      height: spec.height * workArea.height,
    };
  },
  pickDisplayForFrame: (displays: DisplayInfo[]) => displays[0] ?? DISPLAY,
};

interface HarnessOptions {
  supported?: boolean;
  permission?: WindowPermissionState;
  focusedRead?: (pid: number) => Promise<HelperEnvelope<WindowInfo>>;
  displays?: () => Promise<HelperEnvelope<DisplayInfo[]>>;
  writes?: HelperEnvelope<SetFrameData>[];
  restored?: boolean;
  frontmost?: WindowFocusTarget | null;
  frontmostAfterRestore?: (restored: WindowFocusTarget) => WindowFocusTarget;
}

function createFakeManager(
  harnessOptions: HarnessOptions,
  calls: string[],
  setFrameRequests: SetFrameRequest[],
): MacosWindowManager {
  const writes = [...(harnessOptions.writes ?? [])];
  return {
    permissionState() {
      calls.push('permissions');
      return Promise.resolve(harnessOptions.permission ?? 'allowed');
    },
    displays() {
      calls.push('displays');
      return harnessOptions.displays
        ? harnessOptions.displays()
        : Promise.resolve(ok([DISPLAY]));
    },
    focusedWindow(pid) {
      calls.push(`focused:${pid}`);
      if (harnessOptions.focusedRead) {
        return harnessOptions.focusedRead(pid);
      }
      return Promise.resolve(
        ok(focusedWindow({ bundleId: bundleIdForPid(pid) })),
      );
    },
    setFrame(request) {
      calls.push('setFrame');
      setFrameRequests.push(request);
      return Promise.resolve(writes.shift() ?? writeOk(request.frame));
    },
    setFullScreen() {
      return Promise.reject(
        new Error('setFullScreen is not part of phase one'),
      );
    },
  };
}

function createFakePorts(
  harnessOptions: HarnessOptions,
  calls: string[],
): WindowInvocationPorts {
  let frontmost =
    harnessOptions.frontmost === undefined ? FINDER : harnessOptions.frontmost;
  return {
    paletteFocusTarget: () => FINDER,
    hidePaletteAndRestoreFocus(restoreTarget) {
      calls.push(`restore:${restoreTarget.pid}`);
      const restored = harnessOptions.restored ?? true;
      if (restored) {
        frontmost = harnessOptions.frontmostAfterRestore
          ? harnessOptions.frontmostAfterRestore(restoreTarget)
          : restoreTarget;
      }
      return Promise.resolve(restored);
    },
    frontmostTarget() {
      calls.push('frontmost');
      return Promise.resolve(frontmost);
    },
  };
}

function createHarness(harnessOptions: HarnessOptions = {}) {
  const calls: string[] = [];
  const setFrameRequests: SetFrameRequest[] = [];
  const service = createWindowManagementService({
    manager: createFakeManager(harnessOptions, calls, setFrameRequests),
    ports: createFakePorts(harnessOptions, calls),
    engine,
    supported: () => harnessOptions.supported ?? true,
  });
  return { service, calls, setFrameRequests };
}

const OTHER_PID = OTHER_APP.pid;

export type { HarnessOptions };
export {
  createHarness,
  DISPLAY,
  FINDER,
  fail,
  focusedWindow,
  LEFT_HALF,
  NO_OPTIONS,
  ORIGINAL_FRAME,
  OTHER_APP,
  OTHER_PID,
  ok,
  paletteInvocation,
  REQUESTED_LEFT_HALF,
  shortcutInvocation,
  UNRELATED_FRONTMOST,
  WORK_AREA,
  writeOk,
};
