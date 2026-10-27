import ApplicationServices
import Foundation

enum FrameWriteOutcome: String {
  case exact
  case partial
  case unapplied
}

struct FrameWriteResult {
  let outcome: FrameWriteOutcome
  let changed: Bool
  let frame: CGRect
  let reason: String?
}

struct FrameWriteRequest {
  let window: AccessibleWindow
  let initial: CGRect
  let target: CGRect
  let positionOnly: Bool
  let bundleId: String
}

enum FrameWriteStep {
  case position
  case size
}

struct FramePlacement {
  let sizeFirst: Bool
  let crossesDisplays: Bool
}

let frameMatchTolerance: CGFloat = 2
let frameSettleAttemptCount = 10
let frameSettleInterval: TimeInterval = 0.02
let crossDisplayReapplyDelay: TimeInterval = 0.025
let chromiumBrowserBundleIdentifiers: Set<String> = [
  "com.google.Chrome",
  "com.google.Chrome.beta",
  "com.google.Chrome.canary",
  "com.google.Chrome.dev",
  "com.microsoft.edgemac",
  "com.brave.Browser",
  "company.thebrowser.Browser",
  "company.thebrowser.dia",
]

func writeWindowFrame(_ request: FrameWriteRequest) -> FrameWriteResult {
  let application = accessibilityApplication(pid: request.window.pid)
  let enhancedUserInterfaceWasOn =
    booleanAttribute(application, axEnhancedUserInterfaceAttribute) == true
  if enhancedUserInterfaceWasOn {
    _ = setBooleanAttribute(application, axEnhancedUserInterfaceAttribute, false)
  }
  defer {
    if enhancedUserInterfaceWasOn && !isChromiumBrowser(request.bundleId) {
      _ = setBooleanAttribute(application, axEnhancedUserInterfaceAttribute, true)
    }
  }
  return writeFrameAndSettle(request)
}

func isChromiumBrowser(_ bundleId: String) -> Bool {
  chromiumBrowserBundleIdentifiers.contains(bundleId)
}

func writeFrameAndSettle(_ request: FrameWriteRequest) -> FrameWriteResult {
  let element = request.window.element
  let canMove = isAttributeSettable(element, kAXPositionAttribute as CFString)
  let canResize =
    !request.positionOnly && isAttributeSettable(element, kAXSizeAttribute as CFString)
  guard canMove || canResize else {
    return FrameWriteResult(
      outcome: .unapplied,
      changed: false,
      frame: request.initial,
      reason: "Window position and size cannot be set"
    )
  }

  let placement = framePlacement(
    from: request.initial,
    to: request.target,
    displays: currentWindowDisplays()
  )
  var writesAccepted = applyFrameSteps(request, placement, canMove, canResize)
  var observed = settleFrame(element, target: request.target, fallback: request.initial)
  if !framesMatch(observed, request.target) && placement.crossesDisplays {
    Thread.sleep(forTimeInterval: crossDisplayReapplyDelay)
    writesAccepted = applyFrameSteps(request, placement, canMove, canResize) && writesAccepted
    observed = settleFrame(element, target: request.target, fallback: observed)
  }
  return classifyFrameWrite(
    initial: request.initial,
    observed: observed,
    target: request.target,
    writesAccepted: writesAccepted
  )
}

func applyFrameSteps(
  _ request: FrameWriteRequest,
  _ placement: FramePlacement,
  _ canMove: Bool,
  _ canResize: Bool
) -> Bool {
  var accepted = true
  for step in frameWriteSteps(sizeFirst: placement.sizeFirst, positionOnly: request.positionOnly) {
    switch step {
    case .position where canMove:
      accepted = setWindowOrigin(request.window.element, request.target.origin) && accepted
    case .size where canResize:
      accepted = setWindowSize(request.window.element, request.target.size) && accepted
    default:
      break
    }
  }
  return accepted
}

func frameWriteSteps(sizeFirst: Bool, positionOnly: Bool) -> [FrameWriteStep] {
  if positionOnly { return [.position] }
  return sizeFirst ? [.size, .position, .size] : [.position, .size, .position]
}

func framePlacement(
  from current: CGRect,
  to target: CGRect,
  displays: [WindowDisplay]
) -> FramePlacement {
  let currentDisplay = displayWithLargestOverlap(current, in: displays)
  let targetDisplay = displayWithLargestOverlap(target, in: displays)
  let crossesDisplays = currentDisplay?.id != targetDisplay?.id
  guard crossesDisplays, let destination = targetDisplay else {
    return FramePlacement(sizeFirst: true, crossesDisplays: false)
  }
  let sizeFirstOverflow = overflowArea(
    CGRect(origin: current.origin, size: target.size),
    outside: destination.frame
  )
  let positionFirstOverflow = overflowArea(
    CGRect(origin: target.origin, size: current.size),
    outside: destination.frame
  )
  return FramePlacement(
    sizeFirst: sizeFirstOverflow <= positionFirstOverflow,
    crossesDisplays: true
  )
}

func settleFrame(_ element: AXUIElement, target: CGRect, fallback: CGRect) -> CGRect {
  var observed = fallback
  for _ in 0..<frameSettleAttemptCount {
    if let frame = windowFrame(element) {
      if framesMatch(frame, target) { return frame }
      observed = frame
    }
    Thread.sleep(forTimeInterval: frameSettleInterval)
  }
  return windowFrame(element) ?? observed
}

func classifyFrameWrite(
  initial: CGRect,
  observed: CGRect,
  target: CGRect,
  writesAccepted: Bool
) -> FrameWriteResult {
  let changed = !framesMatch(initial, observed)
  if framesMatch(observed, target) {
    return FrameWriteResult(outcome: .exact, changed: changed, frame: observed, reason: nil)
  }
  if changed {
    return FrameWriteResult(
      outcome: .partial,
      changed: true,
      frame: observed,
      reason: partialFrameReason(observed: observed, target: target, writesAccepted: writesAccepted)
    )
  }
  return FrameWriteResult(
    outcome: .unapplied,
    changed: false,
    frame: observed,
    reason: writesAccepted
      ? "Window ignored the requested frame"
      : "Accessibility rejected the frame write"
  )
}

func partialFrameReason(observed: CGRect, target: CGRect, writesAccepted: Bool) -> String {
  if !writesAccepted { return "Accessibility rejected part of the frame write" }
  if !sizesMatch(observed.size, target.size) { return "Window clamped the requested size" }
  return "Window position was only partly applied"
}

func framesMatch(_ first: CGRect, _ second: CGRect) -> Bool {
  componentsMatch(first.origin.x, second.origin.x)
    && componentsMatch(first.origin.y, second.origin.y)
    && sizesMatch(first.size, second.size)
}

func sizesMatch(_ first: CGSize, _ second: CGSize) -> Bool {
  componentsMatch(first.width, second.width) && componentsMatch(first.height, second.height)
}

func componentsMatch(_ first: CGFloat, _ second: CGFloat) -> Bool {
  abs(first - second) <= frameMatchTolerance
}
