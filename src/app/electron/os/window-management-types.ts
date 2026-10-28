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

export type WindowManagementOptions = {
  gap: number;
  cycle: boolean;
};

export const DEFAULT_WINDOW_MANAGEMENT_OPTIONS: WindowManagementOptions = {
  gap: 0,
  cycle: true,
};

export interface EngineInput {
  spec: WindowLayoutSpec;
  workArea: Frame;
  currentFrame: Frame;
  gap: number;
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

export interface WindowManagementService {
  available(): boolean;
  apply(
    spec: WindowLayoutSpec,
    options: WindowManagementOptions,
    invocation: WindowInvocation,
  ): Promise<WindowApplyResult>;
}

export interface WindowInvocationPorts {
  paletteFocusTarget(): WindowFocusTarget | null;
  hidePaletteAndRestoreFocus(target: WindowFocusTarget): Promise<boolean>;
  frontmostTarget(): Promise<WindowFocusTarget | null>;
}
