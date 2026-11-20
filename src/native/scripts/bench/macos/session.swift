// One process launch of one version: cold start, index readiness, memory phases and S1-S7 trials.
// Every trial, including warmups, failures and timeouts, is written to trials.jsonl.
import AppKit
import Foundation

struct SessionConfig: Codable {
    let version: String          // "v1" or "v2"
    let launch: Int              // launch index across the run
    let exe: String              // absolute path to the executable inside the verified bundle
    let args: [String]
    let home: String             // disposable HOME (already holds the fixture and config.json)
    let shortDir: String         // short disposable dir for XDG_RUNTIME_DIR and TMPDIR (socket path limit)
    let outDir: String
    let hotkey: String
    let actionKey: String        // "cmd+k"
    let warmups: Int
    let trials: Int
    let rootText: String         // text visible on the empty open palette
    let actionText: String       // text visible only in the action panel of a file row
    let settingsQuery: String
    let settingsText: String     // text visible only in the Settings subview
    let searchSecondRow: String  // version-specific second result row for the "settings" query (v1 Open Settings, v2 Window Management Settings)
    let timeoutS: Double
    let calibrateOnly: Bool
    let stopAfterIndex: Bool     // unmeasured first-run setup launch: start, index, quit
    let memoryOnly: Bool         // no keys and no capture: hidden-process memory phases only
    let memoryPhases: [String: Int]?  // phase name -> seconds, run in the order of memoryPhaseOrder
    let memoryPhaseOrder: [String]?
}

struct TrialRecord: Codable {
    let version: String; let launch: Int; let scenario: String; let index: Int; let warmup: Bool
    let tStart: Double; let tEnd: Double?; let latencyMs: Double?; let status: String
    let framesSeen: Int; let detail: String
    let load1: Double; let thermal: Int
    var lowerMs: Double? = nil
}

final class Session {
    let c: SessionConfig
    var pid: Int32 = 0
    let cap = Capture()
    let probeCap = Capture()
    var probeOn = false
    var probeStarted = false
    var sampler: MemorySampler!
    var trialsOut: FileHandle!
    var log: FileHandle!
    var palette: CGRect?
    var unsafe = false
    /// PTS of the last captured frame that did not yet show the endpoint: the latency lies in (lastMiss, endpoint].
    var lastMiss: Double?

    init(_ c: SessionConfig) { self.c = c }

    func note(_ s: String) {
        let line = String(format: "%.6f ", hostNow()) + s + "\n"
        log.write(line.data(using: .utf8)!)
        FileHandle.standardError.write(line.data(using: .utf8)!)
    }

    func record(_ r: TrialRecord) {
        let d = try! JSONEncoder().encode(r)
        trialsOut.write(d); trialsOut.write("\n".data(using: .utf8)!)
    }

    func loadAndThermal() -> (Double, Int) {
        var l = [Double](repeating: 0, count: 3)
        getloadavg(&l, 3)
        return (l[0], ProcessInfo.processInfo.thermalState.rawValue)
    }

    // MARK: launch / quit

