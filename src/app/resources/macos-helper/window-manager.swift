import AppKit
import ApplicationServices
import Foundation

enum WindowHelperCode: String {
  case ok
  case denied
  case noWindow = "no-window"
  case targetLost = "target-lost"
  case unsupported
  case unapplied
  case error

  var exitCode: Int32 {
    switch self {
    case .ok: return 0
    case .denied: return 2
    case .noWindow, .unapplied: return 3
    case .targetLost: return 4
    case .unsupported: return 5
    case .error: return 1
    }
  }
}

struct WindowResponse {
  let code: WindowHelperCode
  var data: Any? = nil
  var message: String? = nil
}

let windowUsage = "Usage: macos-selected-text window <permissions|displays|focused <pid>|set-frame <pid> <windowKey> <x> <y> [<width> <height>] [--position-only]|set-fullscreen <pid> <windowKey> <true|false>>"

let accessDeniedResponse = WindowResponse(
  code: .denied,
  message: "Nevermind does not have Accessibility access"
)
let unsupportedResponse = WindowResponse(
  code: .unsupported,
  message: "Window identifiers are unavailable on this macOS version"
)
let usageResponse = WindowResponse(code: .error, message: windowUsage)
let targetLostResponse = WindowResponse(
  code: .targetLost,
  message: "Window is no longer available"
)

func runWindowCommand(_ arguments: [String]) -> Never {
  guard AXIsProcessTrusted() else {
    emitWindowResponse(accessDeniedResponse)
  }
  emitWindowResponse(windowCommandResponse(arguments))
}

func windowCommandResponse(_ arguments: [String]) -> WindowResponse {
  switch arguments.first ?? "" {
  case "permissions": return WindowResponse(code: .ok)
  case "displays": return displaysResponse()
  case "focused": return focusedWindowResponse(arguments)
  case "set-frame": return setFrameResponse(arguments)
  case "set-fullscreen": return setFullScreenResponse(arguments)
  default: return usageResponse
  }
}

func emitWindowResponse(_ response: WindowResponse) -> Never {
  var envelope: [String: Any] = [
    "ok": response.code == .ok,
    "code": response.code.rawValue,
  ]
  if let data = response.data { envelope["data"] = data }
  if let message = response.message { envelope["message"] = message }
  let json = (try? JSONSerialization.data(withJSONObject: envelope))
    ?? Data(#"{"ok":false,"code":"error"}"#.utf8)
  FileHandle.standardOutput.write(json)
  FileHandle.standardOutput.write(Data("\n".utf8))
  exit(response.code.exitCode)
}

func rectDictionary(_ rect: CGRect) -> [String: Any] {
  [
    "x": Double(rect.origin.x),
    "y": Double(rect.origin.y),
    "width": Double(rect.size.width),
    "height": Double(rect.size.height),
  ]
}

func displayDictionary(_ display: WindowDisplay) -> [String: Any] {
  [
    "id": display.id,
    "isPrimary": display.isPrimary,
    "frame": rectDictionary(display.frame),
    "workArea": rectDictionary(display.workArea),
  ]
}

func frameWriteDictionary(_ result: FrameWriteResult) -> [String: Any] {
  var data: [String: Any] = [
    "outcome": result.outcome.rawValue,
    "changed": result.changed,
    "frame": rectDictionary(result.frame),
  ]
  if let reason = result.reason { data["reason"] = reason }
  return data
}

func frameWriteResponse(_ result: FrameWriteResult) -> WindowResponse {
  let data = frameWriteDictionary(result)
  guard result.outcome != .unapplied else {
    return WindowResponse(code: .unapplied, data: data, message: result.reason)
  }
  return WindowResponse(code: .ok, data: data)
}

func noWindowResponse(_ message: String) -> WindowResponse {
  WindowResponse(code: .noWindow, message: message)
}
