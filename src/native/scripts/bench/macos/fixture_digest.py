import sys, pathlib, hashlib
root = pathlib.Path(sys.argv[1])
lines = sorted(f'{p.relative_to(root).as_posix()}\t{hashlib.sha256(p.read_bytes()).hexdigest()}' for p in root.rglob('*') if p.is_file())
print(len(lines), hashlib.sha256(('\n'.join(lines) + '\n').encode()).hexdigest())