    func spawn() -> Double {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: c.exe)
        p.arguments = c.args
        p.currentDirectoryURL = URL(fileURLWithPath: c.home)
        let env = ProcessInfo.processInfo.environment
        p.environment = [
            "HOME": c.home, "CFFIXED_USER_HOME": c.home, "XDG_RUNTIME_DIR": c.shortDir + "/xdg", "TMPDIR": c.shortDir + "/tmp/",
            "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "USER": env["USER"] ?? "", "LOGNAME": env["LOGNAME"] ?? "",
            "SHELL": "/bin/zsh", "LANG": "en_US.UTF-8",
        ]
        p.standardOutput = FileHandle(forWritingAtPath: c.outDir + "/app-stdout.log")
        p.standardError = FileHandle(forWritingAtPath: c.outDir + "/app-stderr.log")
        let t = hostNow()
        try! p.run()
        pid = p.processIdentifier
        try? "\(pid)\n".write(toFile: c.outDir + "/app.pid", atomically: true, encoding: .utf8)
        return t
    }

    func quit() {
        let tree = descendants(of: pid).keys
        kill(pid, SIGTERM)
        let deadline = Date().addingTimeInterval(8)
        while Date() < deadline && tree.contains(where: { kill($0, 0) == 0 }) { usleep(50_000) }
        let left = tree.filter { kill($0, 0) == 0 }
        for p in left { kill(p, SIGKILL) }
        note("quit: tree=\(tree.sorted()) sigkilled=\(left.sorted())")
    }

    // MARK: palette state

    func paletteVisible() -> Bool { !treeWindows(pid).isEmpty }

    var ocrCache: [Double: [TextLine]] = [:]
    func lines(_ f: Frame) -> [TextLine] {
        if let l = ocrCache[f.t] { return l }
        let l = ocr(f); ocrCache[f.t] = l; return l
    }
    func has(_ ls: [TextLine], _ needle: String) -> Bool { ls.contains { $0.text.lowercased().contains(needle.lowercased()) } }
    func settingsResults(_ ls: [TextLine]) -> Bool { has(ls, c.searchSecondRow) && ls.contains { $0.text.trimmingCharacters(in: .whitespaces) == "Settings" } }
    func typeSettingsAndWait() -> Bool {
        typeText(c.settingsQuery)
        cap.begin(); let ok = waitFrame(since: hostNow() - 1, timeout: c.timeoutS) { [self] f in settingsResults(lines(f)) }.0 != nil; cap.end()
        usleep(300_000); return ok
    }
    func resultRow(_ ls: [TextLine], _ id: String) -> Bool { ls.contains { $0.text.contains(id) && $0.text.lowercased().contains("txt") } }

    /// Waits for the first frame at or after `since` that satisfies `pred`. Frames are evaluated in arrival order.
    func waitFrame(since: Double, timeout: Double, _ pred: (Frame) -> Bool) -> (Double?, Int, Frame?) {
        var seen = 0
        lastMiss = nil
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            for f in cap.take() where f.t >= since {
                seen += 1
                if pred(f) { return (f.t, seen, f) }
                lastMiss = f.t
            }
            usleep(2000)
        }
        // A static screen produces no new frames: test the latest frame once more.
        if let f = cap.current(), f.t >= since, pred(f) { return (f.t, seen, f) }
        return (nil, seen, nil)
    }

    func snapshotNow(timeout: Double = 1) -> Frame? {
        cap.begin()
        let t = hostNow()
        usleep(60_000)
        let r = waitFrame(since: 0, timeout: timeout) { _ in true }
        cap.end()
        _ = t
        return r.2 ?? cap.current()
    }

    func savePNG(_ f: Frame?, _ name: String) {
        guard let f, let img = cgImage(f) else { return }
        let rep = NSBitmapImageRep(cgImage: img)
        try? rep.representation(using: .png, properties: [:])?.write(to: URL(fileURLWithPath: c.outDir + "/" + name + ".png"))
    }

    /// Unmeasured reset: hide the palette (Escape until no window), then open it on an empty root.
    func resetToRoot() -> Bool {
        hide()
        return openUnmeasured()
    }

    func hide() {
        for _ in 0..<4 where paletteVisible() {
            sendCombo("escape")
            usleep(250_000)
        }
        usleep(200_000)
    }

    func openUnmeasured() -> Bool {
        cap.begin()
        let t = sendCombo(c.hotkey)
        let r = waitFrame(since: t, timeout: c.timeoutS) { [self] f in has(lines(f), c.rootText) && !has(lines(f), "fixture") }
        cap.end()
        usleep(250_000)
        return r.0 != nil
    }

    func clearQuery() {
        sendCombo("cmd+a"); usleep(30_000); sendCombo("delete"); usleep(300_000)
    }

    // MARK: measured trial

    /// Logged OCR alias, shared with Linux: the font renders "AI" so that OCR often reads "Al"; both map to "ai".
    func normalizeTitle(_ t: String) -> String {
        let base = String(t.lowercased().filter { $0.isLetter || $0.isNumber || $0 == " " })
        return base.split(separator: " ").map { $0 == "al" ? "ai" : String($0) }.joined(separator: " ")
    }

    /// Stable-list signature, shared with Linux: the first nine row titles below the input row.
    func titlesSignature(_ f: Frame) -> String { rowTitles(lines(f)).map(normalizeTitle).joined(separator: "|") }

    /// Records every frame from dispatch to dispatch + holdS without OCR (so the observer does not compete with the
    /// app for CPU), then classifies them: R is the first frame of the final signature run; it must satisfy `until`
    /// and stay unchanged for at least 1 s before the capture ends. Lower bound: the latest frame or idle tick before R.
    @discardableResult
    func trial(_ scenario: String, _ i: Int, holdS: Double = 2.5, probeFlipsMs: [Double] = [], setup: () -> Bool, act: () -> Double, until: @escaping (Frame) -> Bool, stable: @escaping (Frame) -> String) -> Bool {
        let ok = setup()
        let (load, thermal) = loadAndThermal()
        let warm = i < c.warmups
        guard ok else {
            record(TrialRecord(version: c.version, launch: c.launch, scenario: scenario, index: i, warmup: warm, tStart: hostNow(), tEnd: nil, latencyMs: nil, status: "setup-failed", framesSeen: 0, detail: "", load1: load, thermal: thermal))
            return false
        }
        if let why = unsafeToType(pid) {
            record(TrialRecord(version: c.version, launch: c.launch, scenario: scenario, index: i, warmup: warm, tStart: hostNow(), tEnd: nil, latencyMs: nil, status: "aborted-unsafe", framesSeen: 0, detail: why, load1: load, thermal: thermal))
            note("ABORT \(scenario)[\(i)]: \(why)"); unsafe = true
            return false
        }
        let base = cap.current()
        cap.begin()
        let probing = probeOn && !probeFlipsMs.isEmpty
        if probing { probeCap.begin() }
        let tPre = hostNow()
        usleep(1_000_000)  // no-input control window for capture gaps
        let t0 = act()
        let flips = probing ? scheduleProbeFlips(afterMs: probeFlipsMs) : nil
        let tDispatchEnd = hostNow()
        while hostNow() < tDispatchEnd + holdS { usleep(5000) }
        let tCapEnd = hostNow()
        let allFrames = cap.take()
        let frames = allFrames.sorted { $0.t < $1.t }
        let measured = frames.filter { $0.t >= t0 }
        let allTicks = cap.takeTicks()
        let ticks = allTicks.filter { $0 >= t0 }
        let paletteEvents = allTicks + frames.map(\.t)
        let probeFrames = probing ? probeCap.take().map(\.t) : []
        if probing { probeCap.end() }
        cap.end()
        let preGap = largestGap(paletteEvents.filter { $0 < t0 }, from: tPre, to: t0)
        let gap = largestGap(paletteEvents, from: t0, to: tCapEnd)
        var gapText = String(format: " capture_gap_ms=%.1f capture_gap_at_ms=%.1f pre_dispatch_gap_ms=%.1f", gap.length * 1000, (gap.start - t0) * 1000, preGap.length * 1000)
        if let flips { gapText += " probe_flips=[" + probeFlipDelays(flips.times(), probeFrames: probeFrames, t0: t0) + "]" }
        let seq = measured.map { f in (t: f.t, sig: stable(f), ok: until(f), f: f) }
        var status = "ok"
        var r: Int?
        if seq.isEmpty {
            status = base.map(until) == true ? "no-change-already-at-endpoint" : "no-frame"
        } else {
            let finalSig = seq.last!.sig
            let rIdx = seq.lastIndex { $0.sig != finalSig }.map { $0 + 1 } ?? 0
            if !seq[rIdx].ok { status = "final-state-mismatch" }
            else if tCapEnd - seq[rIdx].t < 1.0 { status = "unstable" }
            else { r = rIdx }
        }
        let t1 = r.map { seq[$0].t }
        var tLower = t0
        if let r, let t1 {
            if r > 0 { tLower = max(tLower, seq[r - 1].t) }
            tLower = max(tLower, ticks.filter { $0 < t1 }.max() ?? t0)
        }
        let changes = seq.isEmpty ? 0 : (1..<seq.count).filter { seq[$0].sig != seq[$0 - 1].sig }.count
        let firstMatch = seq.first { $0.ok }.map { String(format: "%.1f", ($0.t - t0) * 1000) } ?? "-"
        var transitions: [String] = []
        for k in seq.indices where k == 0 || seq[k].sig != seq[k - 1].sig {
            transitions.append(String(format: "%.1fms=", (seq[k].t - t0) * 1000) + "\(seq[k].ok ? "match" : "miss"):" + String(seq[k].sig.prefix(120)))
        }
        let shown = (r.map { seq[$0].f } ?? seq.last?.f ?? base)
        let lat = t1.map { ($0 - t0) * 1000 }
        let lowerMs = t1 == nil ? nil : (tLower - t0) * 1000
        let lowerText: String = lowerMs.map { String(format: "%.1f", $0) } ?? "-"
        let dispatchText: String = String(format: "%.2f", (tDispatchEnd - t0) * 1000)
        let sequenceText: String = transitions.joined(separator: " >> ")
        let shownText: String = shown.map { lines($0).map(\.text).prefix(12).joined(separator: " | ") } ?? ""
        var txt = "lower_bound_ms=\(lowerText) first_match_ms=\(firstMatch) late_changes=\(changes)"
        txt += " frames=\(seq.count) idle_ticks=\(ticks.count) dispatch_ms=\(dispatchText) sequence=[\(sequenceText)]"
        txt += gapText + " last_text=" + shownText
        record(TrialRecord(version: c.version, launch: c.launch, scenario: scenario, index: i, warmup: warm, tStart: t0, tEnd: t1, latencyMs: lat, status: status, framesSeen: seq.count, detail: txt, load1: load, thermal: thermal, lowerMs: lowerMs))
        if i == c.warmups { savePNG(shown, "\(scenario)-endpoint") }
        if t1 == nil { savePNG(shown, "\(scenario)-\(i)-failed") }
        ocrCache.removeAll()
        note("\(scenario)[\(i)] \(lat.map { String(format: "%.1f ms", $0) } ?? status) frames=\(seq.count) ticks=\(ticks.count)")
        return t1 != nil
    }

    // MARK: run

    func run() {
        FileManager.default.createFile(atPath: c.outDir + "/trials.jsonl", contents: nil)
        FileManager.default.createFile(atPath: c.outDir + "/session.log", contents: nil)
        FileManager.default.createFile(atPath: c.outDir + "/app-stdout.log", contents: nil)
        FileManager.default.createFile(atPath: c.outDir + "/app-stderr.log", contents: nil)
        trialsOut = FileHandle(forWritingAtPath: c.outDir + "/trials.jsonl")
        log = FileHandle(forWritingAtPath: c.outDir + "/session.log")
        let sem = DispatchSemaphore(value: 0)

        // Cold process start: spawn, then press the hotkey every 100 ms until the palette paints its root.
        _ = spawn()
        sampler = MemorySampler(root: pid, path: c.outDir + "/memory.csv", intervalMs: 100)
        note("spawned pid=\(pid) exe=\(c.exe)")
        sampler.phase = "launch-settle"; sleep(8)
        if paletteVisible() { note("palette visible at launch: Escape, then 2 s settle"); hide(); sampler.phase = "hide-settle"; sleep(2) }
        let hiddenBefore = !paletteVisible()
        sampler.phase = "hidden"; sleep(2)
        sampler.phase = "post-hidden"
        let hiddenAfter = !paletteVisible()
        record(TrialRecord(version: c.version, launch: c.launch, scenario: "hidden-check", index: 0, warmup: false, tStart: 0, tEnd: nil, latencyMs: nil, status: hiddenBefore && hiddenAfter ? "ok" : "palette-visible", framesSeen: 0, detail: "before=\(hiddenBefore) after=\(hiddenAfter)", load1: 0, thermal: 0))
        var capErr: Error?
        Task { do { try await cap.start(rect: nil, pid: pid) } catch { capErr = error }; sem.signal() }
        sem.wait()
        if let capErr { note("capture failed: \(capErr)"); finish(); return }
        note("capture mode=\(cap.mode)")
        // First open: one hotkey press, then wait for the window and the root text; press again only after 10 s.
        var attempts = 0
        let tFirstKey = hostNow()
        var tPaint: Double?
        let startDeadline = hostNow() + 60
        var tPress = 0.0
        while tPaint == nil && hostNow() < startDeadline {
            if !paletteVisible() && hostNow() - tPress > 10 { tPress = sendCombo(c.hotkey); attempts += 1 }
            usleep(200_000)
            if paletteVisible(), let f = snapshotNow(), has(lines(f), c.rootText) { tPaint = f.t }
        }
        let (load, thermal) = loadAndThermal()
        record(TrialRecord(version: c.version, launch: c.launch, scenario: "first-open", index: 0, warmup: false, tStart: tFirstKey, tEnd: tPaint, latencyMs: tPaint.map { ($0 - tFirstKey) * 1000 }, status: tPaint == nil ? "timeout" : "ok", framesSeen: attempts, detail: "hotkey attempts=\(attempts) coarse (200 ms polling)", load1: load, thermal: thermal))
        note("first open=\(tPaint.map { String(format: "%.0f ms", ($0 - tFirstKey) * 1000) } ?? "TIMEOUT") attempts=\(attempts)")
        palette = treeWindows(pid).first
        savePNG(snapshotNow(), "cold-start-root")
        if tPaint == nil || c.calibrateOnly { finish(); return }

        // Restart capture on the palette area only (smaller frames, faster OCR).
        Task { await cap.stop(); try! await cap.start(rect: palette!.insetBy(dx: -40, dy: -40), pid: pid); sem.signal() }
        sem.wait()
        sleep(1)

        // Search readiness (coarse, unmeasured): the shared "settings" query shows its two expected rows.
        sampler.phase = "search-ready-wait"
        let tIdx = hostNow()
        var indexed = false
        while !indexed && hostNow() - tIdx < 60 {
            typeText(c.settingsQuery)
            cap.begin()
            indexed = waitFrame(since: hostNow() - 0.5, timeout: 2) { [self] f in settingsResults(lines(f)) }.0 != nil
            cap.end()
            clearQuery()
        }
        record(TrialRecord(version: c.version, launch: c.launch, scenario: "search-ready", index: 0, warmup: false, tStart: tIdx, tEnd: indexed ? hostNow() : nil, latencyMs: nil, status: indexed ? "ok" : "timeout", framesSeen: 0, detail: "unmeasured readiness gate", load1: 0, thermal: 0))
        note("search ready=\(indexed)")
        if !indexed { finish(); return }
        if c.stopAfterIndex { hide(); sleep(3); finish(); return }

        hide()
        sleep(1)

        let n = c.warmups + c.trials
        sampler.phase = "S1-reopen"
        for i in 0..<n {
            let opened = trial("S1-hotkey-open", i, setup: { hide(); return !paletteVisible() }, act: { sendCombo(c.hotkey) },
                  until: { [self] f in has(lines(f), c.rootText) }, stable: { [self] f in titlesSignature(f) })
            if opened && i >= c.warmups { sampler.phase = "open-\(i)"; sleep(2); sampler.phase = "S1-reopen" }
        }
        sampler.phase = "S2-search"
        for i in 0..<n {
            trial("S2-search", i, holdS: 5.0, setup: resetToRoot, act: { typeText(c.settingsQuery) }, until: { [self] f in settingsResults(lines(f)) }, stable: { [self] f in titlesSignature(f) })
        }
        sampler.phase = "S3-clear"
        for i in 0..<n {
            trial("S3-query-clear", i, setup: { [self] in
                guard resetToRoot() else { return false }
                return typeSettingsAndWait()
            }, act: { let t = sendCombo("cmd+a"); usleep(30_000); sendCombo("delete"); return t },
                  until: { [self] f in let l = lines(f); return has(l, c.rootText) && !has(l, c.searchSecondRow) }, stable: { [self] f in titlesSignature(f) })
        }
        sampler.phase = "S4-rownav"
        for i in 0..<n {
            var base: Frame?
            var bands: [(Int, Int)] = []
            trial("S4-row-down", i, setup: { [self] in
                guard resetToRoot() else { return false }
                guard typeSettingsAndWait() else { return false }
                usleep(700_000)
                base = snapshotNow()
                guard let b = base else { return false }
                let ls = lines(b)
                let top = ls.filter { $0.text.trimmingCharacters(in: .whitespaces) == "Settings" }.sorted { $0.box.minY < $1.box.minY }.prefix(1)
                let second = ls.filter { $0.text.contains(c.searchSecondRow) }.prefix(1)
                let rows = Array(top) + Array(second)
                guard rows.count >= 2 else { return false }
                bands = rows.prefix(2).map { (Int($0.box.minY) - 6, Int($0.box.maxY) + 6) }
                return true
            }, act: { sendCombo("down") }, until: { f in
                guard let b = base else { return false }
                return bandDiff(f, b, y0: bands[0].0, y1: bands[0].1) > 4 && bandDiff(f, b, y0: bands[1].0, y1: bands[1].1) > 4
            }, stable: { [self] f in
                guard let b = base else { return "" }
                return "\(bandDiff(f, b, y0: bands[0].0, y1: bands[0].1) > 4)\(bandDiff(f, b, y0: bands[1].0, y1: bands[1].1) > 4)|" + titlesSignature(f)
            })
        }
        sampler.phase = "S6-subview"
        for i in 0..<n {
            trial("S6-subview-open", i, setup: { [self] in
                guard resetToRoot() else { return false }
                return typeSettingsAndWait()
            }, act: { sendCombo("return") }, until: { [self] f in has(lines(f), c.settingsText) }, stable: { [self] f in titlesSignature(f) })
            usleep(300_000)
            trial("S6-subview-back", i, setup: { [self] in has(lines(snapshotNow()!), c.settingsText) }, act: { sendCombo("escape") },
                  until: { [self] f in let l = lines(f); return !has(l, c.settingsText) && (has(l, c.rootText) || has(l, c.settingsQuery)) }, stable: { [self] f in titlesSignature(f) })
        }
        sampler.phase = "S7-calculator"
        for i in 0..<n {
            var before = 0
            let (load, thermal) = loadAndThermal()
            guard resetToRoot() else {
                record(TrialRecord(version: c.version, launch: c.launch, scenario: "S7-calc-copy", index: i, warmup: i < c.warmups, tStart: hostNow(), tEnd: nil, latencyMs: nil, status: "setup-failed", framesSeen: 0, detail: "", load1: load, thermal: thermal))
                continue
            }
            let operand = 5678 + c.launch * 100 + i
            let expected = String(1234 * operand)
            typeText("1234*\(operand)")
            cap.begin()
            let shown = waitFrame(since: hostNow() - 1, timeout: c.timeoutS) { [self] f in lines(f).contains { $0.text.filter(\.isNumber).contains(expected) } }.0 != nil
            cap.end()
            usleep(300_000)
            if let why = unsafeToType(pid) {
                record(TrialRecord(version: c.version, launch: c.launch, scenario: "S7-calc-copy", index: i, warmup: i < c.warmups, tStart: hostNow(), tEnd: nil, latencyMs: nil, status: "aborted-unsafe", framesSeen: 0, detail: why, load1: load, thermal: thermal))
                unsafe = true; break
            }
            let sentinel = "nvmbench-sentinel-\(UUID().uuidString)"
            NSPasteboard.general.clearContents(); NSPasteboard.general.setString(sentinel, forType: .string)
            let sentinelOK = clipboardState().1 == sentinel
            if !sentinelOK {
                record(TrialRecord(version: c.version, launch: c.launch, scenario: "S7-calc-copy", index: i, warmup: i < c.warmups, tStart: hostNow(), tEnd: nil, latencyMs: nil, status: "sentinel-failed", framesSeen: 0, detail: "", load1: load, thermal: thermal))
                continue
            }
            before = clipboardState().0
            let t0 = sendCombo("return")
            var t1: Double?
            var text = ""
            while t1 == nil && hostNow() - t0 < c.timeoutS {
                let (cc, s) = clipboardState()
                // An app may clear the pasteboard before writing: an empty read after a change keeps polling.
                if cc != before, let s, !s.isEmpty { text = s; if text.filter(\.isNumber) == expected { t1 = hostNow() } else { break } }
                usleep(500)
            }
            let status = !shown ? "setup-failed" : (t1 == nil ? (text.isEmpty ? "timeout" : "wrong-clipboard") : "ok")
            record(TrialRecord(version: c.version, launch: c.launch, scenario: "S7-calc-copy", index: i, warmup: i < c.warmups, tStart: t0, tEnd: t1, latencyMs: t1.map { ($0 - t0) * 1000 }, status: status, framesSeen: 0, detail: "expected=\(expected) clipboard=\(text) prev_change=\(before)", load1: load, thermal: thermal))
            note("S7[\(i)] \(status) \(t1.map { String(format: "%.1f ms", ($0 - t0) * 1000) } ?? "") clip=\(text)")
        }
        // Slow typing memory workload (shared label with Linux): "settings" at 100 ms per character from a verified
        // empty root; accepted samples are those whose read lies entirely inside [first key, last key]. Trial 0 is warmup.
        for i in 0..<4 {
            let rootOK = resetToRoot()
            savePNG(snapshotNow(), "slowtype-\(i)-pre-root")
            sampler.phase = "slow-typing-\(i)"
            let (k0, k1) = typeSlowly(c.settingsQuery, delayMs: 100)
            sampler.phase = "slow-typing-after"
            cap.begin()
            let done = waitFrame(since: k1 - 0.05, timeout: c.timeoutS) { [self] f in let l = lines(f); return has(l, c.searchSecondRow) && l.contains { $0.text.trimmingCharacters(in: .whitespaces) == "Settings" } }.0 != nil
            cap.end()
            savePNG(snapshotNow(), "slowtype-\(i)-post")
            record(TrialRecord(version: c.version, launch: c.launch, scenario: "slow-typing-memory", index: i, warmup: i == 0, tStart: k0, tEnd: k1, latencyMs: nil, status: rootOK && done ? "ok" : "precondition-or-outcome-failed", framesSeen: 0, detail: "root=\(rootOK) settings_visible=\(done) phase=slow-typing-\(i)", load1: loadAndThermal().0, thermal: loadAndThermal().1))
        }
        hide()
        let afterBefore = !paletteVisible()
        sampler.phase = "after-cycle"; sleep(3)
        sampler.phase = "final"
        let afterAfter = !paletteVisible()
        runProbeDiagnostics()
        record(TrialRecord(version: c.version, launch: c.launch, scenario: "after-cycle-check", index: 0, warmup: false, tStart: 0, tEnd: nil, latencyMs: nil, status: afterBefore && afterAfter ? "ok" : "palette-visible", framesSeen: 0, detail: "before=\(afterBefore) after=\(afterAfter)", load1: 0, thermal: 0))
        finish()
    }

    func finish() {
        sampler.stop()
        let s = "{\"peak_phys_footprint_bytes\":\(sampler.peakFootprint),\"peak_rss_bytes\":\(sampler.peakRSS)}\n"
        try? s.write(toFile: c.outDir + "/peaks.json", atomically: true, encoding: .utf8)
        quit()
        let sem = DispatchSemaphore(value: 0)
        Task { await cap.stop(); if probeStarted { await probeCap.stop() }; sem.signal() }
        sem.wait()
        try? trialsOut.close()
        try? log.close()
    }
}

