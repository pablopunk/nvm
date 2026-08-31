const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

if (process.platform !== 'darwin') process.exit(0);

const root = path.resolve(__dirname, '..');
const source = path.join(root, 'src/app/resources/macos-selected-text.swift');
const outputDirectory = path.join(root, 'build/native');
const output = path.join(outputDirectory, 'macos-selected-text');
const architectures = process.argv.includes('--universal')
  ? ['arm64', 'x86_64']
  : [process.arch === 'x64' ? 'x86_64' : 'arm64'];

fs.mkdirSync(outputDirectory, { recursive: true });
const slices = architectures.map((architecture) => {
  const slice = `${output}-${architecture}`;
  execFileSync(
    'xcrun',
    [
      'swiftc',
      source,
      '-O',
      '-target',
      `${architecture}-apple-macos12.0`,
      '-o',
      slice,
    ],
    { stdio: 'inherit' },
  );
  return slice;
});

if (slices.length === 1) fs.renameSync(slices[0], output);
else execFileSync('lipo', ['-create', ...slices, '-output', output]);
for (const slice of slices) fs.rmSync(slice, { force: true });
fs.chmodSync(output, 0o755);
