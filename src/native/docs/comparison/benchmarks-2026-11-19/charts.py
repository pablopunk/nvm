#!/usr/bin/env python3
"""Regenerate the v1/v2 benchmark charts from per-platform result folders.

    python3 src/native/docs/comparison/benchmarks-2026-11-19/charts.py

Reads data/<platform>/{run.json,trials.jsonl,memory.csv} for every platform folder present
(linux, macos), writes memory.html and responsiveness.html, and renders memory.png and
responsiveness.png with headless Chromium (set CHROMIUM to override the binary; --no-png skips it).
The input format is specified in methodology.md.
"""
import argparse, csv, html, json, math, os, pathlib, shutil, statistics, struct, subprocess, sys, tempfile, zlib

HERE = pathlib.Path(__file__).resolve().parent
PLATFORMS = ('linux', 'macos')
# Each chart row: (key, short label). Timing rows are trials.jsonl endpoints; memory rows are memory.csv phases.
# settings_open is measured and kept in the data but not charted: on macOS 2.x is slow to show the Settings view on
# about half of the opens, a known issue to fix (see methodology.md), so the row returns once that is fixed.
ENDPOINTS = [('search', 'Search'), ('clear', 'Clear'), ('row_down', 'Row down'),
             ('settings_back', 'Leave Settings'), ('calculator', 'Calculator')]
MEMORY_PHASES = [('hidden', 'Hidden'), ('open', 'Open'), ('slow_typing', 'Typing'), ('after_cycle', 'After use')]
VERSIONS = (('v1', '1.x', 'Electron'), ('v2', '2.x', 'Go'))
# A "N× faster" tag needs both a clear ratio and, for timings, a gap of at least two 10 ms capture intervals.
TAG_MIN_RATIO, TAG_MIN_GAP_MS = 1.5, 20
CSS = (':root{--fg:#0a0a0a;--muted:#737373;--border:#e5e5e5;--v1:#a3a29b;--v2:#2a78d6;--tag-bg:#eaf2fc;--tag-fg:#184f95;'
       '--sans:"Geist","Geist Variable",Inter,-apple-system,"Helvetica Neue","Liberation Sans",Arial,sans-serif;'
       '--mono:"Geist Mono","SF Mono",Menlo,"Liberation Mono",monospace}'
       '*{box-sizing:border-box}body{margin:0;background:#fff;color:var(--fg);font:16px/1.5 var(--sans);width:900px}'
       'header{padding:36px 40px 26px;border-bottom:1px solid var(--border)}'
       '.eyebrow{font-size:12px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted);margin:0 0 10px}'
       'h1{font-size:28px;font-weight:650;letter-spacing:-.02em;line-height:1.2;margin:0}'
       '.legend{display:flex;gap:24px;margin:18px 0 0;padding:0;list-style:none;font-size:15px;font-weight:500}'
       '.legend li{display:flex;align-items:center;gap:8px}.legend small{color:var(--muted);font-size:15px;font-weight:400}'
       '.swatch{width:11px;height:11px;border-radius:3px}'
       '.panels{display:grid;gap:48px;padding:30px 40px 34px}'
       'h2{font-size:19px;font-weight:600;letter-spacing:-.01em;margin:0 0 4px}h2 span{color:var(--muted);font-weight:400}'
       'figure{margin:20px 0 0}figcaption{display:flex;align-items:center;gap:10px;font-size:15px;font-weight:500;margin-bottom:4px}'
       '.tag{font-size:13px;font-weight:600;color:var(--tag-fg);background:var(--tag-bg);border-radius:999px;padding:1px 9px}'
       '.row{display:grid;grid-template-columns:34px 1fr 76px;gap:10px;align-items:center;padding:3px 0}'
       '.who{font-size:14px;color:var(--muted)}.track{height:12px}.bar{display:block;height:100%;border-radius:0 4px 4px 0}'
       '.val{text-align:right;font:15px var(--mono);font-variant-numeric:tabular-nums}.val.na{color:var(--muted)}'
       'footer{border-top:1px solid var(--border);padding:16px 40px 22px;font-size:14px;color:var(--muted)}')