extension Session {
    /// Hidden-process memory only: no window is opened and no key is sent. Phases run back to back from spawn.
    func runMemoryOnly() {
        for f in ["/session.log", "/trials.jsonl", "/app-stdout.log", "/app-stderr.log"] { FileManager.default.createFile(atPath: c.outDir + f, contents: nil) }
        trialsOut = FileHandle(forWritingAtPath: c.outDir + "/trials.jsonl")
        log = FileHandle(forWritingAtPath: c.outDir + "/session.log")
        let tSpawn = spawn()
        sampler = MemorySampler(root: pid, path: c.outDir + "/memory.csv", intervalMs: 100)
        note("spawned pid=\(pid) memory-only locked=\(screenLocked())")
        for name in c.memoryPhaseOrder ?? [] {
            sampler.phase = name
            let (load, thermal) = loadAndThermal()
            note("phase \(name) at +\(String(format: "%.1f", hostNow() - tSpawn)) s load1=\(load) thermal=\(thermal) locked=\(screenLocked())")
            sleep(UInt32(c.memoryPhases?[name] ?? 0))
        }
        sampler.stop()
        let s = "{\"peak_phys_footprint_bytes\":\(sampler.peakFootprint),\"peak_rss_bytes\":\(sampler.peakRSS)}\n"
        try? s.write(toFile: c.outDir + "/peaks.json", atomically: true, encoding: .utf8)
        quit()
        try? trialsOut.close(); try? log.close()
    }
}

func runSession(configPath: String) {
    let c = try! JSONDecoder().decode(SessionConfig.self, from: Data(contentsOf: URL(fileURLWithPath: configPath)))
    let clip = ClipboardSnapshot()
    clip.save()
    Thread.detachNewThread {
        if c.memoryOnly { Session(c).runMemoryOnly(); exit(0) }
        Session(c).run()
        clip.restore()
        exit(0)
    }
    NSApplication.shared.setActivationPolicy(.prohibited)
    NSApplication.shared.run()
}
