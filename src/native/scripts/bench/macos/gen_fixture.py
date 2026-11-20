import sys, pathlib
home = pathlib.Path(sys.argv[1])
for i in range(51000):
    d = home / 'Documents/bench' / f'{i//1000:03d}'
    d.mkdir(parents=True, exist_ok=True)
    (d / f'fixture-{i:05d}.txt').write_text(f'Nevermind benchmark fixture {i:05d}\n')
