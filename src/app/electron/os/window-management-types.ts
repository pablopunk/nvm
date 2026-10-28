export interface Frame {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface DisplayInfo {
  id: string;
  isPrimary: boolean;
  frame: Frame;
  workArea: Frame;
}

export interface WindowInfo {
  bundleId: string;
  windowKey: string;
  title: string;
  frame: Frame;
  displayId: string;
  isFullScreen: boolean;
  isMinimized: boolean;
  isMovable: boolean;
  isResizable: boolean;
}

export type HelperCode =
  | 'ok'
  | 'denied'
  | 'no-window'
  | 'target-lost'
  | 'unsupported'
  | 'unapplied'
  | 'error';

export interface HelperEnvelope<Data> {
  ok: boolean;
  code: HelperCode;
  data?: Data;
  message?: string;
}

export type WriteOutcome = 'exact' | 'partial' | 'unapplied';

export interface SetFrameData {
  outcome: WriteOutcome;
  changed: boolean;
  frame: Frame;
  reason?: string;
}

export type WindowPermissionState = 'allowed' | 'denied' | 'unknown';

export interface SetFrameRequest {
  pid: number;
  windowKey: string;
  frame: Frame;
  positionOnly?: boolean;
}

export interface MacosWindowManager {
  permissionState(): Promise<WindowPermissionState>;
  displays(): Promise<HelperEnvelope<DisplayInfo[]>>;
  focusedWindow(pid: number): Promise<HelperEnvelope<WindowInfo>>;
  setFrame(request: SetFrameRequest): Promise<HelperEnvelope<SetFrameData>>;
  setFullScreen(
    pid: number,
    windowKey: string,
    fullScreen: boolean,
  ): Promise<HelperEnvelope<SetFrameData>>;
  runningApps(): Promise<HelperEnvelope<RunningApp[]>>;
  windows(pid: number): Promise<HelperEnvelope<WindowInfo[]>>;
  raiseWindow(pid: number, windowKey: string): Promise<HelperEnvelope<null>>;
}

export interface RunningApp {
  pid: number;
  bundleId: string;
  name: string;
}

export type WindowEdge = 'left' | 'right' | 'top' | 'bottom';

export type WindowDimension = {
  value: number;
  unit: 'points' | 'percent';
};

export type WindowCycleGroup = WindowEdge;

export type WindowLayoutSpec =
  | {
      kind: 'fraction';
      x: number;
      y: number;
      width: number;
      height: number;
      cycleGroup?: WindowCycleGroup;
    }
  | { kind: 'center' }
  | { kind: 'maximize-width' }
  | { kind: 'maximize-height' }
  | {
      kind: 'scaled-center';
      widthFraction: number;
      heightFraction: number;
      maxWidth?: number;
      maxHeight?: number;
    }
  | { kind: 'edge'; edge: WindowEdge }
  | { kind: 'resize-step'; direction: 'larger' | 'smaller'; step: number }
  | { kind: 'display'; direction: 'next' | 'previous' }
  | { kind: 'restore' }
  | { kind: 'toggle-full-screen' }
  | {
      kind: 'exact';
      x?: WindowDimension;
      y?: WindowDimension;
      width?: WindowDimension;
      height?: WindowDimension;
    };

export interface WindowEdgeGaps {
  top: number;
  right: number;
  bottom: number;
  left: number;
}

export type WindowCycleOrder =
  | 'half-two-thirds-third'
  | 'half-third-two-thirds';

export type WindowDisplayOrder = 'left-to-right' | 'top-to-bottom';

export type WindowDisplayMoveSizing = 'proportional' | 'keep-size';

export interface WindowManagementOptions {
  outerGap: WindowEdgeGaps;
  innerGap: number;
  applyGapToMaximize: boolean;
  cycle: boolean;
  cycleOrder: WindowCycleOrder;
  displayOrder: WindowDisplayOrder;
  displayMoveSizing: WindowDisplayMoveSizing;
}

export const MAX_WINDOW_GAP_POINTS = 40;

export const DEFAULT_WINDOW_MANAGEMENT_OPTIONS: WindowManagementOptions = {
  outerGap: { top: 8, right: 8, bottom: 8, left: 8 },
  innerGap: 4,
  applyGapToMaximize: true,
  cycle: true,
  cycleOrder: 'half-two-thirds-third',
  displayOrder: 'left-to-right',
  displayMoveSizing: 'proportional',
};

export interface WindowGaps {
  outer: WindowEdgeGaps;
  inner: number;
  applyToMaximize: boolean;
}

export interface EngineInput {
  spec: WindowLayoutSpec;
  workArea: Frame;
  currentFrame: Frame;
  gaps: WindowGaps;
}

export type ComputeTargetFrame = (input: EngineInput) => Frame;

export interface WindowFocusTarget {
  bundleId: string;
  pid: number;
}

export type WindowInvocation =
  | { source: 'palette'; target: WindowFocusTarget | null }
  | { source: 'shortcut'; target: WindowFocusTarget | null };

export type WindowApplyFailureReason =
  | 'no-target'
  | 'focus-not-restored'
  | 'no-window'
  | 'target-lost'
  | 'full-screen'
  | 'minimized'
  | 'denied'
  | 'unapplied'
  | 'not-resizable'
  | 'unsupported'
  | 'invalid-spec'
  | 'no-history'
  | 'unknown-window'
  | 'single-display';

export type WindowApplyResult =
  | {
      ok: true;
      outcome: 'exact' | 'partial';
      changed: boolean;
      frame: Frame;
      reason?: string;
    }
  | { ok: false; reason: WindowApplyFailureReason; message: string };

export interface WindowListQuery {
  appId?: string;
  title?: string;
}

export interface WindowDescriptor {
  handle: string;
  appId: string;
  appName: string;
  title: string;
  frame: Frame;
  displayId: string;
  isFullScreen: boolean;
  isMinimized: boolean;
}

export type WindowOperationFailure = {
  ok: false;
  reason: WindowApplyFailureReason;
  message: string;
};

export type WindowListResult =
  | { ok: true; windows: WindowDescriptor[] }
  | WindowOperationFailure;

export type DisplayListResult =
  | { ok: true; displays: DisplayInfo[] }
  | WindowOperationFailure;

export type WindowFocusResult = { ok: true } | WindowOperationFailure;

export interface WindowManagementService {
  available(): boolean;
  apply(
    spec: WindowLayoutSpec,
    options: WindowManagementOptions,
    invocation: WindowInvocation,
  ): Promise<WindowApplyResult>;
  listDisplays(): Promise<DisplayListResult>;
  listWindows(query: WindowListQuery): Promise<WindowListResult>;
  applyToWindow(
    handle: string,
    spec: WindowLayoutSpec,
    options: WindowManagementOptions,
  ): Promise<WindowApplyResult>;
  focusWindow(handle: string): Promise<WindowFocusResult>;
}

export interface WindowInvocationPorts {
  paletteFocusTarget(): WindowFocusTarget | null;
  hidePaletteAndRestoreFocus(target: WindowFocusTarget): Promise<boolean>;
  frontmostTarget(): Promise<WindowFocusTarget | null>;
}
