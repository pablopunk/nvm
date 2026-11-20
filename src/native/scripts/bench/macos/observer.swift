// nvmbench observer: external key injection, ScreenCaptureKit frame capture, OCR/pixel predicates,
// clipboard polling and whole-process-tree memory sampling. All times use the host time clock (seconds).
import AppKit
import CoreMedia
import Darwin
import Foundation
import ScreenCaptureKit
import Vision

func hostNow() -> Double { CMTimeGetSeconds(CMClockGetTime(CMClockGetHostTimeClock())) }

// MARK: keys

let keyCodes: [String: CGKeyCode] = [
    "space": 49, "return": 36, "escape": 53, "down": 125, "up": 126, "delete": 51, "a": 0, "k": 40, "tab": 48,
]

func screenLocked() -> Bool {
    let d = CGSessionCopyCurrentDictionary() as? [String: Any] ?? [:]
    return (d["CGSSessionScreenIsLocked"] as? Bool ?? false) || CGDisplayIsAsleep(CGMainDisplayID()) != 0
}

func postKey(_ code: CGKeyCode, flags: CGEventFlags = [], unicode: String? = nil) {
    // Never type into the lock screen or a sleeping display.
    if screenLocked() { FileHandle.standardError.write("REFUSED: screen locked or display asleep; no key sent\n".data(using: .utf8)!); exit(4) }
    let src = CGEventSource(stateID: .hidSystemState)
    for down in [true, false] {
        guard let e = CGEvent(keyboardEventSource: src, virtualKey: code, keyDown: down) else { continue }
        e.flags = flags
        if let u = unicode {
            let chars = Array(u.utf16)
            e.keyboardSetUnicodeString(stringLength: chars.count, unicodeString: chars)
        }
        e.post(tap: .cghidEventTap)
    }
}

/// Posts a key combo like "alt+shift+space" or "cmd+k" and returns the host time just before posting.
@discardableResult
func sendCombo(_ combo: String) -> Double {
    var flags: CGEventFlags = []
    var key = ""
    for part in combo.lowercased().split(separator: "+") {
        switch part {
        case "cmd", "command": flags.insert(.maskCommand)
        case "alt", "option": flags.insert(.maskAlternate)
        case "shift": flags.insert(.maskShift)
        case "ctrl", "control": flags.insert(.maskControl)
        default: key = String(part)
        }
    }
    let t = hostNow()
    postKey(keyCodes[key] ?? 0, flags: flags)
    return t
}

/// Types text one character every `delayMs`, like xdotool --delay. Returns (first key time, last key time).
func typeSlowly(_ text: String, delayMs: Int) -> (Double, Double) {
    let t0 = hostNow()
    for (i, ch) in text.enumerated() {
        if i > 0 { usleep(useconds_t(delayMs * 1000)) }
        postKey(0, unicode: String(ch))
    }
    return (t0, hostNow())
}

/// Types text as unicode key events back to back. Returns the host time before the first event.
@discardableResult
func typeText(_ text: String) -> Double {
    let t = hostNow()
    for ch in text { postKey(0, unicode: String(ch)) }
    return t
}

// MARK: process tree and memory

struct ProcSample { let pid: Int32; let ppid: Int32; let startUs: UInt64; let ok: Bool; let name: String; let footprint: UInt64; let rss: UInt64; let cpuNs: UInt64 }

var timebase: mach_timebase_info_data_t = { var t = mach_timebase_info_data_t(); mach_timebase_info(&t); return t }()

func allProcs() -> [(pid: Int32, ppid: Int32)] {
    var mib = [CTL_KERN, KERN_PROC, KERN_PROC_ALL, 0]
    var size = 0
    sysctl(&mib, 4, nil, &size, nil, 0)
    let count = size / MemoryLayout<kinfo_proc>.stride + 16
    var procs = [kinfo_proc](repeating: kinfo_proc(), count: count)
    size = count * MemoryLayout<kinfo_proc>.stride
    sysctl(&mib, 4, &procs, &size, nil, 0)
    return procs.prefix(size / MemoryLayout<kinfo_proc>.stride).map { ($0.kp_proc.p_pid, $0.kp_eproc.e_ppid) }
}

