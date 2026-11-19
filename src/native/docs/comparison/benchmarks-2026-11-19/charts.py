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
MEMORY_PHASES = [('hidden', 'Hidden, settled', None), ('open', 'Palette open, settled', None),
                 ('slow_typing', 'Slow typing', '100 ms per character'), ('after_cycle', 'Hidden after a full cycle', None)]
ENDPOINTS = [('search', 'Type “settings” → stable result list'), ('clear', 'Clear query → stable root list'),
             ('row_down', 'Down arrow → next row selected'), ('settings_open', 'Enter on Settings → Settings view'),
             ('settings_back', 'Escape → back out of Settings'), ('calculator', 'Calculator Enter → result on the clipboard')]
CSS = ('body{margin:0;background:#fff;color:#162d36;font:16px Arial,sans-serif}*{box-sizing:border-box}main{padding:30px}'
       'h1{font-size:30px;margin:0 0 8px;letter-spacing:-.6px}p{color:#52666e;line-height:1.5;margin:10px 0}.sub{font-size:15px}'
       '.legend{display:flex;gap:25px;margin:20px 0;font-size:14px}.dot{display:inline-block;width:12px;height:12px;border-radius:3px;margin-right:7px}'
       '.old{background:#77899e}.new{background:#008675}.cols{display:flex;gap:28px}.col{flex:1;min-width:0}'
       '.col h2{font-size:19px;margin:6px 0 4px}.col .metric{font-size:13px;color:#52666e;margin-bottom:6px}'
       '.row{display:grid;grid-template-columns:190px 1fr;gap:15px;margin:20px 0;align-items:center}.label{font-weight:bold;font-size:16px}'
       '.bar{height:31px;border-radius:5px;line-height:31px;color:white;padding-left:12px;min-width:115px;font-size:14px;white-space:nowrap}'
       '.bar.new{margin-top:6px}.foot{font-size:13px;line-height:1.6;border-top:1px solid #dce5e8;padding-top:15px}'
       '.range{color:#52666e;font-size:12px;font-weight:normal;display:block;margin-top:5px}.source{font-size:12px;color:#52666e}'
       '.timing{padding:16px 0;border-top:1px solid #dce5e8}.timing h2{font-size:19px;margin:0 0 12px}.pair{display:flex;gap:18px}'
       '.plat{flex:1;min-width:0}.plat h3{font-size:13px;color:#52666e;margin:0 0 8px;text-transform:uppercase;letter-spacing:.6px}'
       '.boxes{display:flex;gap:12px}.box{flex:1;border-radius:10px;background:#f1f5f6;padding:15px}.num{font-size:27px;font-weight:bold;margin:6px 0}'
       '.bounds{font-size:13px;color:#52666e}.oldtext{color:#61758c}.newtext{color:#008675}.na{font-size:15px;color:#52666e;margin:12px 0}')


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
    rel = [t for t in data['trials'] if t['version'] == version and t['endpoint'] == endpoint and not t['warmup']]
    ok = [t for t in rel if t['success']]
    if not ok:
        return None
    lo = [t['lower_ms'] for t in ok]; up = [t['upper_ms'] for t in ok]
    return dict(attempts=len(rel), success=len(ok), lower=statistics.median(lo), upper=statistics.median(up),
                p95_upper=nearest_rank(up, .95), point=all(t['lower_ms'] == t['upper_ms'] for t in ok))


def memory(data, version, phase):
    vals = [float(r['value_mib']) for r in data['memory'] if r['version'] == version and r['phase'] == phase]
    if not vals:
        return None
    return dict(median=statistics.median(vals), lo=min(vals), hi=max(vals), blocks=len(vals))


def legend(datas):
    v2 = sorted({d['run'].get('v2_label', 'v2') for d in datas})
    v1 = sorted({d['run'].get('v1_label', 'v1') for d in datas})
    return (f'<div class="legend"><span><i class="dot old"></i>{html.escape(" / ".join(v1))}</span>'
            f'<span><i class="dot new"></i>{html.escape(" / ".join(v2))}</span></div>')


def notes(datas):
    parts = []
    for d in datas:
        r = d['run']
        text = ' '.join(r.get('notes', []))
        parts.append(f'<b>{html.escape(r.get("title", d["platform"]))}:</b> {html.escape(r.get("machine", ""))} {html.escape(text)}')
    return '<br>'.join(parts)


def source(datas):
    dates = ', '.join(sorted({d['run'].get('collected', '') for d in datas if d['run'].get('collected')}))
    return f'<div class="source">Source: pablopunk/nvm PR #280 · measurements collected {html.escape(dates)}</div>'


