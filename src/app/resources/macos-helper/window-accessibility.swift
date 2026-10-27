import AppKit
import ApplicationServices
import Foundation

typealias AccessibilityWindowIDFunction = @convention(c) (
  AXUIElement,
  UnsafeMutablePointer<CGWindowID>
) -> AXError

struct AccessibleWindow {
  let element: AXUIElement
  let pid: pid_t
  let id: CGWindowID

  var key: String {
    "\(pid):\(id)"
  }
}

let windowMessagingTimeoutSeconds: Float = 1
let rejectedWindowSubroles: Set<String> = [
  "AXSheet",
  "AXSystemDialog",
  "AXSystemFloatingWindow",
]
let axEnhancedUserInterfaceAttribute = "AXEnhancedUserInterface" as CFString
let axFullScreenAttribute = "AXFullScreen" as CFString
let accessibilityWindowIDFunction = loadAccessibilityWindowIDFunction()

func loadAccessibilityWindowIDFunction() -> AccessibilityWindowIDFunction? {
  guard
    let handle = dlopen(nil, RTLD_NOW),
    let symbol = dlsym(handle, "_AXUIElementGetWindow")
  else {
    return nil
  }
  return unsafeBitCast(symbol, to: AccessibilityWindowIDFunction.self)
}

func windowIDResolutionIsSupported() -> Bool {
  accessibilityWindowIDFunction != nil
}

func accessibilityApplication(pid: pid_t) -> AXUIElement {
  let application = AXUIElementCreateApplication(pid)
  _ = AXUIElementSetMessagingTimeout(application, windowMessagingTimeoutSeconds)
  return application
}

func accessibleWindow(_ element: AXUIElement, pid: pid_t) -> AccessibleWindow? {
  guard let resolveWindowID = accessibilityWindowIDFunction else { return nil }
  var identifier: CGWindowID = 0
  guard resolveWindowID(element, &identifier) == .success, identifier != 0 else {
    return nil
  }
  _ = AXUIElementSetMessagingTimeout(element, windowMessagingTimeoutSeconds)
  return AccessibleWindow(element: element, pid: pid, id: identifier)
}

func resolveAccessibleWindow(pid: pid_t, key: String) -> AccessibleWindow? {
  for element in accessibilityWindows(accessibilityApplication(pid: pid)) {
    guard let window = accessibleWindow(element, pid: pid), window.key == key else {
      continue
    }
    return window
  }
  return nil
}

func accessibilityWindows(_ application: AXUIElement) -> [AXUIElement] {
  (copyAttribute(application, kAXWindowsAttribute as CFString) as? [AXUIElement]) ?? []
}

func focusedOrMainWindowElement(_ application: AXUIElement) -> AXUIElement? {
  copyElementAttribute(application, kAXFocusedWindowAttribute as CFString)
    ?? copyElementAttribute(application, kAXMainWindowAttribute as CFString)
}

func windowRejectionReason(_ window: AccessibleWindow) -> String? {
  if
    let subrole = stringAttribute(window.element, kAXSubroleAttribute as CFString),
    rejectedWindowSubroles.contains(subrole)
  {
    return "Sheets and system dialogs are not managed"
  }
  let title = stringAttribute(window.element, kAXTitleAttribute as CFString) ?? ""
  if isPictureInPictureTitle(title) {
    return "Picture-in-Picture windows are not managed"
  }
  if let layer = windowLayer(window.id), layer != normalWindowLayer() {
    return "Only normal application windows are managed"
  }
  return nil
}

func normalWindowLayer() -> Int {
  Int(CGWindowLevelForKey(.normalWindow))
}

func isPictureInPictureTitle(_ title: String) -> Bool {
  let normalized = title.lowercased()
  return normalized.contains("picture in picture")
    || normalized.contains("picture-in-picture")
}

func windowLayer(_ identifier: CGWindowID) -> Int? {
  guard
    let windows = CGWindowListCopyWindowInfo(.optionIncludingWindow, identifier)
      as? [[String: Any]],
    let window = windows.first
  else {
    return nil
  }
  return window[kCGWindowLayer as String] as? Int
}

func bundleIdentifier(pid: pid_t) -> String {
  NSRunningApplication(processIdentifier: pid)?.bundleIdentifier ?? ""
}

func windowTitle(_ element: AXUIElement) -> String {
  stringAttribute(element, kAXTitleAttribute as CFString) ?? ""
}

func windowFrame(_ element: AXUIElement) -> CGRect? {
  guard
    let origin = pointAttribute(element, kAXPositionAttribute as CFString),
    let size = sizeAttribute(element, kAXSizeAttribute as CFString)
  else {
    return nil
  }
  return CGRect(origin: origin, size: size)
}

func setWindowOrigin(_ element: AXUIElement, _ origin: CGPoint) -> Bool {
  var mutableOrigin = origin
  guard let value = AXValueCreate(.cgPoint, &mutableOrigin) else { return false }
  return AXUIElementSetAttributeValue(element, kAXPositionAttribute as CFString, value)
    == .success
}

func setWindowSize(_ element: AXUIElement, _ size: CGSize) -> Bool {
  var mutableSize = size
  guard let value = AXValueCreate(.cgSize, &mutableSize) else { return false }
  return AXUIElementSetAttributeValue(element, kAXSizeAttribute as CFString, value)
    == .success
}

func setBooleanAttribute(_ element: AXUIElement, _ name: CFString, _ value: Bool) -> Bool {
  AXUIElementSetAttributeValue(element, name, value ? kCFBooleanTrue : kCFBooleanFalse)
    == .success
}

func isAttributeSettable(_ element: AXUIElement, _ name: CFString) -> Bool {
  var settable = DarwinBoolean(false)
  return AXUIElementIsAttributeSettable(element, name, &settable) == .success
    && settable.boolValue
}

func booleanAttribute(_ element: AXUIElement, _ name: CFString) -> Bool? {
  (copyAttribute(element, name) as? NSNumber)?.boolValue
}

func stringAttribute(_ element: AXUIElement, _ name: CFString) -> String? {
  copyAttribute(element, name) as? String
}

func pointAttribute(_ element: AXUIElement, _ name: CFString) -> CGPoint? {
  guard let value = axValueAttribute(element, name) else { return nil }
  var point = CGPoint.zero
  return AXValueGetValue(value, .cgPoint, &point) ? point : nil
}

func sizeAttribute(_ element: AXUIElement, _ name: CFString) -> CGSize? {
  guard let value = axValueAttribute(element, name) else { return nil }
  var size = CGSize.zero
  return AXValueGetValue(value, .cgSize, &size) ? size : nil
}

func axValueAttribute(_ element: AXUIElement, _ name: CFString) -> AXValue? {
  guard
    let value = copyAttribute(element, name),
    CFGetTypeID(value) == AXValueGetTypeID()
  else {
    return nil
  }
  return (value as! AXValue)
}

func copyElementAttribute(_ element: AXUIElement, _ name: CFString) -> AXUIElement? {
  guard
    let value = copyAttribute(element, name),
    CFGetTypeID(value) == AXUIElementGetTypeID()
  else {
    return nil
  }
  return (value as! AXUIElement)
}

func copyAttribute(_ element: AXUIElement, _ name: CFString) -> AnyObject? {
  var value: AnyObject?
  guard AXUIElementCopyAttributeValue(element, name, &value) == .success else {
    return nil
  }
  return value
}