func descendants(of root: Int32) -> [Int32: Int32] {
    let procs = allProcs()
    var tree: [Int32: Int32] = [root: 0]
    var changed = true
    while changed {
        changed = false
        for p in procs where tree[p.pid] == nil && tree[p.ppid] != nil {
            tree[p.pid] = p.ppid
            changed = true
        }
    }
    return tree
}

func procName(_ pid: Int32) -> String {
    var buf = [CChar](repeating: 0, count: 4 * Int(MAXPATHLEN))
    let n = proc_pidpath(pid, &buf, UInt32(buf.count))
    return n > 0 ? String(cString: buf) : "?"
}

func sampleTree(_ root: Int32) -> [ProcSample] {
    descendants(of: root).compactMap { pid, ppid in
        var info = rusage_info_v4()
        let ok = withUnsafeMutablePointer(to: &info) {
            $0.withMemoryRebound(to: rusage_info_t?.self, capacity: 1) { proc_pid_rusage(pid, RUSAGE_INFO_V4, $0) }
        }
        guard ok == 0 else { return ProcSample(pid: pid, ppid: ppid, startUs: 0, ok: false, name: procName(pid), footprint: 0, rss: 0, cpuNs: 0) }
        let cpu = (info.ri_user_time + info.ri_system_time) * UInt64(timebase.numer) / UInt64(timebase.denom)
        return ProcSample(pid: pid, ppid: ppid, startUs: info.ri_proc_start_abstime * UInt64(timebase.numer) / UInt64(timebase.denom) / 1000, ok: true, name: procName(pid), footprint: info.ri_phys_footprint, rss: info.ri_resident_size, cpuNs: cpu)
    }
}

/// Samples the root's whole tree every interval into a CSV until stopped. Phase labels come from `phase`.
final class MemorySampler {
    let root: Int32
    let out: FileHandle
    var phase = "startup"
    private var running = true
    private let thread: Thread
    private(set) var peakFootprint: UInt64 = 0
    private(set) var peakRSS: UInt64 = 0

    init(root: Int32, path: String, intervalMs: Int) {
        self.root = root
        FileManager.default.createFile(atPath: path, contents: "t_host,t_read_end,phase,phase_after,pid,ppid,start_us,name,phys_footprint_bytes,rss_bytes,cpu_ns,read_ok\n".data(using: .utf8))
        out = FileHandle(forWritingAtPath: path)!
        out.seekToEndOfFile()
        thread = Thread {}
        let t = Thread { [unowned self] in
            while self.running {
                let now = hostNow()
                let phaseBefore = self.phase
                let s = sampleTree(self.root)
                let phaseAfter = self.phase
                let readEnd = hostNow()
                var lines = ""
                var fp: UInt64 = 0, rss: UInt64 = 0
                for p in s {
                    fp += p.footprint; rss += p.rss
                    lines += String(format: "%.6f,%.6f", now, readEnd) + ",\(phaseBefore),\(phaseAfter),\(p.pid),\(p.ppid),\(p.startUs),\"\(p.name)\",\(p.footprint),\(p.rss),\(p.cpuNs),\(p.ok)\n"
                }
                self.peakFootprint = max(self.peakFootprint, fp)
                self.peakRSS = max(self.peakRSS, rss)
                self.out.write(lines.data(using: .utf8)!)
                usleep(useconds_t(intervalMs * 1000))
            }
        }
        t.start()
    }

    func stop() { running = false; usleep(200_000); try? out.close() }
}

// MARK: windows

func treeWindows(_ root: Int32) -> [CGRect] {
    let pids = Set(descendants(of: root).keys)
    guard let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else { return [] }
    return list.compactMap { w in
        guard let pid = w[kCGWindowOwnerPID as String] as? Int32, pids.contains(pid),
              let alpha = w[kCGWindowAlpha as String] as? Double, alpha > 0,
              let b = w[kCGWindowBounds as String] as? [String: Double] else { return nil }
        let r = CGRect(x: b["X"]!, y: b["Y"]!, width: b["Width"]!, height: b["Height"]!)
        return r.width >= 300 && r.height >= 40 ? r : nil
    }
}

// MARK: frames

struct Frame { let t: Double; let width: Int; let height: Int; let bytesPerRow: Int; let data: Data }

