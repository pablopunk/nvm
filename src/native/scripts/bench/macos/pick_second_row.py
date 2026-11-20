# pick_second_row.py <explore session.log>: prints the title of the row below "Settings" for the "settings" query,
# read from the explore pass's OCR dump ("minY<TAB>text" lines after the STEP settings-5s marker).
import re, sys
text = open(sys.argv[1]).read()
block = text.split(" settings-5s ", 1)[1].split("\n", 1)[1]
block = re.split(r"\n\d+\.\d+ STEP ", block, maxsplit=1)[0]
rows = []
for line in block.splitlines():
    y, _, t = line.partition("\t")
    if y.strip().lstrip("-").isdigit(): rows.append((int(y), t.strip()))
rows.sort()
top = next((y for y, t in rows if t == "Settings"), None)
if top is None: sys.exit("no 'Settings' row in the explore OCR")
cands = [t for y, t in rows if y > top + 4 and t.endswith("Settings") and t != "Settings"]
if not cands: sys.exit("no second Settings row below 'Settings'")
print(cands[0])
