import AppKit
import Foundation

struct WindowDisplay {
  let id: String
  let isPrimary: Bool
  let frame: CGRect
  let workArea: CGRect
}

func currentWindowDisplays() -> [WindowDisplay] {
  guard let primary = NSScreen.screens.first else { return [] }
  var displays: [WindowDisplay] = []
  for screen in NSScreen.screens {
    guard let id = displayIdentifier(screen) else { continue }
    displays.append(
      WindowDisplay(
        id: id,
        isPrimary: screen === primary,
        frame: accessibilityRect(screen.frame, primaryFrame: primary.frame),
        workArea: accessibilityRect(screen.visibleFrame, primaryFrame: primary.frame)
      )
    )
  }
  return displays
}

func displayIdentifier(_ screen: NSScreen) -> String? {
  let number = screen.deviceDescription[NSDeviceDescriptionKey("NSScreenNumber")]
  return (number as? NSNumber)?.stringValue
}

func accessibilityRect(_ rect: CGRect, primaryFrame: CGRect) -> CGRect {
  CGRect(
    x: rect.origin.x,
    y: primaryFrame.maxY - (rect.origin.y + rect.height),
    width: rect.width,
    height: rect.height
  )
}

func displayWithLargestOverlap(_ frame: CGRect, in displays: [WindowDisplay]) -> WindowDisplay? {
  var best: WindowDisplay?
  var bestArea: CGFloat = 0
  for display in displays {
    let area = overlapArea(frame, display.frame)
    if area > bestArea {
      best = display
      bestArea = area
    }
  }
  return best
}

func primaryWindowDisplay(in displays: [WindowDisplay]) -> WindowDisplay? {
  displays.first { $0.isPrimary } ?? displays.first
}

func overlapArea(_ first: CGRect, _ second: CGRect) -> CGFloat {
  let intersection = first.intersection(second)
  return intersection.isNull ? 0 : intersection.width * intersection.height
}

func overflowArea(_ frame: CGRect, outside display: CGRect) -> CGFloat {
  frame.width * frame.height - overlapArea(frame, display)
}
