// Compositor control for capture gaps, used only in diagnostic trials after the measured ones: a 24x24 harness panel
// in the screen's top-left corner flips colour a few times after a key and is captured by its own stream. If its flips
// show promptly while the app's window has not changed yet, the compositor was presenting and the delay is the app's;
// if they show only when the app's change appears, the compositor or the capture stalled.
import AppKit

final class ProbeView: NSView {
    var on = false
    override func draw(_ dirtyRect: NSRect) { (on ? NSColor.white : NSColor.black).setFill(); dirtyRect.fill() }
}

final class NonActivatingPanel: NSPanel {
    override var canBecomeKey: Bool { false }
    override var canBecomeMain: Bool { false }
}

private var probePanel: NSPanel?
private var probeView: ProbeView?

/// Shows the static probe without activating the harness and returns its rectangle in top-left screen points.
func startProbeWindow() -> CGRect {
    let size: CGFloat = 24, top: CGFloat = 40
    DispatchQueue.main.sync {
        NSApplication.shared.setActivationPolicy(.accessory)
        let screen = NSScreen.main!.frame
        let panel = NonActivatingPanel(contentRect: NSRect(x: 0, y: screen.height - top - size, width: size, height: size),
                                       styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
        panel.level = .statusBar
        panel.ignoresMouseEvents = true
        panel.hasShadow = false
        panel.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle]
        let view = ProbeView(frame: NSRect(x: 0, y: 0, width: size, height: size))
        panel.contentView = view
        panel.orderFrontRegardless()
        probePanel = panel
        probeView = view
    }
    return CGRect(x: 0, y: top, width: size, height: size)
}

/// Host times at which the probe actually flipped (recorded on the main thread).
final class ProbeFlips {
    private let lock = NSLock()
    private var flipped: [Double] = []
    func add(_ t: Double) { lock.lock(); flipped.append(t); lock.unlock() }
    func times() -> [Double] { lock.lock(); defer { lock.unlock() }; return flipped }
}

/// Flips the probe colour a few times after dispatch; each flip is one small redraw, so the compositor stays nearly idle.
func scheduleProbeFlips(afterMs: [Double]) -> ProbeFlips {
    let flips = ProbeFlips()
    for ms in afterMs {
        DispatchQueue.main.asyncAfter(deadline: .now() + ms / 1000) {
            guard let view = probeView else { return }
            view.on.toggle()
            view.display()
            flips.add(hostNow())
        }
    }
    return flips
}

/// "offset:delay" per flip, in ms: when the probe flipped after dispatch, and how long until its stream showed a frame.
func probeFlipDelays(_ flips: [Double], probeFrames: [Double], t0: Double) -> String {
    flips.map { f in
        let shown = probeFrames.filter { $0 >= f - 0.002 }.min()
        return String(format: "%.0f:%@", (f - t0) * 1000, shown.map { String(format: "%.0f", ($0 - f) * 1000) } ?? "none")
    }.joined(separator: ",")
}

/// Largest interval between consecutive capture events (frames or idle samples) in [from, to].
func largestGap(_ events: [Double], from: Double, to: Double) -> (start: Double, length: Double) {
    let ts = ([from] + events.filter { $0 > from && $0 < to }.sorted() + [to])
    var best = (start: from, length: 0.0)
    for k in 1..<ts.count where ts[k] - ts[k - 1] > best.length { best = (ts[k - 1], ts[k] - ts[k - 1]) }
    return best
}
