// Canary / reference discovery: launch once, drive each scenario's keys with generous waits, and dump OCR lines and a
// PNG after every step. Produces the per-version reference strings; no timings come from this mode.
import AppKit
import Foundation
import Vision

func runExplore(configPath: String) {
    let c = try! JSONDecoder().decode(SessionConfig.self, from: Data(contentsOf: URL(fileURLWithPath: configPath)))
    let clip = ClipboardSnapshot()
    clip.save()
    Thread.detachNewThread {
        let s = Session(c)
        for f in ["/session.log", "/trials.jsonl", "/app-stdout.log", "/app-stderr.log"] { FileManager.default.createFile(atPath: c.outDir + f, contents: nil) }
        s.log = FileHandle(forWritingAtPath: c.outDir + "/session.log")
        s.trialsOut = FileHandle(forWritingAtPath: c.outDir + "/trials.jsonl")
        let tSpawn = s.spawn()
        let sem = DispatchSemaphore(value: 0)
        Task { try! await s.cap.start(rect: nil, pid: s.pid); sem.signal() }
        sem.wait()
        s.sampler = MemorySampler(root: s.pid, path: c.outDir + "/memory.csv", intervalMs: 200)
        s.note("spawned pid=\(s.pid)")
        var step = 0
        func dump(_ name: String, wait: Double = 1.5) {
            usleep(useconds_t(wait * 1_000_000))
            if let why = unsafeToType(s.pid) { s.note("UNSAFE at \(name): \(why); stopping"); s.finish(); clip.restore(); exit(3) }
            let f = s.snapshotNow()
            let rect = treeWindows(s.pid).first
            var text = ""
            if let f, let r = rect {
                // Crop to the palette window for OCR legibility.
                let crop = cgImage(f)?.cropping(to: r)
                if let crop {
                    let rep = NSBitmapImageRep(cgImage: crop)
                    try? rep.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: c.outDir + String(format: "/explore-%02d-%@.png", step, name)))
                    let req = VNRecognizeTextRequestCompat.lines(crop)
                    text = req.joined(separator: "\n")
                }
            }
            s.note("STEP \(step) \(name) windows=\(treeWindows(s.pid)) clipboard=\(clipboardState().1 ?? "nil")\n\(text)")
            step += 1
        }
        // One press, then wait: a second press queued before the window shows would toggle it closed again.
        var opened = false
        var tPress = 0.0
        let deadline = hostNow() + 60
        while !opened && hostNow() < deadline {
            if hostNow() - tPress > 10 { tPress = sendCombo(c.hotkey) }
            usleep(200_000)
            if s.paletteVisible() { usleep(1_000_000); opened = s.paletteVisible() }
        }
        let sem2 = DispatchSemaphore(value: 0)
        Task { await s.cap.stop(); try? await s.cap.start(rect: nil, pid: s.pid); sem2.signal() }
        sem2.wait()
        s.note("capture mode=\(s.cap.mode)")
        s.note("opened=\(opened) after \(String(format: "%.0f", (hostNow() - tSpawn) * 1000)) ms")
        dump("open")
        sleep(10); dump("open-after-10s")
        // Post-query sequence for the shared "settings" query: is the row order stable from first paint onward?
        typeText(c.settingsQuery)
        dump("settings-0.15s", wait: 0.15); dump("settings-0.4s", wait: 0.1); dump("settings-1s", wait: 0.4)
        dump("settings-2s", wait: 0.8); dump("settings-5s", wait: 2.8)
        sendCombo(c.actionKey); dump("action-panel")
        sendCombo("escape"); dump("action-closed")
        sendCombo("down"); dump("row-down")
        sendCombo("up"); dump("row-up")
        sendCombo("return"); dump("settings-open")
        sendCombo("escape"); dump("settings-back")
        s.clearQuery(); dump("cleared")
        typeText("1234*5678"); dump("calc")
        sendCombo("return"); dump("calc-enter")
        s.hide(); dump("hidden")
        s.finish()
        clip.restore()
        exit(0)
    }
    NSApplication.shared.setActivationPolicy(.prohibited)
    NSApplication.shared.run()
}

enum VNRecognizeTextRequestCompat {
    static func lines(_ img: CGImage) -> [String] {
        let bpr = img.width * 4
        guard let ctx = CGContext(data: nil, width: img.width, height: img.height, bitsPerComponent: 8, bytesPerRow: bpr, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue) else { return [] }
        ctx.draw(img, in: CGRect(x: 0, y: 0, width: img.width, height: img.height))
        let data = Data(bytes: ctx.data!, count: bpr * img.height)
        return ocr(Frame(t: 0, width: img.width, height: img.height, bytesPerRow: bpr, data: data)).map { "\(Int($0.box.minY))\t\($0.text)" }
    }
}
