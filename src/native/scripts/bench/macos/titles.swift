// Row titles of a palette list, read the way the Linux harness reads them: the first line of each row in the list's
// text column, below the input row. Subtitles (the next line of the same row), right-hand values and footer chips
// are left out, so later changes to those do not count as list changes under the stable-list rule.
import AppKit
import Foundation

func rowTitles(_ all: [TextLine], limit: Int = 9) -> [String] {
    let sorted = all.filter { $0.confidence >= 0.5 }.sorted { $0.box.minY < $1.box.minY }
    guard let input = sorted.first else { return [] }
    let below = sorted.filter { $0.box.minY > input.box.maxY + 4 }
    let columnKey = { (l: TextLine) in Int((l.box.minX / 4).rounded()) }
    let byColumn = Dictionary(grouping: below, by: columnKey)
    guard let column = byColumn.max(by: { ($0.value.count, -$0.key) < ($1.value.count, -$1.key) })?.key else { return [] }
    var titles: [String] = []
    var previous: TextLine?
    for line in below where abs(columnKey(line) - column) <= 1 {
        let isSubtitle = previous.map { line.box.minY - $0.box.minY < 1.6 * $0.box.height } ?? false
        if !isSubtitle { titles.append(line.text) }
        previous = line
    }
    return Array(titles.prefix(limit))
}

/// Offline check of rowTitles on saved endpoint PNGs (no screen, keys or clipboard involved).
func printTitles(pngPaths: [String]) {
    for path in pngPaths {
        guard let img = NSImage(contentsOfFile: path)?.cgImage(forProposedRect: nil, context: nil, hints: nil) else { print("\(path): unreadable"); continue }
        let bpr = img.width * 4
        guard let ctx = CGContext(data: nil, width: img.width, height: img.height, bitsPerComponent: 8, bytesPerRow: bpr, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue) else { continue }
        ctx.draw(img, in: CGRect(x: 0, y: 0, width: img.width, height: img.height))
        let frame = Frame(t: 0, width: img.width, height: img.height, bytesPerRow: bpr, data: Data(bytes: ctx.data!, count: bpr * img.height))
        print("\((path as NSString).lastPathComponent): " + rowTitles(ocr(frame)).joined(separator: " | "))
    }
}
