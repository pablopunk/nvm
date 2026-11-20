# finalize.py <B> <out> <blocks> <v1Label> <v2Sha>: writes the methodology's trials.jsonl, memory.csv and run.json,
# a frame sample (endpoint PNGs of the first block, palette crops of the runner image's own apps only) and a SHA-256 manifest.
import datetime, hashlib, json, os, shutil, statistics, subprocess, sys
B, OUT, BLOCKS, V1, V2SHA = sys.argv[1], sys.argv[2], int(sys.argv[3]), sys.argv[4], sys.argv[5]
H = os.path.dirname(os.path.abspath(__file__))
subprocess.run([sys.executable, "-I", f"{H}/export_format.py", B, OUT], check=True)
def sh(*a): return subprocess.run(a, capture_output=True, text=True).stdout.strip()
ncpu, mem = int(sh("sysctl", "-n", "hw.ncpu")), int(sh("sysctl", "-n", "hw.memsize")) / 2**30
model, osv = sh("sysctl", "-n", "machdep.cpu.brand_string"), sh("sw_vers", "-productVersion")
rows = {v: open(f"{B}/explore/second-row-{v}.txt").read().strip() for v in ("v1", "v2")}
floor = sh("python3", "-I", "-c", "import csv,statistics,sys;r=[float(x['latency_ms']) for x in csv.DictReader(open(sys.argv[1])) if x['status']=='ok'];print(f'{statistics.median(r):.1f}' if r else '-')", f"{B}/runs/observer-floor.csv")
def capture_gap_note(out):
    """Capture gaps of 300 ms or more after dispatch, the no-input control before dispatch, and the probe window's frames
    inside the gaps (the probe is redrawn at 120 Hz by the harness and captured by its own stream)."""
    rows = [json.loads(l) for l in open(f"{out}/trials.jsonl")]
    measured = [r for r in rows if not r["warmup"] and r["endpoint"] != "calculator" and "capture_gap_ms" in r]
    parts = []
    for v in ("v1", "v2"):
        mine = [r for r in measured if r["version"] == v]
        gaps = [r for r in mine if r["capture_gap_ms"] >= 300]
        pre = [r for r in mine if r.get("pre_dispatch_gap_ms", 0) >= 300]
        parts.append(f"{v} {len(gaps)}/{len(mine)} after dispatch, {len(pre)}/{len(mine)} in the 1 s no-input window before it")
    return "Capture gaps of 300 ms or more (no frame and no idle sample from the app's stream): " + "; ".join(parts) + ". " + probe_note(B)


def probe_note(b):
    """Diagnostic Down/Enter attempts after the measured trials: when the app's change appeared 400 ms or more after the
    key, how quickly the probe window's flips scheduled before that change reached the screen."""
    import glob, re
    parts = []
    for v in ("v1", "v2"):
        slow, delays, missing = 0, [], 0
        for p in glob.glob(f"{b}/runs/{v}-b*-run/trials.jsonl"):
            for line in open(p):
                r = json.loads(line)
                if not r["scenario"].endswith("-probe") or r.get("latencyMs") is None or r["latencyMs"] < 400: continue
                m = re.search(r"probe_flips=\[([^\]]*)\]", r.get("detail", ""))
                if not m: continue
                slow += 1
                for item in filter(None, m.group(1).split(",")):
                    offset, delay = item.split(":")
                    if float(offset) >= r["latencyMs"] - 20: continue
                    if delay == "none": missing += 1
                    else: delays.append(float(delay))
        if slow:
            parts.append(f"{v}: {slow} diagnostic attempts changed 400 ms or more after the key; probe flips made before that change "
                         f"reached the screen after a median of {statistics.median(delays):.0f} ms" + (f" ({missing} never shown)" if missing else "") if delays else
                         f"{v}: {slow} slow diagnostic attempts, no probe flip before the change")
        else:
            parts.append(f"{v}: no diagnostic attempt changed 400 ms or more after the key")
    return "Probe diagnostics (6 extra attempts per block, a harness window flipped 5 times at +150 to +550 ms): " + "; ".join(parts) + "."
run = {
    "platform": "macos", "title": "macOS (GitHub-hosted VM)",
    "machine": f"GitHub-hosted macos-15 runner: {model} virtual machine, {ncpu} vCPU, {mem:.0f} GB, no GPU acceleration, macOS {osv}.",
    "memory_metric": "physical footprint of the whole process tree",
    "collected": datetime.date.today().strftime("%-d %b %Y"),
    "v1_label": f"v1 · {V1.lstrip('v')} (Electron)", "v2_label": f"v2 · {V2SHA[:8]}",
    "attempts_note": f"{3 * BLOCKS} across {BLOCKS} alternating process block{'s' if BLOCKS > 1 else ''}",
    "notes": [
        "A virtual machine, not a real Mac: absolute numbers differ from hardware; compare v1 with v2 within this run only. "
        f"Alternating AB/BA blocks, disposable profiles with identical 51,000-file fixtures, one 60-second priming launch per profile, profiles excluded from Spotlight; observer key-to-frame floor median {floor} ms.",
        f"Bounds are ScreenCaptureKit frame and idle-sample times of the palette region, from CGEvent dispatch start; frames are classified by Vision OCR after each capture window. "
        f"Second Settings row from the explore pass: '{rows['v1']}' (v1), '{rows['v2']}' (v2). A palette shown at launch is dismissed with Escape before the hidden phase. The action menu is not measured (v1 has none).",
        "The stable-list signature is the first nine row titles below the input, as on Linux; subtitles, right-hand values and footer chips are not compared.",
        capture_gap_note(OUT),
    ],
}
json.dump(run, open(f"{OUT}/run.json", "w"), indent=2, ensure_ascii=False); open(f"{OUT}/run.json", "a").write("\n")
sample = f"{B}/upload/frame-sample"; os.makedirs(sample, exist_ok=True)
for d in sorted(os.listdir(f"{B}/runs")):
    if d.endswith("-b01-run") or d.startswith("explore-"):
        for f in sorted(os.listdir(f"{B}/runs/{d}")):
            if f.endswith(".png") and ("endpoint" in f or f.startswith("explore-")): shutil.copy(f"{B}/runs/{d}/{f}", f"{sample}/{d}-{f}")
for f in ("trials.jsonl", "memory.csv", "run.json"): shutil.copy(f"{OUT}/{f}", f"{B}/upload/{f}")
for f in os.listdir(f"{B}/provenance"): shutil.copy(f"{B}/provenance/{f}", f"{B}/upload/provenance-{f}")
lines = []
for root, _, files in os.walk(f"{B}/upload"):
    for f in sorted(files):
        p = os.path.join(root, f)
        if f != "SHA256SUMS": lines.append(f"{hashlib.sha256(open(p,'rb').read()).hexdigest()}  {os.path.relpath(p, B + '/upload')}")
open(f"{B}/upload/SHA256SUMS", "w").write("\n".join(sorted(lines, key=lambda l: l[66:])) + "\n")
print(json.dumps(run, indent=2, ensure_ascii=False))