func cgImage(_ f: Frame) -> CGImage? {
    guard let provider = CGDataProvider(data: f.data as CFData) else { return nil }
    return CGImage(width: f.width, height: f.height, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: f.bytesPerRow,
                   space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue),
                   provider: provider, decode: nil, shouldInterpolate: false, intent: .defaultIntent)
}

struct TextLine { let text: String; let box: CGRect; var confidence: Float = 1 } // box in pixel coordinates, origin top-left

func ocr(_ f: Frame) -> [TextLine] {
    guard let img = cgImage(f) else { return [] }
    let req = VNRecognizeTextRequest()
    req.recognitionLevel = .accurate
    req.usesLanguageCorrection = false
    try? VNImageRequestHandler(cgImage: img).perform([req])
    return (req.results ?? []).compactMap { o in
        guard let cand = o.topCandidates(1).first else { return nil }
        let s = cand.string
        let b = o.boundingBox
        return TextLine(text: s, box: CGRect(x: b.minX * Double(f.width), y: (1 - b.maxY) * Double(f.height), width: b.width * Double(f.width), height: b.height * Double(f.height)), confidence: cand.confidence)
    }
}

/// Mean absolute per-channel difference between two frames over a band (rows y0..<y1), 0...255.
func bandDiff(_ a: Frame, _ b: Frame, y0: Int, y1: Int) -> Double {
    guard a.width == b.width, a.height == b.height else { return 255 }
    var sum = 0, n = 0
    a.data.withUnsafeBytes { pa in
        b.data.withUnsafeBytes { pb in
            let ua = pa.bindMemory(to: UInt8.self), ub = pb.bindMemory(to: UInt8.self)
            for y in stride(from: max(0, y0), to: min(a.height, y1), by: 2) {
                let row = y * a.bytesPerRow
                for x in stride(from: 0, to: a.width * 4, by: 8) {
                    sum += abs(Int(ua[row + x]) - Int(ub[row + x])); n += 1
                }
            }
        }
    }
    return n == 0 ? 0 : Double(sum) / Double(n)
}

/// Captures the main display (optionally a sub-rectangle) and keeps changed frames in order.
final class Capture: NSObject, SCStreamOutput {
    private var stream: SCStream?
    private let lock = NSLock()
    private var frames: [Frame] = []
    private(set) var latest: Frame?
    var recording = false
    var mode = ""
    var frameCount = 0

