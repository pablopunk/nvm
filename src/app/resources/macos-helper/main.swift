import AppKit
import ApplicationServices
import Foundation

if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "window" {
  runWindowCommand(Array(CommandLine.arguments.dropFirst(2)))
}
if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "microphone" {
  runMicrophoneCommand(Array(CommandLine.arguments.dropFirst(2)))
}

if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "ocr" {
  runOCRCommand(Array(CommandLine.arguments.dropFirst(2)))
}

if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "thumbnail" {
  runThumbnailCommand(Array(CommandLine.arguments.dropFirst(2)))
}

if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "icon" {
  runIconCommand(Array(CommandLine.arguments.dropFirst(2)))
}

if CommandLine.arguments.count >= 2 && CommandLine.arguments[1] == "menus" {
  runMenusCommand(Array(CommandLine.arguments.dropFirst(2)))
}

guard AXIsProcessTrusted() else {
  fail("Nevermind does not have Accessibility access", code: 2)
}
if CommandLine.arguments.count == 2 && CommandLine.arguments[1] == "permissions" {
  guard CGPreflightPostEventAccess() else {
    fail("Accessibility access for keyboard events is blocked", code: 2)
  }
  _ = attribute(AXUIElementCreateSystemWide(), kAXFocusedApplicationAttribute as CFString)
  exit(0)
}
guard
  CommandLine.arguments.count == 3,
  let pid = pid_t(CommandLine.arguments[2])
else {
  fail("Usage: macos-selected-text <read|copy|replace> <pid> | permissions")
}

switch CommandLine.arguments[1] {
case "read": readSelection(pid: pid)
case "copy": copySelection(pid: pid)
case "replace": replaceSelection(pid: pid)
default: fail("Unknown selected-text operation")
}
