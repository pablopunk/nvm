import type {
  Frame,
  HelperEnvelope,
  ListedWindow,
  SetFrameData,
  SetFrameRequest,
  WindowInfo,
} from './window-management-types';

const SIMULATED_BUNDLE_ID = 'com.apple.finder';
const SIMULATED_WINDOW_KEY = '42:7';
const SIMULATED_PID = 42;
const SIMULATED_APP_NAME = 'Simulated app';

function okEnvelope<Data>(data: Data): HelperEnvelope<Data> {
  return { ok: true, code: 'ok', data };
}

function sameFrame(first: Frame, second: Frame) {
  return (
    first.x === second.x &&
    first.y === second.y &&
    first.width === second.width &&
    first.height === second.height
  );
}

function simulatedWindowInfo(state: SimulatedWindowState): WindowInfo {
  return {
    bundleId: SIMULATED_BUNDLE_ID,
    windowKey: SIMULATED_WINDOW_KEY,
    title: 'Simulated window',
    frame: state.frame,
    displayId: 'main',
    isFullScreen: state.isFullScreen,
    isMinimized: state.isMinimized,
    isMovable: true,
    isResizable: state.isResizable,
  };
}

export interface SimulatedWindowState {
  frame: Frame;
  isFullScreen: boolean;
  isMinimized: boolean;
  isResizable: boolean;
}

export interface SimulatedWindowOptions extends Partial<SimulatedWindowState> {
  fullScreenResponse?: HelperEnvelope<SetFrameData>;
}

export interface SimulatedWindow {
  state: SimulatedWindowState;
  focusedRead(): Promise<HelperEnvelope<WindowInfo>>;
  windowsRead(): Promise<HelperEnvelope<WindowInfo[]>>;
  allWindowsRead(): Promise<HelperEnvelope<ListedWindow[]>>;
  setFrame(request: SetFrameRequest): Promise<HelperEnvelope<SetFrameData>>;
  setFullScreen(
    pid: number,
    windowKey: string,
    fullScreen: boolean,
  ): Promise<HelperEnvelope<SetFrameData>>;
}

export function createSimulatedWindow(
  options: SimulatedWindowOptions,
): SimulatedWindow {
  const { fullScreenResponse, ...initial } = options;
  const state: SimulatedWindowState = {
    frame: initial.frame ?? { x: 100, y: 100, width: 600, height: 400 },
    isFullScreen: initial.isFullScreen ?? false,
    isMinimized: initial.isMinimized ?? false,
    isResizable: initial.isResizable ?? true,
  };
  return {
    state,
    focusedRead: () => Promise.resolve(okEnvelope(simulatedWindowInfo(state))),
    windowsRead: () =>
      Promise.resolve(okEnvelope([simulatedWindowInfo(state)])),
    allWindowsRead: () =>
      Promise.resolve(
        okEnvelope([
          {
            ...simulatedWindowInfo(state),
            pid: SIMULATED_PID,
            appName: SIMULATED_APP_NAME,
            zIndex: 0,
          },
        ]),
      ),
    setFrame(request) {
      const changed = !sameFrame(state.frame, request.frame);
      state.frame = request.frame;
      return Promise.resolve(
        okEnvelope({
          outcome: 'exact',
          changed,
          frame: request.frame,
        }),
      );
    },
    setFullScreen(_pid, _windowKey, fullScreen) {
      if (fullScreenResponse) {
        return Promise.resolve(fullScreenResponse);
      }
      state.isFullScreen = fullScreen;
      return Promise.resolve(
        okEnvelope({ outcome: 'exact', changed: true, frame: state.frame }),
      );
    },
  };
}
