// Probe diagnostics after the measured trials: three Down and three Enter attempts judged by pixel change only (no
// OCR), with five probe flips between +150 and +550 ms. Not exported as measurements.
import Foundation

extension Session {
    func runProbeDiagnostics() {
        sampler.phase = "probe-diagnostics"
        let probeRect = startProbeWindow()
        let sem = DispatchSemaphore(value: 0)
        Task { do { try await probeCap.start(rect: probeRect, pid: getpid()); probeOn = true } catch { note("probe capture failed: \(error)") }; sem.signal() }
        sem.wait()
        probeStarted = probeOn
        note("probe mode=\(probeCap.mode) on=\(probeOn)")
        guard probeOn else { return }
        let flipsMs: [Double] = [150, 250, 350, 450, 550]
        for i in 0..<3 {
            var base: Frame?
            var bands: [(Int, Int)] = []
            let moved = { (f: Frame) -> Bool in
                guard let b = base, bands.count == 2 else { return false }
                return bandDiff(f, b, y0: bands[0].0, y1: bands[0].1) > 4 && bandDiff(f, b, y0: bands[1].0, y1: bands[1].1) > 4
            }
            trial("D4-row-down-probe", 100 + i, probeFlipsMs: flipsMs, setup: { [self] in
                guard resetToRoot(), typeSettingsAndWait() else { return false }
                usleep(700_000)
                base = snapshotNow()
                guard let b = base else { return false }
                let ls = lines(b)
                let top = ls.filter { $0.text.trimmingCharacters(in: .whitespaces) == "Settings" }.sorted { $0.box.minY < $1.box.minY }.prefix(1)
                let second = ls.filter { $0.text.contains(c.searchSecondRow) }.prefix(1)
                bands = (Array(top) + Array(second)).prefix(2).map { (Int($0.box.minY) - 6, Int($0.box.maxY) + 6) }
                return bands.count == 2
            }, act: { sendCombo("down") }, until: moved, stable: { "\(moved($0))" })
        }
        for i in 0..<3 {
            var base: Frame?
            let changed = { (f: Frame) -> Bool in base.map { bandDiff(f, $0, y0: 0, y1: f.height) > 6 } ?? false }
            trial("D6-subview-open-probe", 100 + i, probeFlipsMs: flipsMs, setup: { [self] in
                guard resetToRoot(), typeSettingsAndWait() else { return false }
                base = snapshotNow()
                return base != nil
            }, act: { sendCombo("return") }, until: changed, stable: { "\(changed($0))" })
        }
        hide()
    }
}
