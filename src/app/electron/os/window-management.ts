import type {
  ComputeTargetFrame,
  DisplayInfo,
  Frame,
  HelperCode,
  HelperEnvelope,
  MacosWindowManager,
  SetFrameData,
  WindowApplyFailureReason,
  WindowApplyResult,
  WindowFocusTarget,
  WindowInfo,
  WindowInvocation,
  WindowInvocationPorts,
  WindowLayoutSpec,
  WindowManagementOptions,
  WindowManagementService,
  WriteOutcome,
} from './window-management-types';

interface WindowManagementServiceDependencies {
  manager: MacosWindowManager;
  ports: WindowInvocationPorts;
  engine: {
    computeTargetFrame: ComputeTargetFrame;
    pickDisplayForFrame(displays: DisplayInfo[], frame: Frame): DisplayInfo;
  };
  supported(): boolean;
  selfPid?: number;
}

type Engine = WindowManagementServiceDependencies['engine'];
type FailedResult = Extract<WindowApplyResult, { ok: false }>;
type Settled<Value> =
  | { ok: true; value: Value }
  | { ok: false; failure: FailedResult };
type FractionSpec = Extract<WindowLayoutSpec, { kind: 'fraction' }>;
interface WritableWindow {
  focused: WindowInfo;
  workArea: Frame;
}
interface ApplyRequest {
  spec: WindowLayoutSpec;
  options: WindowManagementOptions;
  source: WindowInvocation['source'];
  target: WindowFocusTarget;
}

const EXACT_TOLERANCE_POINTS = 2;
const FRACTION_EDGE_EPSILON = 1e-6;
const MAX_GAP_POINTS = 100;

const FAILURE_MESSAGES: Record<WindowApplyFailureReason, string> = {
  'no-target': 'No target app was captured. Reopen Nevermind and try again.',
  'focus-not-restored':
    'Could not return focus to the target app. No window was changed.',
  'no-window': 'The target app has no window to move.',
  'target-lost': 'The target window is no longer available.',
  'full-screen': 'Exit full screen before moving this window.',
  minimized: 'Restore the window before moving it.',
  denied:
    'Accessibility access is blocked. Open Nevermind OS Permissions, enable access, then restart Nevermind.',
  unapplied: 'The window did not accept the new frame.',
  'not-resizable': 'This window cannot be resized.',
  unsupported: 'Window management is not supported on this Mac.',
  'invalid-spec': 'The window layout request is invalid.',
};
const PALETTE_NO_TARGET =
  'No target app was captured. Reopen Nevermind from the app you want to move, then try again.';
const PALETTE_FOCUS_NOT_RESTORED =
  'Could not return focus to the target app. No window was changed.';
const SHORTCUT_NO_TARGET =
  'No frontmost app was available when the shortcut ran.';
const SHORTCUT_FOCUS_CHANGED =
  'The frontmost app changed before the window was moved. No window was changed.';
const TARGET_WINDOW_CHANGED =
  'The focused window now belongs to a different app.';
const NO_DISPLAY_MESSAGE = 'No display is available to place the window on.';
const INVALID_SPEC_MESSAGE =
  'The window layout must be a fraction inside the display, with a positive width and height.';
const INVALID_GAP_MESSAGE = `The window gap must be a whole number from 0 to ${MAX_GAP_POINTS}.`;
const CORRECTION_FAILED_REASON =
  'Part of the window stayed outside its display.';

const HELPER_FAILURE_REASONS: Partial<
  Record<HelperCode, WindowApplyFailureReason>
> = {
  denied: 'denied',
  'no-window': 'no-window',
  'target-lost': 'target-lost',
  unsupported: 'unsupported',
};

function failure(
  reason: WindowApplyFailureReason,
  message: string = FAILURE_MESSAGES[reason],
): FailedResult {
  return { ok: false, reason, message };
}

const SELF_TARGET_MESSAGE =
  'Nevermind is in front. Focus the window you want to move, then run the command again.';

function noTargetMessage(source: WindowInvocation['source']) {
  return source === 'palette' ? PALETTE_NO_TARGET : SHORTCUT_NO_TARGET;
}

function ignoreSettled() {
  return;
}

function releaseQueueIfIdle(
  queues: Map<number, Promise<void>>,
  pid: number,
  tail: Promise<void>,
) {
  if (queues.get(pid) === tail) {
    queues.delete(pid);
  }
}

function enqueueForPid<Result>(
  queues: Map<number, Promise<void>>,
  pid: number,
  task: () => Promise<Result>,
): Promise<Result> {
  const run = (queues.get(pid) ?? Promise.resolve()).then(task);
  const tail = run.then(ignoreSettled, ignoreSettled);
  queues.set(pid, tail);
  tail.then(() => releaseQueueIfIdle(queues, pid, tail));
  return run;
}

function isValidGap(gap: number) {
  return Number.isInteger(gap) && gap >= 0 && gap <= MAX_GAP_POINTS;
}

