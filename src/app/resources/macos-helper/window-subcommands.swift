import AppKit
import ApplicationServices
import Foundation

let positionOnlyFlag = "--position-only"

struct SetFrameArguments {
  let pid: pid_t
  let windowKey: String
  let target: CGRect
  let positionOnly: Bool
}

func displaysResponse() -> WindowResponse {
  WindowResponse(code: .ok, data: currentWindowDisplays().map(displayDictionary))
}

func focusedWindowResponse(_ arguments: [String]) -> WindowResponse {
  guard arguments.count == 2, let pid = pid_t(arguments[1]) else { return usageResponse }
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  guard
    let element = focusedOrMainWindowElement(accessibilityApplication(pid: pid))
  else {
    return noWindowResponse("No focused window")
  }
  guard let window = accessibleWindow(element, pid: pid) else {
    return noWindowResponse("Window identifier is unavailable")
  }
  if let reason = windowRejectionReason(window) { return noWindowResponse(reason) }
  guard let frame = windowFrame(element) else {
    return noWindowResponse("Window frame is unavailable")
  }
  let displays = currentWindowDisplays()
  let display = displayWithLargestOverlap(frame, in: displays)
    ?? primaryWindowDisplay(in: displays)
  let data: [String: Any] = [
    "bundleId": bundleIdentifier(pid: pid),
    "windowKey": window.key,
    "title": windowTitle(element),
    "frame": rectDictionary(frame),
    "displayId": display?.id ?? "",
    "isFullScreen": booleanAttribute(element, axFullScreenAttribute) ?? false,
    "isMinimized": booleanAttribute(element, kAXMinimizedAttribute as CFString) ?? false,
    "isMovable": isAttributeSettable(element, kAXPositionAttribute as CFString),
    "isResizable": isAttributeSettable(element, kAXSizeAttribute as CFString),
  ]
  return WindowResponse(code: .ok, data: data)
}

func setFrameResponse(_ arguments: [String]) -> WindowResponse {
  guard let request = parseSetFrameArguments(arguments) else { return usageResponse }
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  guard
    let window = resolveAccessibleWindow(pid: request.pid, key: request.windowKey)
  else {
    return targetLostResponse
  }
  if let reason = windowRejectionReason(window) { return noWindowResponse(reason) }
  guard let initial = windowFrame(window.element) else { return targetLostResponse }
  let target = request.positionOnly
    ? CGRect(origin: request.target.origin, size: initial.size)
    : request.target
  return frameWriteResponse(
    writeWindowFrame(
      FrameWriteRequest(
        window: window,
        initial: initial,
        target: target,
        positionOnly: request.positionOnly,
        bundleId: bundleIdentifier(pid: request.pid)
      )
    )
  )
}

func setFullScreenResponse(_ arguments: [String]) -> WindowResponse {
  guard
    arguments.count == 4,
    let pid = pid_t(arguments[1]),
    let requested = parseBooleanArgument(arguments[3])
  else {
    return usageResponse
  }
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  guard
    let window = resolveAccessibleWindow(pid: pid, key: arguments[2])
  else {
    return targetLostResponse
  }
  if let reason = windowRejectionReason(window) { return noWindowResponse(reason) }
  guard let initial = windowFrame(window.element) else { return targetLostResponse }
  let initialFullScreen = booleanAttribute(window.element, axFullScreenAttribute) ?? false
  return frameWriteResponse(
    writeFullScreen(
      window.element,
      requested: requested,
      initial: initial,
      initialFullScreen: initialFullScreen
    )
  )
}

func parseSetFrameArguments(_ arguments: [String]) -> SetFrameArguments? {
  let positionOnly = arguments.contains(positionOnlyFlag)
  let values = arguments.filter { $0 != positionOnlyFlag }
  guard
    values.count >= 3,
    let pid = pid_t(values[1]),
    let numbers = finiteNumbers(values.dropFirst(3)),
    numbers.count == 4 || (positionOnly && numbers.count == 2)
  else {
    return nil
  }
  let hasSize = numbers.count == 4
  let target = CGRect(
    x: numbers[0],
    y: numbers[1],
    width: hasSize ? numbers[2] : 0,
    height: hasSize ? numbers[3] : 0
  )
  return SetFrameArguments(
    pid: pid,
    windowKey: values[2],
    target: target,
    positionOnly: positionOnly
  )
}

func finiteNumbers(_ values: ArraySlice<String>) -> [CGFloat]? {
  let numbers = values.compactMap { Double($0) }
  guard numbers.count == values.count, numbers.allSatisfy({ $0.isFinite }) else {
    return nil
  }
  return numbers.map { CGFloat($0) }
}

func parseBooleanArgument(_ value: String) -> Bool? {
  switch value {
  case "true": return true
  case "false": return false
  default: return nil
  }
}
