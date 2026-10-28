import AppKit
import ApplicationServices
import Foundation

let fullScreenSettleAttemptCount = 50
let fullScreenSettleInterval: TimeInterval = 0.02

func writeFullScreen(
  _ element: AXUIElement,
  requested: Bool,
  initial: CGRect,
  initialFullScreen: Bool
) -> FrameWriteResult {
  guard isAttributeSettable(element, axFullScreenAttribute) else {
    return FrameWriteResult(
      outcome: .unapplied,
      changed: false,
      frame: initial,
      reason: "Full screen cannot be changed for this window"
    )
  }
  let writeAccepted = setBooleanAttribute(element, axFullScreenAttribute, requested)
  let observed = settleFullScreen(element, requested: requested, fallback: initialFullScreen)
  let frame = windowFrame(element) ?? initial
  let changed = observed != initialFullScreen
  if observed == requested {
    return FrameWriteResult(outcome: .exact, changed: changed, frame: frame, reason: nil)
  }
  if changed {
    return FrameWriteResult(
      outcome: .partial,
      changed: true,
      frame: frame,
      reason: "Full screen transition did not finish"
    )
  }
  return FrameWriteResult(
    outcome: .unapplied,
    changed: false,
    frame: frame,
    reason: writeAccepted
      ? "Window did not change full screen state"
      : "Accessibility rejected the full screen change"
  )
}

func settleFullScreen(_ element: AXUIElement, requested: Bool, fallback: Bool) -> Bool {
  var observed = fallback
  for _ in 0..<fullScreenSettleAttemptCount {
    observed = booleanAttribute(element, axFullScreenAttribute) ?? observed
    if observed == requested { return observed }
    Thread.sleep(forTimeInterval: fullScreenSettleInterval)
  }
  return booleanAttribute(element, axFullScreenAttribute) ?? observed
}
