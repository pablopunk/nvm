// Compositor control for capture gaps: a 24x24 harness panel in the screen's top-left corner flips colour at 120 Hz
// and is captured by its own stream. If the probe keeps presenting while the app's stream is silent, the gap is in
// the app's window; if the probe goes silent too, the compositor or the capture stalled.
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
private var probeTimer: Timer?

/// Shows the probe without activating the harness and returns its rectangle in top-left screen points.
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
        probeTimer = Timer.scheduledTimer(withTimeInterval: 1.0 / 120, repeats: true) { _ in view.on.toggle(); view.needsDisplay = true }
        probePanel = panel
    }
    return CGRect(x: 0, y: top, width: size, height: size)
}

/// Largest interval between consecutive capture events (frames or idle samples) in [from, to].
func largestGap(_ events: [Double], from: Double, to: Double) -> (start: Double, length: Double) {
    let ts = ([from] + events.filter { $0 > from && $0 < to }.sorted() + [to])
    var best = (start: from, length: 0.0)
    for k in 1..<ts.count where ts[k] - ts[k - 1] > best.length { best = (ts[k - 1], ts[k] - ts[k - 1]) }
    return best
}
