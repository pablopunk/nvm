import AppKit
import ApplicationServices
import Foundation

private let noSelectionExitCode: Int32 = 3

private func fail(_ message: String, code: Int32 = 1) -> Never {
  FileHandle.standardError.write(Data("\(message)\n".utf8))
  exit(code)
}

private func attribute(_ element: AXUIElement, _ name: CFString) -> AnyObject? {
  var value: AnyObject?
  guard AXUIElementCopyAttributeValue(element, name, &value) == .success else {
    return nil
  }
  return value
}

private func parameterizedAttribute(
  _ element: AXUIElement,
  _ name: CFString,
  _ parameter: AnyObject
) -> AnyObject? {
  var value: AnyObject?
  guard
    AXUIElementCopyParameterizedAttributeValue(
      element,
      name,
      parameter,
      &value
    ) == .success
  else {
    return nil
  }
  return value
}

private func text(_ value: AnyObject?) -> String? {
  if let value = value as? String, !value.isEmpty { return value }
  if let value = value as? NSAttributedString, !value.string.isEmpty {
    return value.string
  }
  return nil
}

private func element(_ value: AnyObject?) -> AXUIElement? {
  guard let value, CFGetTypeID(value) == AXUIElementGetTypeID() else {
    return nil
  }
  return (value as! AXUIElement)
}

private func selectedRange(_ element: AXUIElement) -> (AnyObject, CFRange)? {
  guard
    let value = attribute(element, kAXSelectedTextRangeAttribute as CFString)
  else {
    return nil
  }
  var range = CFRange(location: 0, length: 0)
  guard
    CFGetTypeID(value) == AXValueGetTypeID(),
    AXValueGetValue(value as! AXValue, .cfRange, &range),
    range.length > 0
  else {
    return nil
  }
  return (value, range)
}

private func selectedTextFromRange(
  _ element: AXUIElement,
  _ rangeValue: AnyObject,
  _ range: CFRange
) -> String? {
  for name in [
    kAXStringForRangeParameterizedAttribute as CFString,
    kAXAttributedStringForRangeParameterizedAttribute as CFString,
  ] {
    if let result = text(parameterizedAttribute(element, name, rangeValue)) {
      return result
    }
  }

  guard let fullText = attribute(element, kAXValueAttribute as CFString) as? String
  else {
    return nil
  }
  let utf16 = fullText.utf16
  guard
    let start = utf16.index(
      utf16.startIndex,
      offsetBy: range.location,
      limitedBy: utf16.endIndex
    ),
    let end = utf16.index(
      start,
      offsetBy: range.length,
      limitedBy: utf16.endIndex
    )
  else {
    return nil
  }
  return String(utf16[start..<end])
}

private func selectedText(_ element: AXUIElement) -> String? {
  let role = attribute(element, kAXRoleAttribute as CFString) as? String
  let subrole = attribute(element, kAXSubroleAttribute as CFString) as? String
  if role == "AXSecureTextField" || subrole == (kAXSecureTextFieldSubrole as String) {
    return nil
  }

  if let result = text(attribute(element, kAXSelectedTextAttribute as CFString)) {
    return result
  }
  if let markerRange = attribute(element, "AXSelectedTextMarkerRange" as CFString) {
    for name in [
      "AXStringForTextMarkerRange" as CFString,
      "AXAttributedStringForTextMarkerRange" as CFString,
    ] {
      if let result = text(parameterizedAttribute(element, name, markerRange)) {
        return result
      }
    }
  }
  if let (rangeValue, range) = selectedRange(element) {
    return selectedTextFromRange(element, rangeValue, range)
  }
  return nil
}

private func selectedTextFromFocusedHierarchy(_ root: AXUIElement) -> String? {
  var current = root
  for _ in 0..<8 {
    if let result = selectedText(current) { return result }
    guard let focused = element(
      attribute(current, kAXFocusedUIElementAttribute as CFString)
    ) else {
      return nil
    }
    current = focused
  }
  return nil
}

private func readSelection(pid: pid_t) -> Never {
  let app = AXUIElementCreateApplication(pid)
  AXUIElementSetAttributeValue(
    app,
    "AXManualAccessibility" as CFString,
    kCFBooleanTrue
  )

  for attempt in 0..<2 {
    if let focused = element(
      attribute(app, kAXFocusedUIElementAttribute as CFString)
    ), let result = selectedTextFromFocusedHierarchy(focused) {
      FileHandle.standardOutput.write(Data(result.utf8))
      exit(0)
    }
    if attempt == 0 { Thread.sleep(forTimeInterval: 0.06) }
  }
  exit(noSelectionExitCode)
}

private func copySelection(pid: pid_t) -> Never {
  guard NSWorkspace.shared.frontmostApplication?.processIdentifier == pid else {
    fail("Selection target is not frontmost", code: 4)
  }
  guard let source = CGEventSource(stateID: .hidSystemState) else {
    fail("Could not create keyboard event source")
  }
  for keyDown in [true, false] {
    guard
      let event = CGEvent(
        keyboardEventSource: source,
        virtualKey: 8,
        keyDown: keyDown
      )
    else {
      fail("Could not create keyboard event")
    }
    event.flags = .maskCommand
    event.post(tap: .cghidEventTap)
  }
  exit(0)
}

guard AXIsProcessTrusted() else {
  fail("Nevermind does not have Accessibility access", code: 2)
}
guard
  CommandLine.arguments.count == 3,
  let pid = pid_t(CommandLine.arguments[2])
else {
  fail("Usage: macos-selected-text <read|copy> <pid>")
}

switch CommandLine.arguments[1] {
case "read": readSelection(pid: pid)
case "copy": copySelection(pid: pid)
default: fail("Unknown selected-text operation")
}
