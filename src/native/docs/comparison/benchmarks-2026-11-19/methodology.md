# Nevermind v1 and v2: 19 November 2026 measurements

These charts compare packaged v1 2026.1031.2 (Electron) with the packaged v2 build of PR head 71b918bf. Compare versions within one platform: the machines and memory metrics differ, so this is not a Linux-versus-macOS test.

`charts.py` regenerates every chart from the per-platform folders under `data/`. Each platform folder is optional; a chart shows every platform it finds, side by side.

```sh
python3 src/native/docs/comparison/benchmarks-2026-11-19/charts.py
```

To add macOS, copy its `trials.jsonl`, `memory.csv` and `run.json` into `data/macos/` and run the same command. It needs Python 3.9+ and Chromium (`CHROMIUM=/path/to/chrome` overrides the binary; `--no-png` writes only the HTML sources).

## Reading the charts

The images carry only the comparison; every caveat lives in this document.

- Each bar is one median. Lower is better on both charts. Each platform panel has its own scale, shared by all of its rows.
- A timing bar is the midpoint of the median capture interval: the median lower bound and the median upper bound, averaged. The calculator is a point measurement, so its bar is the median itself. Medians use successful measured attempts only; warmups and failures are left out. The intervals, p95 upper bounds and success counts are in the tables below and in each bar's hover text in the HTML source.
- A memory bar is the median of the ten per-block medians. The block range is in the tables below and in the hover text.
- An "N× faster" tag appears when 1.x takes at least 1.5 times as long as 2.x and at least 20 ms longer (two capture intervals). An "N× less" tag appears when 1.x uses at least 1.5 times as much memory. No tag means the gap is too small to call, not that 2.x lost.
- The 0–12 ms results (one capture interval) show as 6 ms; they mean the change was visible in the first capture after dispatch.
- Linux memory is PSS and macOS memory is physical footprint, on different machines. Compare 1.x with 2.x within one panel, never Linux with macOS.
- Each platform's machine, capture method and run-specific limits are described in its section below. A platform added through `data/` needs its own section here; its `run.json` notes are not drawn in the images.

## Input format

Put each platform's results in `data/<platform>/`, where the platform is `linux` or `macos`. The folder holds three files.

### `trials.jsonl`

One JSON object per line, one line per attempt. Include warmups and failures; the charts drop warmups and count failures.

| Field | Type | Meaning |
| --- | --- | --- |
| `platform` | string | `linux` or `macos`; must match the folder |
| `version` | string | `v1` or `v2` |
| `block` | integer | process block, 0–9; one app launch per version per block |
| `trial` | integer | attempt within the block; 0 is the warmup |
| `warmup` | boolean | `true` for trial 0 |
| `endpoint` | string | one of the endpoints below |
| `success` | boolean | the attempt met the endpoint's rule |
| `lower_ms` | number or null | lower bound, ms after key dispatch start; null on failure |
| `upper_ms` | number or null | upper bound, ms after key dispatch start; null on failure |
| `reason` | string or null | why the attempt failed |

Extra fields are allowed and ignored. For a point measurement, such as the calculator, set `lower_ms` equal to `upper_ms`.

| `endpoint` | Action | End of the action |
| --- | --- | --- |
| `search` | type `settings` into the empty root query | the first frame after the last change of the visible result list, held at least 1 s, showing the expected Settings results |
| `clear` | select all and delete the `settings` query | the same stable-list rule, showing the root list |
| `row_down` | Down arrow on the Settings results | the same rule, with the second row selected |
| `settings_open` | Enter on the Settings row | the same rule, showing the Settings view |
| `settings_back` | Escape in the Settings view | the same rule, showing where each version returns |
| `calculator` | Enter on an entered calculator expression | the clipboard first holds the expected result; a unique sentinel was read back before Enter |

A platform may omit any endpoint; the chart then shows a dash for it. Visual bounds are capture intervals: `lower_ms` is the start of the last capture before the final stable state, or dispatch start if that is later; `upper_ms` is the end of the first capture showing it.

### `memory.csv`

A header row, then one row per version, block and phase. Each value is the median of the complete process-tree snapshots accepted in that block and phase. The chart reports the median of the block values; the range is in the hover text.

| Column | Meaning |
| --- | --- |
| `platform` | `linux` or `macos` |
| `version` | `v1` or `v2` |
| `block` | process block, 0–9 |
| `phase` | `hidden`, `open`, `slow_typing` or `after_cycle` (other phases are kept but not charted) |
| `metric` | `pss` on Linux, `phys_footprint` on macOS |
| `value_mib` | block median, MiB (1,048,576 bytes) |
| `samples` | accepted snapshots behind the median |

Extra columns, such as `rss_mib`, are allowed and ignored.

