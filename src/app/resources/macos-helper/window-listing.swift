import AppKit
import ApplicationServices
import Foundation

func runningAppsResponse() -> WindowResponse {
  let applications = NSWorkspace.shared.runningApplications
    .filter(isListableApplication)
    .map(runningAppDictionary)
  return WindowResponse(code: .ok, data: applications)
}

func isListableApplication(_ application: NSRunningApplication) -> Bool {
  application.activationPolicy == .regular
    && !(application.bundleIdentifier ?? "").isEmpty
    && application.processIdentifier != nevermindProcessIdentifier()
}

func nevermindProcessIdentifier() -> pid_t {
  getppid()
}

func runningAppDictionary(_ application: NSRunningApplication) -> [String: Any] {
  [
    "pid": Int(application.processIdentifier),
    "bundleId": application.bundleIdentifier ?? "",
    "name": application.localizedName ?? "",
  ]
}

func windowListResponse(_ arguments: [String]) -> WindowResponse {
  guard arguments.count == 2, let pid = pid_t(arguments[1]) else { return usageResponse }
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  let displays = currentWindowDisplays()
  let windows = accessibilityWindows(accessibilityApplication(pid: pid)).compactMap {
    listedWindowDescriptor($0, pid: pid, displays: displays)
  }
  return WindowResponse(code: .ok, data: windows)
}

func listedWindowDescriptor(
  _ element: AXUIElement,
  pid: pid_t,
  displays: [WindowDisplay]
) -> [String: Any]? {
  guard
    let window = accessibleWindow(element, pid: pid),
    windowRejectionReason(window) == nil,
    let frame = windowFrame(element)
  else {
    return nil
  }
  return windowDescriptor(window, frame: frame, displays: displays)
}

func windowDescriptor(
  _ window: AccessibleWindow,
  frame: CGRect,
  displays: [WindowDisplay]
) -> [String: Any] {
  let element = window.element
  let display = displayWithLargestOverlap(frame, in: displays)
    ?? primaryWindowDisplay(in: displays)
  return [
    "bundleId": bundleIdentifier(pid: window.pid),
    "windowKey": window.key,
    "title": windowTitle(element),
    "frame": rectDictionary(frame),
    "displayId": display?.id ?? "",
    "isFullScreen": booleanAttribute(element, axFullScreenAttribute) ?? false,
    "isMinimized": booleanAttribute(element, kAXMinimizedAttribute as CFString) ?? false,
    "isMovable": isAttributeSettable(element, kAXPositionAttribute as CFString),
    "isResizable": isAttributeSettable(element, kAXSizeAttribute as CFString),
  ]
}
