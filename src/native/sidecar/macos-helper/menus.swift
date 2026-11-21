import AppKit
import ApplicationServices
import Foundation

private struct MenuIdentity: Codable, Equatable {
  let pid: Int32
  let launchTime: Double
  let bundleID: String?
}

private struct MenuStep: Codable {
  let index: Int
  let role: String
  let title: String
}

private struct MenuReference: Codable {
  let identity: MenuIdentity
  let steps: [MenuStep]
}

private struct MenuEntry: Encodable {
  let reference: String
  let title: String
  let path: [String]
  let shortcut: String
  let enabled: Bool
  let checked: Bool
}

private func encodeMenuToken<T: Encodable>(_ value: T) -> String {
  guard let data = try? JSONEncoder().encode(value) else { fail("Menu encoding failed") }
  return data.base64EncodedString()
}

private func menuIdentity(pid: pid_t) -> MenuIdentity? {
  guard pid > 0, let app = NSRunningApplication(processIdentifier: pid),
    !app.isTerminated, let launch = app.launchDate
  else { return nil }
  let time = launch.timeIntervalSince1970
  guard time.isFinite, time > 0 else { return nil }
  return MenuIdentity(pid: pid, launchTime: time, bundleID: app.bundleIdentifier)
}

private func writeMenuJSON<T: Encodable>(_ value: T) -> Never {
  guard let data = try? JSONEncoder().encode(value) else { fail("Menu encoding failed") }
  FileHandle.standardOutput.write(data)
  FileHandle.standardOutput.write(Data("\n".utf8))
  exit(0)
}

func runMenusCommand(_ arguments: [String]) -> Never {
  guard arguments.count >= 2, let pid = pid_t(arguments[1]), pid > 0 else {
    fail("Usage: menus source <pid> | menus snapshot <pid> <identity> | menus invoke <pid> <identity> <reference>")
  }
  switch arguments[0] {
  case "source":
    guard arguments.count == 2 else { fail("Invalid menu arguments") }
    guard let identity = menuIdentity(pid: pid) else { fail("Menu source unavailable") }
    writeMenuJSON(MenuSource(pid: pid, identity: encodeMenuToken(identity)))
  case "snapshot":
    guard arguments.count == 3 else { fail("Invalid menu arguments") }
    guard AXIsProcessTrusted() else { fail("Nevermind does not have Accessibility access", code: 2) }
    guard let data = Data(base64Encoded: arguments[2]),
      let expected = try? JSONDecoder().decode(MenuIdentity.self, from: data),
      expected.pid == pid, let current = menuIdentity(pid: pid), current == expected
    else { fail("Menu source unavailable") }
    let application = accessibilityApplication(pid: pid)
    guard let bar = copyElementAttribute(application, kAXMenuBarAttribute as CFString) else {
      fail("Menu bar unavailable")
    }
    var entries: [MenuEntry] = []
    var visited = 0
    collectMenuEntries(bar, identity: expected, steps: [], path: [], depth: 0,
      visited: &visited, entries: &entries)
    guard menuIdentity(pid: pid) == expected else { fail("Menu source unavailable") }
    writeMenuJSON(entries)
  case "invoke":
    guard arguments.count == 4 else { fail("Invalid menu arguments") }
    guard AXIsProcessTrusted() else { fail("Nevermind does not have Accessibility access", code: 2) }
    guard let expected: MenuIdentity = decodeMenuToken(arguments[2]),
      let reference: MenuReference = decodeMenuToken(arguments[3]),
      expected.pid == pid, reference.identity == expected, menuIdentity(pid: pid) == expected,
      !reference.steps.isEmpty, reference.steps.count <= 12
    else { fail("Menu source or reference unavailable") }
    invokeMenu(pid: pid, identity: expected, reference: reference)
  default:
    fail("Unknown menu operation")
  }
}

private func decodeMenuToken<T: Decodable>(_ token: String) -> T? {
  guard token.utf8.count <= 65536, let data = Data(base64Encoded: token) else { return nil }
  return try? JSONDecoder().decode(T.self, from: data)
}

private func menuHasPress(_ element: AXUIElement) -> Bool {
  var actions: CFArray?
  return AXUIElementCopyActionNames(element, &actions) == .success
    && ((actions as? [String])?.contains(kAXPressAction as String) ?? false)
}

