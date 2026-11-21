import AppKit
import ApplicationServices
import Foundation

let allWindowsMessagingTimeoutSeconds: Float = 0.3
let offScreenWindowZIndex = 100_000

func allWindowsResponse() -> WindowResponse {
  guard windowIDResolutionIsSupported() else { return unsupportedResponse }
  let displays = currentWindowDisplays()
  let zOrder = frontToBackWindowOrder()
  let windows = listableApplications().flatMap {
    listedApplicationWindows($0, displays: displays, zOrder: zOrder)
  }
  return WindowResponse(code: .ok, data: windows)
}

func frontToBackWindowOrder() -> [CGWindowID: Int] {
  let windows =
    (CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]])
    ?? []
  var order: [CGWindowID: Int] = [:]
  for (index, window) in windows.enumerated() {
    guard
      let number = window[kCGWindowNumber as String] as? Int,
      let identifier = CGWindowID(exactly: number)
    else {
      continue
    }
    order[identifier] = index
  }
  return order
}

func listedApplicationWindows(
  _ application: NSRunningApplication,
  displays: [WindowDisplay],
  zOrder: [CGWindowID: Int]
) -> [[String: Any]] {
  let pid = application.processIdentifier
  let appName = application.localizedName ?? ""
  let appElement = accessibilityApplication(
    pid: pid,
    timeoutSeconds: allWindowsMessagingTimeoutSeconds
  )
  return accessibilityWindows(appElement).compactMap {
    listedApplicationWindow(
      $0,
      pid: pid,
      appName: appName,
      displays: displays,
      zOrder: zOrder
    )
  }
}

func listedApplicationWindow(
  _ element: AXUIElement,
  pid: pid_t,
  appName: String,
  displays: [WindowDisplay],
  zOrder: [CGWindowID: Int]
) -> [String: Any]? {
  guard
    let window = accessibleWindow(
      element,
      pid: pid,
      timeoutSeconds: allWindowsMessagingTimeoutSeconds
    ),
    windowRejectionReason(window) == nil,
    let frame = windowFrame(element)
  else {
    return nil
  }
  var descriptor = windowDescriptor(window, frame: frame, displays: displays)
  descriptor["pid"] = Int(pid)
  descriptor["appName"] = appName
  descriptor["zIndex"] = zOrder[window.id] ?? offScreenWindowZIndex
  return descriptor
}
