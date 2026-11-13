import AppKit
import Foundation
import QuickLookThumbnailing

let thumbnailTimeoutSeconds = 20

/// Writes the Quick Look thumbnail of a file as a PNG: `thumbnail <path> <edge> <output>`.
func runThumbnailCommand(_ arguments: [String]) -> Never {
  guard arguments.count == 3, let edge = Double(arguments[1]), edge > 0 else {
    fputs("usage: thumbnail <path> <edge> <output>\n", stderr)
    exit(2)
  }
  let request = QLThumbnailGenerator.Request(
    fileAt: URL(fileURLWithPath: arguments[0]),
    size: CGSize(width: edge, height: edge),
    scale: 1,
    representationTypes: .all
  )
  let finished = DispatchSemaphore(value: 0)
  var written = false
  QLThumbnailGenerator.shared.generateBestRepresentation(for: request) { representation, error in
    defer { finished.signal() }
    guard let image = representation?.cgImage else {
      fputs("no thumbnail: \(error?.localizedDescription ?? "unknown")\n", stderr)
      return
    }
    let bitmap = NSBitmapImageRep(cgImage: image)
    guard let data = bitmap.representation(using: .png, properties: [:]) else { return }
    written = (try? data.write(to: URL(fileURLWithPath: arguments[2]))) != nil
  }
  guard finished.wait(timeout: .now() + .seconds(thumbnailTimeoutSeconds)) == .success else {
    fputs("thumbnail timed out\n", stderr)
    exit(3)
  }
  exit(written ? 0 : 1)
}

/// Writes the system icon of a file as a PNG: `icon <path> <size> <output>`. A new process draws the icon look the system
/// has now, which a running app does not see until it restarts.
func runIconCommand(_ arguments: [String]) -> Never {
  guard arguments.count == 3, let size = Int(arguments[1]), size > 0 else {
    fputs("usage: icon <path> <size> <output>\n", stderr)
    exit(2)
  }
  guard let bitmap = NSBitmapImageRep(
    bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size, bitsPerSample: 8, samplesPerPixel: 4,
    hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0
  ) else { exit(1) }
  NSGraphicsContext.saveGraphicsState()
  NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
  NSWorkspace.shared.icon(forFile: arguments[0]).draw(in: NSRect(x: 0, y: 0, width: size, height: size))
  NSGraphicsContext.restoreGraphicsState()
  guard let data = bitmap.representation(using: .png, properties: [:]) else { exit(1) }
  exit((try? data.write(to: URL(fileURLWithPath: arguments[2]))) != nil ? 0 : 1)
}