    /// Captures only the windows of the application whose process id is `pid` (nil: the whole display, calibration only).
    func start(rect: CGRect?, pid: Int32? = nil) async throws {
        var content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: false)
        var app: SCRunningApplication?
        var others: [SCRunningApplication] = []
        if let pid {
            for _ in 0..<25 {
                app = content.applications.first { $0.processID == pid }
                if app != nil { break }
                try await Task.sleep(nanoseconds: 20_000_000)
                content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: false)
            }
            // Before the test app has a window it is not listed: exclude every other application instead.
            if app == nil { others = content.applications.filter { $0.processID != pid } }
        }
        let displays = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true).displays
        guard let display = displays.first(where: { $0.displayID == CGMainDisplayID() }) ?? displays.first else {
            throw NSError(domain: "nvmbench", code: 1, userInfo: [NSLocalizedDescriptionKey: "no display: \(displays.map(\.displayID)) main=\(CGMainDisplayID())"])
        }
        let cfg = SCStreamConfiguration()
        let r = rect ?? CGRect(x: 0, y: 0, width: display.width, height: display.height)
        cfg.sourceRect = r
        cfg.width = Int(r.width)
        cfg.height = Int(r.height)
        cfg.minimumFrameInterval = CMTime(value: 1, timescale: 240)
        cfg.queueDepth = 8
        cfg.showsCursor = false
        cfg.pixelFormat = kCVPixelFormatType_32BGRA
        let filter: SCContentFilter
        if let app { filter = SCContentFilter(display: display, including: [app], exceptingWindows: []) }
        else if pid != nil { filter = SCContentFilter(display: display, excludingApplications: others, exceptingWindows: []) }
        else { filter = SCContentFilter(display: display, excludingWindows: []) }
        mode = app != nil ? "include-app" : (pid != nil ? "exclude-others" : "display")
        let s = SCStream(filter: filter, configuration: cfg, delegate: nil)
        try s.addStreamOutput(self, type: .screen, sampleHandlerQueue: DispatchQueue(label: "frames", qos: .userInteractive))
        try await s.startCapture()
        stream = s
    }

    func stop() async { try? await stream?.stopCapture() }

    func stream(_ stream: SCStream, didOutputSampleBuffer sb: CMSampleBuffer, of type: SCStreamOutputType) {
        guard let attachments = CMSampleBufferGetSampleAttachmentsArray(sb, createIfNecessary: false) as? [[SCStreamFrameInfo: Any]],
              let raw = attachments.first?[.status] as? Int, SCFrameStatus(rawValue: raw) == .complete,
              let pb = CMSampleBufferGetImageBuffer(sb) else { return }
        let t = CMTimeGetSeconds(CMSampleBufferGetPresentationTimeStamp(sb))
        CVPixelBufferLockBaseAddress(pb, .readOnly)
        let bpr = CVPixelBufferGetBytesPerRow(pb), h = CVPixelBufferGetHeight(pb), w = CVPixelBufferGetWidth(pb)
        let data = Data(bytes: CVPixelBufferGetBaseAddress(pb)!, count: bpr * h)
        CVPixelBufferUnlockBaseAddress(pb, .readOnly)
        let f = Frame(t: t, width: w, height: h, bytesPerRow: bpr, data: data)
        lock.lock()
        latest = f
        frameCount += 1
        if recording { frames.append(f) }
        lock.unlock()
    }

    func begin() { lock.lock(); frames = []; recording = true; lock.unlock() }
    func take() -> [Frame] { lock.lock(); let f = frames; frames = []; lock.unlock(); return f }
    func end() { lock.lock(); recording = false; frames = []; lock.unlock() }
    func current() -> Frame? { lock.lock(); defer { lock.unlock() }; return latest }
}

// MARK: clipboard

func clipboardState() -> (Int, String?) {
    let pb = NSPasteboard.general
    return (pb.changeCount, pb.string(forType: .string))
}

final class ClipboardSnapshot {
    private var items: [[(NSPasteboard.PasteboardType, Data)]] = []
    func save() {
        items = (NSPasteboard.general.pasteboardItems ?? []).map { it in it.types.compactMap { t in it.data(forType: t).map { (t, $0) } } }
    }
    func restore() {
        let pb = NSPasteboard.general
        pb.clearContents()
        let restored = items.map { pairs -> NSPasteboardItem in
            let it = NSPasteboardItem()
            for (t, d) in pairs { it.setData(d, forType: t) }
            return it
        }
        if !restored.isEmpty { pb.writeObjects(restored) }
    }
    var typeCount: Int { items.reduce(0) { $0 + $1.count } }

    /// Keeps the snapshot in a private file (mode 600) so an interrupted run can still restore it; never in artifacts.
    func save(toFile path: String) throws {
        let plain = items.map { pairs in Dictionary(uniqueKeysWithValues: pairs.map { ($0.0.rawValue, $0.1) }) }
        let data = try PropertyListSerialization.data(fromPropertyList: plain, format: .binary, options: 0)
        FileManager.default.createFile(atPath: path, contents: data, attributes: [.posixPermissions: 0o600])
    }

    func load(fromFile path: String) throws {
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        let plain = try PropertyListSerialization.propertyList(from: data, format: nil) as? [[String: Data]] ?? []
        items = plain.map { d in d.map { (NSPasteboard.PasteboardType($0.key), $0.value) } }
    }
}

// MARK: safety guard

let blockedOwners = ["SecurityAgent", "coreautha", "UserNotificationCenter", "universalAccessAuthWarn", "Keychain Access"]

/// Returns a reason when a system or security dialog is on screen, or when the test tree shows more than one window.
func unsafeToType(_ root: Int32) -> String? {
    guard let list = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] else { return "no window list" }
    for w in list {
        let owner = w[kCGWindowOwnerName as String] as? String ?? ""
        if blockedOwners.contains(where: { owner.contains($0) }) { return "dialog owner \(owner)" }
    }
    let n = treeWindows(root).count
    return n > 1 ? "test tree shows \(n) windows" : nil
}
