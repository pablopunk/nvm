# summarize.py <dataDir>: Markdown table of measured attempts (warmups dropped) and memory block medians.
import csv, json, statistics, sys
from collections import defaultdict
D = sys.argv[1]
def pct(xs, q):
    xs = sorted(xs); k = (len(xs) - 1) * q; f = int(k)
    return xs[f] if f + 1 >= len(xs) else xs[f] + (xs[f + 1] - xs[f]) * (k - f)
t = defaultdict(list)
for l in open(f"{D}/trials.jsonl"):
    r = json.loads(l)
    if not r["warmup"]: t[(r["endpoint"], r["version"])].append(r)
print("| Endpoint, ms | v1 ok | v1 median lower–upper | v1 p90 / p95 upper | v2 ok | v2 median lower–upper | v2 p90 / p95 upper |\n|---|---|---|---|---|---|---|")
for ep in ["open", "search", "clear", "row_down", "settings_open", "settings_back", "calculator"]:
    cells = []
    for v in ("v1", "v2"):
        rs = t.get((ep, v), []); ok = [r for r in rs if r["success"]]
        if not ok: cells += [f"0/{len(rs)}", "-", "-"]; continue
        lo, up = [r["lower_ms"] for r in ok], [r["upper_ms"] for r in ok]
        cells += [f"{len(ok)}/{len(rs)}", f"{statistics.median(lo):.0f}–{statistics.median(up):.0f}", f"{pct(up, .9):.0f} / {pct(up, .95):.0f}"]
    print(f"| {ep} | " + " | ".join(cells) + " |")
m = defaultdict(list)
for r in csv.DictReader(open(f"{D}/memory.csv")):
    if r["value_mib"]: m[(r["phase"], r["version"])].append(float(r["value_mib"]))
print("\n| phys_footprint, MiB | v1 median | v1 block range | v2 median | v2 block range |\n|---|---|---|---|---|")
for ph in ["hidden", "open", "slow_typing", "after_cycle"]:
    cells = []
    for v in ("v1", "v2"):
        xs = m.get((ph, v), [])
        cells += [f"{statistics.median(xs):.0f}", f"{min(xs):.0f}–{max(xs):.0f}"] if xs else ["-", "-"]
    print(f"| {ph} | " + " | ".join(cells) + " |")