| `phase` | State sampled |
| --- | --- |
| `hidden` | palette hidden for 2 s after launch settled, before the first interaction |
| `open` | palette open and idle for 2 s after a verified open, measured trials only |
| `slow_typing` | snapshots read wholly while `settings` is typed at 100 ms per character, measured trials only |
| `after_cycle` | palette hidden for 3 s after all trials and slow typing, hidden state verified |

### `run.json`

Facts about the run. The charts draw only `title`, as the panel heading (text in parentheses is shown muted); the rest records the run alongside the data and belongs in the platform's section below.

```json
{
  "platform": "macos",
  "title": "macOS",
  "machine": "Apple M4 Pro, 48 GB, macOS 27.0.1, AC power.",
  "memory_metric": "physical footprint",
  "collected": "20 Nov 2026",
  "v1_label": "v1 · 2026.1031.2",
  "v2_label": "v2 · 71b918bf",
  "attempts_note": "30 across 10 blocks",
  "notes": ["One or two sentences of conditions and limits."]
}
```

## Linux

Collected 19 November 2026, 18:45–19:34 UTC, on a disposable Debian LXC: 4 effective CPUs of an AMD Ryzen 7 PRO 8845HS host, 4 GiB RAM, no guest swap. Xvfb 1280×800 with Openbox and Mesa software rendering; each app ran in an offline network namespace with a private D-Bus session.

Each version had its own fresh home with the same portable `Alt+Shift+Space` hotkey and an identical 51,000-file fixture (manifest digest 18133a583130f4acdde93f87ee9f275acc4e832393c15d44039894c4e252b6fe). Each profile had one unmeasured 60-second priming launch. Then ten process blocks per version ran in alternating AB/BA order. A block is one launch: 8 s to settle, hidden 2 s, then one warmup and three measured trials, then slow typing and a hidden after-cycle phase.

One trial runs: open, type `settings`, Down, Up (not measured), clear, type `settings` again (not measured), Enter on Settings, Escape, then the calculator. Every visual action is captured from 150 ms before dispatch until 2.5 s after it (5 s for search). Capture uses XGetImage on a 720×570 region into preallocated memory at a 10 ms target; PNG encoding and OCR happen afterwards. Over 148,022 intervals the median cadence was 10.0 ms (p95 10.03, maximum 15.6); the median capture took 1.68 ms; there were no capture errors.

Offline Tesseract OCR reads the query and the first nine row titles of each distinct frame; a pixel test finds the selected row's Enter glyph. The expected Settings result lists were checked by hand against calibration screenshots of both builds. A near-miss OCR line maps to a known title only at similarity 0.9 or above. The end of each action follows the stable-list rule in the input format above. Precondition and final-state checks: search starts from the root and ends on the expected list; Down ends with row 2 selected; clear ends with no query and the Welcome row visible; Enter ends in the Settings view; Escape ends where each version returns (v1 to the Settings results, v2 to the root).

Every measured endpoint succeeded 30 of 30 times for both versions. The first classification pass required the Welcome row first and a fixed placeholder, but the root list reorders with use (recent Settings, clipboard history) and the placeholder rotates. That pass failed every root-dependent attempt after block 0. The predicate was relaxed to "no Settings query and the Welcome row visible", and the same frames were reclassified. Both passes are kept with the raw data.

| Endpoint, ms | v1 median interval | v1 p95 upper | v2 median interval | v2 p95 upper |
| --- | --- | --- | --- | --- |
| Search to stable list | 90–102 | 112 | 20–32 | 32 |
| Clear to stable root | 80–92 | 104 | 50–62 | 62 |
| Down to row 2 selected | 0–12 | 22 | 0–12 | 22 |
| Enter to Settings view | 10–22 | 22 | 0–12 | 12 |
| Escape out of Settings | 0–12 | 22 | 0–12 | 32 |
| Calculator Enter to clipboard | 68 | 81 | 16 | 17 |

Bounds start at key dispatch start and include xdotool injection. Median dispatch took 9.4 ms for typing `settings`, 14.7 ms for one key and 39.7 ms for Ctrl+A then Backspace. Results within one capture interval (0–12 ms) mean the change was visible in the first capture after dispatch began. The calculator figure is the return of the first clipboard read that held the result; reads poll every 10 ms plus `xclip` time.

Warm open (hotkey to a stable root list) is captured but not charted. In v1 it took 220–232 ms at the median, with a p95 upper bound of 762 ms, and one attempt did not hold for 1 s. In v2 it took 40–52 ms, with a p95 of 52 ms. As in earlier runs, v1 is too unstable here for a single figure.

