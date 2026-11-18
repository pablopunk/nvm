# Nevermind v1 and v2: November 2026 measurements

These figures compare packaged v1 2026.1031.2 with v2's b8632d92 source tree. Compare versions within each environment. The machines and memory metrics differ; this is not a macOS-versus-Linux performance test.

## Linux

Four effective CPUs, 4 GiB LXC, Xvfb/Openbox at 1280×800, software rendering, private D-Bus and offline network namespace. Separate profiles contained identical 51,000-file fixture trees. Default indexing policies were retained: this does not mean both versions indexed 51,000 entries. Ten process blocks per version alternated AB/BA order. Each block retained one warmup and three measured interaction trials.

Memory figures are medians of ten process-block medians, summing current descendants' PSS. The sampler records read-start/read-end times and phase generations and rejects whole incomplete or crossing snapshots. Approximately 100 ms sampling can miss short-lived or reparented processes. Slow typing is a separate workload at 100 ms per character. RSS is retained in the raw evidence but can double-count shared pages.

Visual timing captures a 720×295 region before and during injection. Captures go into preallocated memory; PNG encoding and OCR occur afterward. Median cadence was 10.0 ms, p95 10.04 ms; capture duration median 0.323 ms. Reported bounds span the last negative capture start to the first positive capture end, relative to key dispatch start. They bound observed recognized content rather than exact application execution or display scanout. Settings recognition requires the typed query and the first two expected result rows; secondary row titles differ between versions. Clear requires the verified query followed by recognized root content. There were 30 successful measured attempts for each endpoint and version. Percentiles use nearest rank on the lower and upper bounds separately.

The separately reviewed calculator run verifies a unique clipboard sentinel before Enter and a varying expected result after Enter. The expression was already entered and settled. Times include injection, clipboard reads and 10 ms polling sleeps; they are not expression-to-result latency. Thirty measured attempts across ten process blocks succeeded per version. Median/p95: v1 66.58/84.81 ms; v2 16.13/16.69 ms. The nearest-rank p95 is the 29th sorted observation.

Warm root recognition was unstable in v1 (about 70–830 ms); it is excluded from the latency summary rather than converted to a single speedup ratio. Clean startup and matched menu timing were not established. No earlier coarse-observer timing is included.

Observer idle calibration consumed about 9.5% of one CPU plus 2.8% for Xvfb. During collection, combined observation/encoding/sampling CPU varied. No overhead was subtracted. The host's low-swap safety guard was overridden with user approval. Block-start load ranged 0.85–2.53; no recorded cgroup throttling does not prove a quiet host. These are environment-specific observations, not universal performance promises.

Linux archive SHA-256: v1 916b43cc87b556f983a562c849405aea644ac559e20c8f09fd66c7bdbd6ff655; v2 c13355ac004a42677de574dd5c9bbdd23ed9c85e3c9a3d5cc56fce330db3c599. The v2 artifact is native-linux-amd64, ID 14354039676, run 44142967399. It embeds PR merge SHA 1bc8f8345d8f9b8fb2dddcfbbc1fd3f52c37e697; that commit and b8632d92 have the same tree, 6a2f54c342074bb5ba2d65730c1c9996b13e4d9e.

## M4 Pro

Apple M4 Pro, 48 GB, macOS 27.0.1, AC power. Ten alternating paired blocks used fresh disposable profiles, each primed by an unmeasured 60-second first launch before the measured launch. The session was locked/asleep and no palette interaction was measured. The user's normal Nevermind Next remained running; other host activity and load varied. The harness's session predicate combines lock or display sleep.

The chart reports median per-launch hidden-idle process-tree physical footprint: v1 219.90 MiB (range 218.83–222.26), v2 84.70 MiB (73.69–89.41). Physical footprint is not Linux PSS. Current-descendant sampling can miss transient processes. Three incomplete v1 startup snapshots do not affect hidden-idle figures; incomplete snapshots should be excluded from startup summaries. Any launch maxima are sampled maxima, not true peaks. The Mac review verified all 509 manifest entries and recomputed the raw phase statistics. Recorded keychain search-list/default settings were restored; this is not a claim of isolation from every shared OS service.

The v1 executable SHA-256 is 0577e2f6aa8c48b3ddad5c28b68bdf45c7495336a7a2b1df605dbfdc9bbe6d42; v2 is ad9665f8c26a70267864376b779f0ca33b67aab0e865887efc63c5d51a9a4d3e. Both passed strict signature verification. Recorded v2 provenance associates it with b8632d92. Active Mac interaction measurements remain outstanding.

## Review of the Linux replacement

The coordinator independently hashed all 23,631 high-cadence manifest entries and all 196 slow-typing entries: zero mismatches. Raw JSONL recomputation reproduced the published block-median arithmetic. Every accepted snapshot was checked against the raw sample, stable phase generation, process-read completion and corresponding event-time bounds. Both fixture manifests have 51,000 entries and digest 18133a583130f4acdde93f87ee9f275acc4e832393c15d44039894c4e252b6fe. Interval endpoints and trial counts were checked against raw frame timestamps. The coordinator inspected classifier code and representative negative/positive captures, including the slowest v1 open and search cases; it did not rerun OCR for every frame. Public figures retain the limitations above.

Raw evidence remains preserved in the task's Linux and Mac benchmark directories. Earlier exploratory runs are kept separately and are not used in these charts.