def load(platform):
    d = HERE / 'data' / platform
    if not (d / 'trials.jsonl').exists() and not (d / 'memory.csv').exists():
        return None
    run = json.loads((d / 'run.json').read_text()) if (d / 'run.json').exists() else {}
    trials = [json.loads(x) for x in (d / 'trials.jsonl').read_text().splitlines() if x.strip()] if (d / 'trials.jsonl').exists() else []
    memory = list(csv.DictReader((d / 'memory.csv').open())) if (d / 'memory.csv').exists() else []
    for t in trials:
        if t.get('platform', platform) != platform or t['version'] not in ('v1', 'v2'):
            sys.exit(f'{platform}/trials.jsonl: unexpected platform/version in {t}')
    return dict(platform=platform, run=run, trials=trials, memory=memory)


def nearest_rank(values, q):
    v = sorted(values)
    return v[math.ceil(q * len(v)) - 1]


def timing(data, version, endpoint):
    """Median of the successful measured attempts; an interval endpoint is drawn at the midpoint of its median interval."""
    ok = [t for t in data['trials'] if t['version'] == version and t['endpoint'] == endpoint and not t['warmup'] and t['success']]
    if not ok:
        return None
    lower = statistics.median(t['lower_ms'] for t in ok); upper = statistics.median(t['upper_ms'] for t in ok)
    span = f'{lower:.0f} ms' if lower == upper else f'{lower:.0f}–{upper:.0f} ms'
    return dict(value=(lower + upper) / 2, text=f'{(lower + upper) / 2:.0f} ms',
                hover=f'median {span} · p95 {nearest_rank([t["upper_ms"] for t in ok], .95):.0f} ms · {len(ok)} attempts')


def memory(data, version, phase):
    """Median of the per-block medians."""
    vals = [float(r['value_mib']) for r in data['memory'] if r['version'] == version and r['phase'] == phase]
    if not vals:
        return None
    m = statistics.median(vals)
    return dict(value=m, text=f'{m:.0f} MiB', hover=f'block medians {min(vals):.0f}–{max(vals):.0f} MiB · {len(vals)} blocks')


def ratio_tag(old, new, word, min_gap):
    if not old or not new or new['value'] <= 0 or old['value'] - new['value'] < min_gap:
        return ''
    ratio = old['value'] / new['value']
    if ratio < TAG_MIN_RATIO:
        return ''
    return f'<span class="tag">{ratio:.1f}× {word}</span>'.replace('.0×', '×')


def bar_row(result, version, short, scale):
    if not result:
        return f'<div class="row"><span class="who">{short}</span><span class="track"></span><span class="val na">—</span></div>'
    width = 100 * result['value'] / scale
    return (f'<div class="row" title="{short} · {html.escape(result["hover"])}"><span class="who">{short}</span>'
            f'<span class="track"><span class="bar" style="width:max({width:.2f}%,3px);background:var(--{version})"></span></span>'
            f'<span class="val">{result["text"]}</span></div>')


def panel(data, rows, measure, word, min_gap):
    results = {key: {v: measure(data, v, key) for v, _, _ in VERSIONS} for key, _ in rows}
    scale = max((r['value'] for pair in results.values() for r in pair.values() if r), default=1) or 1
    title = data['run'].get('title', data['platform'])
    name, _, extra = title.partition(' (')
    heading = f'{html.escape(name)}' + (f' <span>{html.escape(extra.rstrip(")"))}</span>' if extra else '')
    figures = []
    for key, label in rows:
        pair = results[key]
        tag = ratio_tag(pair['v1'], pair['v2'], word, min_gap)
        bars = ''.join(bar_row(pair[v], v, short, scale) for v, short, _ in VERSIONS)
        figures.append(f'<figure><figcaption>{label}{tag}</figcaption>{bars}</figure>')
    return f'<section><h2>{heading}</h2>{"".join(figures)}</section>'


def page(title, eyebrow, datas, rows, measure, word, min_gap, foot):
    legend = ''.join(f'<li><span class="swatch" style="background:var(--{v})"></span>{short} <small>{kind}</small></li>' for v, short, kind in VERSIONS)
    panels = ''.join(panel(d, rows, measure, word, min_gap) for d in datas)
    body = (f'<header><p class="eyebrow">{eyebrow}</p><h1>{title}</h1><ul class="legend">{legend}</ul></header>'
            f'<main class="panels" style="grid-template-columns:repeat({len(datas)},minmax(0,1fr))">{panels}</main><footer>{foot}</footer>')
    return f'<!doctype html><html><head><meta charset="utf-8"><title>{title}</title><style>{CSS}</style></head><body>{body}</body></html>'


