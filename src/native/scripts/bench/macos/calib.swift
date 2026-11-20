// Observer calibration: a harness-owned window flips its colour on every key press; the same key injection and
// frame capture used for the app trials measure key -> visible change. This is the floor of the observer pipeline.
import AppKit
import Foundation

final class FlipView: NSView {
    var on = false
    var onFlip: (() -> Void)?
    override var acceptsFirstResponder: Bool { true }
    override func keyDown(with event: NSEvent) { on.toggle(); needsDisplay = true; onFlip?() }
    override func draw(_ dirtyRect: NSRect) {
        (on ? NSColor.white : NSColor.black).setFill()
        dirtyRect.fill()
    }
}

func runCalibration(trials: Int, out: String) {
    let app = NSApplication.shared
    app.setActivationPolicy(.regular)
    let rect = NSRect(x: 200, y: 200, width: 400, height: 300)
    let win = NSWindow(contentRect: rect, styleMask: [.titled], backing: .buffered, defer: false)
    let view = FlipView(frame: rect)
    win.contentView = view
    win.makeKeyAndOrderFront(nil)
    win.makeFirstResponder(view)
    var flips = 0
    view.onFlip = { flips += 1 }
    app.activate(ignoringOtherApps: true)

    Thread.detachNewThread {
        let cap = Capture()
        let sem = DispatchSemaphore(value: 0)
        Task {
            // Screen coordinates for SCK are top-left based.
            let screenH = NSScreen.main!.frame.height
            let wf = DispatchQueue.main.sync { win.frame }
            let r = CGRect(x: wf.minX + 100, y: screenH - wf.maxY + 100, width: 200, height: 100)
            FileHandle.standardError.write("window=\(wf) capture=\(r)\n".data(using: .utf8)!)
            try! await cap.start(rect: r)
            sem.signal()
        }
        sem.wait()
        sleep(1)
        var rows = "trial,t_key,t_frame,latency_ms,status\n"
        for i in 0..<trials {
            let before = cap.current()
            cap.begin()
            let tKey = sendCombo("a")
            var found: Double?
            let deadline = Date().addingTimeInterval(2)
            while found == nil && Date() < deadline {
                for f in cap.take() {
                    if f.t < tKey - 0.001 { continue }
                    if let b = before, bandDiff(f, b, y0: 0, y1: f.height) > 100 { found = f.t; break }
                }
                usleep(2000)
            }
            cap.end()
            if let f = found {
                rows += "\(i),\(tKey),\(f),\(String(format: "%.2f", (f - tKey) * 1000)),ok\n"
            } else {
                rows += "\(i),\(tKey),,,timeout\n"
            }
            usleep(useconds_t(150_000 + Int.random(in: 0..<50_000)))
        }
        FileHandle.standardError.write("frames=\(cap.frameCount) flips=\(flips) active=\(NSApp.isActive) key=\(win.isKeyWindow)\n".data(using: .utf8)!)
        try! rows.write(toFile: out, atomically: true, encoding: .utf8)
        let s2 = DispatchSemaphore(value: 0)
        Task { await cap.stop(); s2.signal() }
        s2.wait()
        exit(0)
    }
    app.run()
}