function isValidFractionRect(spec: FractionSpec) {
  const { x, y, width, height } = spec;
  return (
    [x, y, width, height].every(Number.isFinite) &&
    x >= 0 &&
    y >= 0 &&
    x <= 1 &&
    y <= 1 &&
    width > 0 &&
    width <= 1 &&
    height > 0 &&
    height <= 1 &&
    x + width <= 1 + FRACTION_EDGE_EPSILON &&
    y + height <= 1 + FRACTION_EDGE_EPSILON
  );
}

function describeInvalidRequest(
  spec: WindowLayoutSpec,
  options: WindowManagementOptions,
) {
  if (!isValidGap(options.gap)) {
    return INVALID_GAP_MESSAGE;
  }
  if (spec.kind === 'center') {
    return null;
  }
  return isValidFractionRect(spec) ? null : INVALID_SPEC_MESSAGE;
}

function clampOffsetInside(
  offset: number,
  size: number,
  start: number,
  extent: number,
) {
  return Math.max(start, Math.min(offset, start + extent - size));
}

function clampFrameInside(frame: Frame, workArea: Frame): Frame {
  return {
    x: clampOffsetInside(frame.x, frame.width, workArea.x, workArea.width),
    y: clampOffsetInside(frame.y, frame.height, workArea.y, workArea.height),
    width: frame.width,
    height: frame.height,
  };
}

function framesMatch(first: Frame, second: Frame, tolerance: number) {
  return (
    Math.abs(first.x - second.x) <= tolerance &&
    Math.abs(first.y - second.y) <= tolerance &&
    Math.abs(first.width - second.width) <= tolerance &&
    Math.abs(first.height - second.height) <= tolerance
  );
}

function isFrameInside(frame: Frame, workArea: Frame) {
  const right = workArea.x + workArea.width + EXACT_TOLERANCE_POINTS;
  const bottom = workArea.y + workArea.height + EXACT_TOLERANCE_POINTS;
  return (
    frame.x >= workArea.x - EXACT_TOLERANCE_POINTS &&
    frame.y >= workArea.y - EXACT_TOLERANCE_POINTS &&
    frame.x + frame.width <= right &&
    frame.y + frame.height <= bottom
  );
}

function classifyFinalOutcome(
  observed: Frame,
  requested: Frame,
  original: Frame,
): WriteOutcome {
  if (framesMatch(observed, requested, EXACT_TOLERANCE_POINTS)) {
    return 'exact';
  }
  if (framesMatch(observed, original, EXACT_TOLERANCE_POINTS)) {
    return 'unapplied';
  }
  return 'partial';
}

function successResult(
  outcome: WriteOutcome,
  changed: boolean,
  frame: Frame,
  reason?: string,
): WindowApplyResult {
  if (outcome === 'unapplied') {
    return failure('unapplied');
  }
  return {
    ok: true,
    outcome,
    changed,
    frame,
    ...(reason === undefined ? {} : { reason }),
  };
}

function helperFailure(envelope: HelperEnvelope<unknown>): FailedResult {
  const reason = HELPER_FAILURE_REASONS[envelope.code] ?? 'unapplied';
  return failure(reason, envelope.message ?? FAILURE_MESSAGES[reason]);
}

function unwrapEnvelope<Data>(envelope: HelperEnvelope<Data>): Settled<Data> {
  if (envelope.ok && envelope.data !== undefined) {
    return { ok: true, value: envelope.data };
  }
  return { ok: false, failure: helperFailure(envelope) };
}

function settleWrite(
  envelope: HelperEnvelope<SetFrameData>,
): Settled<SetFrameData> {
  const settled = unwrapEnvelope(envelope);
  if (!settled.ok || settled.value.outcome !== 'unapplied') {
    return settled;
  }
  return {
    ok: false,
    failure: failure(
      'unapplied',
      settled.value.reason ?? FAILURE_MESSAGES.unapplied,
    ),
  };
}

function correctionReason(
  correction: HelperEnvelope<SetFrameData>,
  written: SetFrameData,
) {
  return (
    correction.data?.reason ??
    (correction.ok ? written.reason : CORRECTION_FAILED_REASON)
  );
}

function describeFocusedWindowRejection(
  focused: WindowInfo,
  target: WindowFocusTarget,
): FailedResult | null {
  if (focused.bundleId !== target.bundleId) {
    return failure('target-lost', TARGET_WINDOW_CHANGED);
  }
  if (focused.isFullScreen) {
    return failure('full-screen');
  }
  if (focused.isMinimized) {
    return failure('minimized');
  }
  return null;
}

function requestedFrameFor(
  engine: Engine,
  spec: WindowLayoutSpec,
  writable: WritableWindow,
  gap: number,
): Frame {
  const { focused, workArea } = writable;
  const computed = engine.computeTargetFrame({
    spec,
    workArea,
    currentFrame: focused.frame,
    gap,
  });
  if (focused.isResizable) {
    return computed;
  }
  return clampFrameInside(
    {
      x: computed.x,
      y: computed.y,
      width: focused.frame.width,
      height: focused.frame.height,
    },
    workArea,
  );
}