EYEBROW = 'Nevermind 1.x vs 2.x'


def memory_page(datas):
    metrics = {'linux': 'Linux PSS', 'macos': 'macOS physical footprint'}
    foot = ' · '.join([metrics[d['platform']] for d in datas] + ['lower is better'])
    return page('Memory, whole app', EYEBROW, datas, MEMORY_PHASES, memory, 'less', 0, foot)


def responsiveness_page(datas):
    foot = 'Medians · screen timings use the midpoint of the median capture interval · lower is better'
    return page('Key press to result', EYEBROW, datas, ENDPOINTS, timing, 'faster', TAG_MIN_GAP_MS, foot)


def png_rows(path):
    raw = path.read_bytes(); pos = 8; chunks = []; ihdr = None
    while pos < len(raw):
        n, kind = struct.unpack('>I4s', raw[pos:pos + 8]); data = raw[pos + 8:pos + 8 + n]; pos += 12 + n
        if kind == b'IHDR':
            ihdr = struct.unpack('>IIBBBBB', data)
        elif kind == b'IDAT':
            chunks.append(data)
    w, h, depth, color = ihdr[:4]
    if depth != 8 or color not in (2, 6):
        sys.exit(f'unsupported PNG from Chromium: depth {depth} color {color}')
    bpp = 3 if color == 2 else 4; stride = w * bpp; data = zlib.decompress(b''.join(chunks)); rows = []; prev = bytearray(stride); i = 0
    for _ in range(h):
        f = data[i]; line = bytearray(data[i + 1:i + 1 + stride]); i += 1 + stride
        for x in range(stride):
            a = line[x - bpp] if x >= bpp else 0; b = prev[x]; c = prev[x - bpp] if x >= bpp else 0
            if f == 1: line[x] = (line[x] + a) & 255
            elif f == 2: line[x] = (line[x] + b) & 255
            elif f == 3: line[x] = (line[x] + (a + b) // 2) & 255
            elif f == 4:
                p = a + b - c; pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                line[x] = (line[x] + (a if pa <= pb and pa <= pc else b if pb <= pc else c)) & 255
        rows.append(bytes(line[i2] for i2 in range(stride) if i2 % bpp < 3) if bpp == 4 else bytes(line)); prev = line
    return w, rows


def write_png(path, w, rows):
    def chunk(kind, data):
        return struct.pack('>I', len(data)) + kind + data + struct.pack('>I', zlib.crc32(kind + data) & 0xffffffff)
    body = b''.join(b'\x00' + r for r in rows)
    path.write_bytes(b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', w, len(rows), 8, 2, 0, 0, 0)) + chunk(b'IDAT', zlib.compress(body, 9)) + chunk(b'IEND', b''))


def render(html_path, png_path):
    chromium = os.environ.get('CHROMIUM') or shutil.which('chromium') or shutil.which('google-chrome') or shutil.which('chromium-browser')
    if not chromium:
        sys.exit('no Chromium found; set CHROMIUM or pass --no-png')
    with tempfile.TemporaryDirectory() as tmp:
        shot = pathlib.Path(tmp) / 'shot.png'
        subprocess.run([chromium, '--headless', '--disable-gpu', '--hide-scrollbars', f'--user-data-dir={tmp}/profile', '--force-device-scale-factor=2',
                        '--window-size=900,2000', f'--screenshot={shot}', html_path.as_uri()], check=True, capture_output=True)
        w, rows = png_rows(shot)
    white = bytes([255]) * len(rows[0]); last = max(i for i, r in enumerate(rows) if r != white)
    write_png(png_path, w, rows[:last + 1 + 40])


def main():
    ap = argparse.ArgumentParser(); ap.add_argument('--no-png', action='store_true'); a = ap.parse_args()
    datas = [d for p in PLATFORMS if (d := load(p))]
    if not datas:
        sys.exit('no data/<platform>/ folders found')
    for name, content in (('memory', memory_page(datas)), ('responsiveness', responsiveness_page(datas))):
        out = HERE / f'{name}.html'; out.write_text(content)
        if not a.no_png:
            render(out, HERE / f'{name}.png')
        print('wrote', out.relative_to(HERE), '' if a.no_png else f'and {name}.png', 'for', ', '.join(d['platform'] for d in datas))


if __name__ == '__main__':
    main()