def memory_page(datas):
    cols = []
    for d in datas:
        scale = max((m['median'] for v in ('v1', 'v2') for p, _, _ in MEMORY_PHASES if (m := memory(d, v, p))), default=1) / .9
        rows = []
        for phase, label, note in MEMORY_PHASES:
            a, b = memory(d, 'v1', phase), memory(d, 'v2', phase)
            sub = f'<span class="range">{note}</span>' if note else ''
            if not a or not b:
                rows.append(f'<div class="row"><div class="label">{label}{sub}</div><div class="na">not measured</div></div>'); continue
            rows.append(f'<div class="row"><div class="label">{label}{sub}</div><div>'
                        f'<div class="bar old" style="width:{100 * a["median"] / scale:.1f}%">{a["median"]:.0f} MiB</div>'
                        f'<div class="bar new" style="width:{100 * b["median"] / scale:.1f}%">{b["median"]:.0f} MiB</div></div></div>')
        r = d['run']
        cols.append(f'<div class="col"><h2>{html.escape(r.get("title", d["platform"]))}</h2>'
                    f'<div class="metric">{html.escape(r.get("memory_metric", ""))}</div>{"".join(rows)}</div>')
    ranges = []
    for d in datas:
        bits = []
        for phase, label, _ in MEMORY_PHASES:
            a, b = memory(d, 'v1', phase), memory(d, 'v2', phase)
            if a and b:
                bits.append(f'{label.lower()} v1 {a["lo"]:.0f}–{a["hi"]:.0f}, v2 {b["lo"]:.0f}–{b["hi"]:.0f}')
        ranges.append(f'{d["run"].get("title", d["platform"])} block-median ranges (MiB): ' + '; '.join(bits) + '.')
    body = (f'<h1>Process-tree memory</h1><p class="sub">MiB · median of per-block medians · lower is better · compare versions within a platform; the metrics differ</p>'
            f'{legend(datas)}<div class="cols">{"".join(cols)}</div><p class="sub">{html.escape(" ".join(ranges))}</p>'
            f'<p class="foot">{notes(datas)}</p>{source(datas)}')
    return page('Process-tree memory', body)


def timing_box(t, cls, version):
    if not t:
        return f'<div class="box"><b>{version}</b><div class="na">not measured</div></div>'
    if t['point']:
        num = f'{t["upper"]:.0f} ms'; sub = f'median · p95 {t["p95_upper"]:.0f} ms'
    else:
        num = f'{t["lower"]:.0f}–{t["upper"]:.0f} ms'; sub = f'median interval · p95 ≤{t["p95_upper"]:.0f} ms'
    return f'<div class="box"><b>{version}</b><div class="num {cls}">{num}</div><div class="bounds">{sub}<br>{t["success"]}/{t["attempts"]} attempts succeeded</div></div>'


def responsiveness_page(datas):
    sections = []
    for endpoint, label in ENDPOINTS:
        plats = []
        for d in datas:
            a, b = timing(d, 'v1', endpoint), timing(d, 'v2', endpoint)
            head = f'<h3>{html.escape(d["run"].get("title", d["platform"]))}</h3>' if len(datas) > 1 else ''
            boxes = f'<div class="boxes">{timing_box(a, "oldtext", "v1")}{timing_box(b, "newtext", "v2")}</div>'
            plats.append(f'<div class="plat">{head}{boxes}</div>')
        sections.append(f'<section class="timing"><h2>{label}</h2><div class="pair">{"".join(plats)}</div></section>')
    counts = ', '.join(f'{d["run"].get("title", d["platform"])} {d["run"].get("attempts_note", "")}'.strip() for d in datas)
    body = (f'<h1>Interaction responsiveness</h1><p class="sub">Measured attempts per version: {html.escape(counts)}</p>{legend(datas)}{"".join(sections)}'
            '<p class="foot">Visual values are capture intervals: the lower bound is the start of the last capture before the final stable state, the upper bound '
            'the end of its first capture, both measured from key dispatch start. The end of an action is the first frame after the last list change, held at '
            'least 1 s. They bound observed content, not exact application execution or display scanout. Calculator: expression already entered, unique '
            'clipboard sentinel verified before Enter, result varies per attempt; includes key injection and clipboard polling. p95 is nearest rank. '
            f'No observer overhead was subtracted.<br>{notes(datas)}</p>{source(datas)}')
    return page('Interaction responsiveness', body)


def page(title, body):
    return f'<!doctype html><html><head><meta charset="utf-8"><title>{title}</title><style>{CSS}</style></head><body><main>{body}</main></body></html>'


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
                        '--window-size=1100,3000', f'--screenshot={shot}', html_path.as_uri()], check=True, capture_output=True)
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