async function resolveWritableWindow(
  dependencies: WindowManagementServiceDependencies,
  target: WindowFocusTarget,
): Promise<Settled<WritableWindow>> {
  const { manager, engine } = dependencies;
  if ((await manager.permissionState()) !== 'allowed') {
    return { ok: false, failure: failure('denied') };
  }
  const focused = unwrapEnvelope(await manager.focusedWindow(target.pid));
  if (focused.ok === false) {
    return focused;
  }
  const rejection = describeFocusedWindowRejection(focused.value, target);
  if (rejection) {
    return { ok: false, failure: rejection };
  }
  const displays = unwrapEnvelope(await manager.displays());
  if (displays.ok === false) {
    return displays;
  }
  if (displays.value.length === 0) {
    return { ok: false, failure: failure('unsupported', NO_DISPLAY_MESSAGE) };
  }
  const display = engine.pickDisplayForFrame(
    displays.value,
    focused.value.frame,
  );
  return {
    ok: true,
    value: { focused: focused.value, workArea: display.workArea },
  };
}

async function writeWithCorrection(
  manager: MacosWindowManager,
  pid: number,
  writable: WritableWindow,
  requested: Frame,
): Promise<WindowApplyResult> {
  const { focused, workArea } = writable;
  const first = settleWrite(
    await manager.setFrame({
      pid,
      windowKey: focused.windowKey,
      frame: requested,
    }),
  );
  if (first.ok === false) {
    return first.failure;
  }
  const written = first.value;
  if (written.outcome !== 'partial' || isFrameInside(written.frame, workArea)) {
    return successResult(
      written.outcome,
      written.changed,
      written.frame,
      written.reason,
    );
  }
  const correction = await manager.setFrame({
    pid,
    windowKey: focused.windowKey,
    frame: clampFrameInside(
      {
        ...requested,
        width: written.frame.width,
        height: written.frame.height,
      },
      workArea,
    ),
    positionOnly: true,
  });
  const observed = correction.data?.frame ?? written.frame;
  return successResult(
    classifyFinalOutcome(observed, requested, focused.frame),
    !framesMatch(observed, focused.frame, EXACT_TOLERANCE_POINTS),
    observed,
    correctionReason(correction, written),
  );
}

async function isFrontmostPid(
  ports: WindowInvocationPorts,
  pid: number,
): Promise<boolean> {
  return (await ports.frontmostTarget())?.pid === pid;
}

async function restorePaletteFocus(
  ports: WindowInvocationPorts,
  target: WindowFocusTarget,
): Promise<FailedResult | null> {
  const restored = await ports.hidePaletteAndRestoreFocus(target);
  if (restored && (await isFrontmostPid(ports, target.pid))) {
    return null;
  }
  return failure('focus-not-restored', PALETTE_FOCUS_NOT_RESTORED);
}

async function applyToTarget(
  dependencies: WindowManagementServiceDependencies,
  request: ApplyRequest,
): Promise<WindowApplyResult> {
  const { manager, ports, engine } = dependencies;
  const { spec, options, source, target } = request;
  if (source === 'palette') {
    const focusFailure = await restorePaletteFocus(ports, target);
    if (focusFailure) {
      return focusFailure;
    }
  }
  const writable = await resolveWritableWindow(dependencies, target);
  if (writable.ok === false) {
    return writable.failure;
  }
  const requested = requestedFrameFor(
    engine,
    spec,
    writable.value,
    options.gap,
  );
  if (source === 'shortcut' && !(await isFrontmostPid(ports, target.pid))) {
    return failure('focus-not-restored', SHORTCUT_FOCUS_CHANGED);
  }
  return writeWithCorrection(manager, target.pid, writable.value, requested);
}

export function createWindowManagementService(
  dependencies: WindowManagementServiceDependencies,
): WindowManagementService {
  const queues = new Map<number, Promise<void>>();

  function apply(
    spec: WindowLayoutSpec,
    options: WindowManagementOptions,
    invocation: WindowInvocation,
  ): Promise<WindowApplyResult> {
    const invalidMessage = describeInvalidRequest(spec, options);
    if (invalidMessage) {
      return Promise.resolve(failure('invalid-spec', invalidMessage));
    }
    if (!dependencies.supported()) {
      return Promise.resolve(failure('unsupported'));
    }
    const { target, source } = invocation;
    if (!target) {
      return Promise.resolve(failure('no-target', noTargetMessage(source)));
    }
    if (target.pid === dependencies.selfPid) {
      return Promise.resolve(failure('no-target', SELF_TARGET_MESSAGE));
    }
    return enqueueForPid(queues, target.pid, () =>
      applyToTarget(dependencies, { spec, options, source, target }),
    );
  }

  return {
    available: () => dependencies.supported(),
    apply,
  };
}