Memory sums `Pss` from `/proc/<pid>/smaps_rollup` for the live process tree about every 100 ms. A snapshot counts only if every process read succeeded, its phase generation did not change during the read, and the phase's visual state was verified (the trial's open or search succeeded, or the slow-typing pre and post screenshots, or the after-cycle hidden screenshot). Slow typing counts only reads that lie wholly inside the 100 ms-per-character dispatch. Every version, phase and block had at least 9 accepted snapshots.

| PSS, MiB | v1 | v1 block range | v2 | v2 block range |
| --- | --- | --- | --- | --- |
| Hidden, settled | 483 | 481–486 | 143 | 131–146 |
| Palette open, settled | 457 | 452–460 | 142 | 139–143 |
| Slow typing | 535 | 520–557 | 142 | 140–144 |
| Hidden after a full cycle | 565 | 526–581 | 141 | 139–143 |

RSS sums are kept in `memory.csv`; they double-count shared pages.

## Build provenance

| Item | Value |
| --- | --- |
| v1 package | `Nevermind-2026.1031.2-linux-amd64.deb` from release v2026.1031.2, SHA-256 916b43cc87b556f983a562c849405aea644ac559e20c8f09fd66c7bdbd6ff655 (matches the release asset digest) |
| v1 executable | `opt/Nevermind/com.pablopunk.nvm`, SHA-256 39fe53f282b977e69fe10ba0c063f65c7eff3254f674c6fed01606d3c8d5eea7 |
| v2 source | PR head 71b918bfdec02a585c777c844ba29c327e6aa606 |
| v2 CI run | Native packages run 44573477996, job "Linux amd64" 135734563725, all four platform jobs green |
| v2 artifact | `native-linux-amd64`, ID 14553904851, zip SHA-256 c5505c84802c3bde185b21447e8129cba3707b4c610c841167fe8e0f8b06ce0c (matches the digest GitHub recorded at upload) |
| v2 package | `nevermind-next_2.0.0~next.0_amd64.deb`, SHA-256 c494831b7471a043f417096f9224138a9d3333cafb9bf901c3ca2762d88f8405; tarball e61a10301a3279a48b54faa0f7ec27d7fefb597e39312347678db23d223561b7; both match the artifact's `SHA256SUMS` |
| v2 executable | `opt/nevermind-next/nevermind-next`, SHA-256 90a0aef6d188f4e8bd3c0b2d485f3bc8f07756147e8f114b88fe84f9dd0983ef, identical in the deb and the tarball |
| Commit binding | The job checked out bd1be7fb6a3a351a3b66b83bf7b4191752130894, the PR merge of 71b918bf into 517dd878. Its tree, e8df23ab9a814a71eaf0edb322ffd5fea6eadd76, equals 71b918bf's tree, and 517dd878 is an ancestor of 71b918bf. The executable embeds bd1be7fb as `analyticsSourceCommit`. |

The macOS artifact of the same run is `native-macos-arm64`, ID 14556355283, digest 3d37605196cce541caf718be4f999eec029f3d1700751ccb5197320ddf149a72.

## Limits

- These are capture intervals on a software-rendered X server. They bound observed content, not application execution or display scanout, and they do not predict GPU-composited desktops.
- The host was shared. Block-start load average ranged 0.86–5.52 on the guest's view. The cgroup recorded no throttling, but that does not prove a quiet host. The observer used a median 22% of one CPU during capture windows (maximum 31%). Nothing was subtracted.
- Thirty attempts per endpoint come from ten launches per version, three per launch, so they are not 30 independent processes. Nothing was removed as an outlier; warmups stay in `trials.jsonl`.
- Default file-indexing limits differ between versions, so both indexing all 51,000 files is not implied. The timed search is Settings, not file search.
- Process-tree sampling every 100 ms can miss short-lived or reparented processes; it reports sampled values, not true peaks.
- The action menu is not a matched endpoint: v1's panel was clipped under Xvfb in earlier runs and is absent on macOS. Clean startup is not measured.
- Linux PSS and macOS physical footprint are different metrics on different hardware. Compare v1 with v2 within each platform only.

Raw frames, events, memory snapshots, both classification passes, calibration screenshots and harness scripts are kept, with a SHA-256 manifest, in the task's `.tmp/benchmarks/linux-v1-v2-20261119/` folder. They are not committed.

## Open Settings is not charted

`settings_open` stays in the data of both platforms but is left out of the charts for now. On the macOS VM, 2.x showed
the Settings view after about 340–470 ms on 13 of its 28 successful opens (median 396 ms), against 3–131 ms for 1.x;
the capture probe showed the compositor drawing normally while 2.x's window had not changed yet. On Linux 2.x opens it
in about 6 ms. This is a 2.x issue on macOS to fix; the row goes back into the charts once it is.
