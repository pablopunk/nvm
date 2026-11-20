# export_format.py <A> <outDir>: converts runs/<v>-bNN-run/{trials.jsonl,memory.csv} into the methodology's
# trials.jsonl and memory.csv (no OCR text leaves this script: only numbers and status codes).
import csv, json, os, re, statistics, sys
from collections import defaultdict
A, OUT = sys.argv[1], sys.argv[2]
ENDPOINT = {"S1-hotkey-open": "open", "S2-search": "search", "S3-query-clear": "clear", "S4-row-down": "row_down",
            "S6-subview-open": "settings_open", "S6-subview-back": "settings_back", "S7-calc-copy": "calculator"}
def capture_gap_fields(detail):
    """Extra fields (ignored by the charts): the largest capture-event gap after dispatch, the no-input control gap
    before dispatch, and how many probe-window frames the compositor presented inside the gap."""
    fields = {}
    for key in ("capture_gap_ms", "capture_gap_at_ms", "pre_dispatch_gap_ms", "probe_frames_in_gap"):
        m = re.search(key + r"=([0-9.]+)", detail)
        if m: fields[key] = float(m.group(1)) if "." in m.group(1) else int(m.group(1))
    return fields
runs = sorted(d for d in os.listdir(A + "/runs") if d.endswith("-run"))
trials, mem = [], []
for d in runs:
    v, b = d.split("-")[0], int(d.split("-")[1][1:]) - 1
    recs = [json.loads(l) for l in open(f"{A}/runs/{d}/trials.jsonl")]
    for r in recs:
        ep = ENDPOINT.get(r["scenario"])
        if not ep: continue
        ok = r["status"] == "ok"
        up = r.get("latencyMs") if ok else None
        lo = (up if ep == "calculator" else r.get("lowerMs")) if ok else None
        trials.append({"platform": "macos", "version": v, "block": b, "trial": r["index"], "warmup": r["index"] == 0,
                       "endpoint": ep, "success": ok, "lower_ms": None if lo is None else round(lo, 2),
                       "upper_ms": None if up is None else round(up, 2), "reason": None if ok else r["status"],
                       **capture_gap_fields(r.get("detail", ""))})
    checks = {r["scenario"]: r for r in recs}
    slow = {r["index"]: r for r in recs if r["scenario"] == "slow-typing-memory"}
    snaps = defaultdict(lambda: {"fp": 0, "rss": 0, "ok": True})
    for row in csv.DictReader(open(f"{A}/runs/{d}/memory.csv")):
        s = snaps[row["t_host"]]
        s.update(t=float(row["t_host"]), te=float(row["t_read_end"]), phase=row["phase"])
        s["ok"] &= row["read_ok"] == "true" and row["phase"] == row["phase_after"]
        s["fp"] += int(row["phys_footprint_bytes"]); s["rss"] += int(row["rss_bytes"])
    def accept(s):
        p = s["phase"]
        if p == "hidden": return "hidden" if checks.get("hidden-check", {}).get("status") == "ok" else None
        if p.startswith("open-"): return "open"
        if p == "after-cycle": return "after_cycle" if checks.get("after-cycle-check", {}).get("status") == "ok" else None
        if p.startswith("slow-typing-") and p[12:].isdigit():
            r = slow.get(int(p[12:]))
            if r and not r["warmup"] and r["status"] == "ok" and s["t"] >= r["tStart"] and s["te"] <= r["tEnd"]: return "slow_typing"
        return None
    groups = defaultdict(list)
    for s in snaps.values():
        ph = accept(s) if s["ok"] else None
        if ph: groups[ph].append(s)
    for ph in ["hidden", "open", "slow_typing", "after_cycle"]:
        g = groups.get(ph, [])
        mem.append({"platform": "macos", "version": v, "block": b, "phase": ph, "metric": "phys_footprint",
                    "value_mib": round(statistics.median(x["fp"] for x in g) / 1048576, 2) if g else "",
                    "samples": len(g), "rss_mib": round(statistics.median(x["rss"] for x in g) / 1048576, 2) if g else ""})
os.makedirs(OUT, exist_ok=True)
trials.sort(key=lambda t: (t["version"], t["block"], list(ENDPOINT.values()).index(t["endpoint"]), t["trial"]))
with open(OUT + "/trials.jsonl", "w") as f:
    for t in trials: f.write(json.dumps(t) + "\n")
mem.sort(key=lambda m: (m["version"], m["block"], m["phase"]))
with open(OUT + "/memory.csv", "w", newline="") as f:
    w = csv.DictWriter(f, fieldnames=list(mem[0].keys())); w.writeheader(); w.writerows(mem)
print(len(trials), "trials,", len(mem), "memory rows from", len(runs), "runs")