private func invokeMenu(pid: pid_t, identity: MenuIdentity, reference: MenuReference) -> Never {
  let application = accessibilityApplication(pid: pid)
  guard var element = copyElementAttribute(application, kAXMenuBarAttribute as CFString) else {
    fail("Menu reference unavailable")
  }
  for step in reference.steps {
    guard step.index >= 0, step.index < 5000,
      let children = copyAttribute(element, kAXChildrenAttribute as CFString) as? [AXUIElement],
      step.index < children.count
    else { fail("Menu reference unavailable") }
    element = children[step.index]
    _ = AXUIElementSetMessagingTimeout(element, windowMessagingTimeoutSeconds)
    guard stringAttribute(element, kAXRoleAttribute as CFString) == step.role,
      (stringAttribute(element, kAXTitleAttribute as CFString) ?? "") == step.title
    else { fail("Menu reference unavailable") }
    if step.role == "AXMenuItem",
      booleanAttribute(element, kAXEnabledAttribute as CFString) != true {
      fail("Menu reference unavailable")
    }
  }
  guard let terminal = reference.steps.last, terminal.role == "AXMenuItem", !terminal.title.isEmpty,
    menuIdentity(pid: pid) == identity,
    stringAttribute(element, kAXRoleAttribute as CFString) == terminal.role,
    (stringAttribute(element, kAXTitleAttribute as CFString) ?? "") == terminal.title,
    booleanAttribute(element, kAXEnabledAttribute as CFString) == true,
    menuHasPress(element), !menuHasSubmenu(element),
    menuIdentity(pid: pid) == identity,
    AXUIElementPerformAction(element, kAXPressAction as CFString) == .success
  else { fail("Menu reference unavailable") }
  writeMenuJSON(MenuInvokeResult(ok: true))
}

private struct MenuInvokeResult: Encodable {
  let ok: Bool
}

private func menuHasSubmenu(_ element: AXUIElement) -> Bool {
  let children = (copyAttribute(element, kAXChildrenAttribute as CFString) as? [AXUIElement]) ?? []
  return children.contains { stringAttribute($0, kAXRoleAttribute as CFString) == "AXMenu" }
}

private struct MenuSource: Encodable {
  let pid: Int32
  let identity: String
}

private func menuShortcut(_ element: AXUIElement) -> String {
  guard let key = stringAttribute(element, "AXMenuItemCmdChar" as CFString), !key.isEmpty else {
    return ""
  }
  let modifiers = (copyAttribute(element, "AXMenuItemCmdModifiers" as CFString) as? NSNumber)?.intValue ?? 0
  var parts: [String] = []
  if modifiers & 8 == 0 { parts.append("Command") }
  if modifiers & 4 != 0 { parts.append("Control") }
  if modifiers & 2 != 0 { parts.append("Alt") }
  if modifiers & 1 != 0 { parts.append("Shift") }
  parts.append(key.uppercased())
  return parts.joined(separator: "+")
}

private func collectMenuEntries(
  _ parent: AXUIElement, identity: MenuIdentity, steps: [MenuStep], path: [String], depth: Int,
  visited: inout Int, entries: inout [MenuEntry]
) {
  guard depth < 12, visited < 5000, entries.count < 2000 else { return }
  let children = (copyAttribute(parent, kAXChildrenAttribute as CFString) as? [AXUIElement]) ?? []
  for (index, child) in children.enumerated() {
    guard visited < 5000, entries.count < 2000 else { return }
    visited += 1
    _ = AXUIElementSetMessagingTimeout(child, windowMessagingTimeoutSeconds)
    guard let role = stringAttribute(child, kAXRoleAttribute as CFString) else { continue }
    let title = stringAttribute(child, kAXTitleAttribute as CFString) ?? ""
    let nextSteps = steps + [MenuStep(index: index, role: role, title: title)]
    let nextPath = role == "AXMenuItem" && !title.isEmpty ? path + [title] : path
    if role == "AXMenuItem", !title.isEmpty, menuHasPress(child), !menuHasSubmenu(child) {
      let mark = stringAttribute(child, "AXMenuItemMarkChar" as CFString) ?? ""
      entries.append(MenuEntry(
        reference: encodeMenuToken(MenuReference(identity: identity, steps: nextSteps)),
        title: title, path: path, shortcut: menuShortcut(child),
        enabled: booleanAttribute(child, kAXEnabledAttribute as CFString) ?? false,
        checked: !mark.isEmpty
      ))
    }
    collectMenuEntries(child, identity: identity, steps: nextSteps, path: nextPath,
      depth: depth + 1, visited: &visited, entries: &entries)
  }
}
