import AppKit
import ApplicationServices
import Foundation

func raiseWindowResponse(_ arguments: [String]) -> WindowResponse {
  guard arguments.count == 3, let pid = pid_t(arguments[1]) else { return usageResponse }
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  guard let window = resolveAccessibleWindow(pid: pid, key: arguments[2]) else {
    return targetLostResponse
  }
  if let reason = windowRejectionReason(window) { return noWindowResponse(reason) }
  restoreMinimizedWindow(window.element)
  raiseAccessibleWindow(window.element)
  activateApplication(pid: pid)
  return WindowResponse(code: .ok)
}

func restoreMinimizedWindow(_ element: AXUIElement) {
  guard booleanAttribute(element, kAXMinimizedAttribute as CFString) == true else { return }
  _ = setBooleanAttribute(element, kAXMinimizedAttribute as CFString, false)
}

func raiseAccessibleWindow(_ element: AXUIElement) {
  _ = AXUIElementPerformAction(element, kAXRaiseAction as CFString)
  _ = setBooleanAttribute(element, kAXMainAttribute as CFString, true)
  _ = setBooleanAttribute(element, kAXFocusedAttribute as CFString, true)
}

func activateApplication(pid: pid_t) {
  _ = NSRunningApplication(processIdentifier: pid)?.activate(options: [.activateIgnoringOtherApps])
}
