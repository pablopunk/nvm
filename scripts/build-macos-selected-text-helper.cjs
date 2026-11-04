const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

if (process.platform !== 'darwin') process.exit(0);

const root = path.resolve(__dirname, '..');
const sources = [
  'main.swift',
  'microphone.swift',
  'selected-text.swift',
  'window-manager.swift',
  'window-subcommands.swift',
  'window-accessibility.swift',
  'window-all.swift',
  'window-display.swift',
  'window-frame-writer.swift',
  'window-fullscreen.swift',
  'window-listing.swift',
  'window-raise.swift',
].map((name) => path.join(root, 'src/app/resources/macos-helper', name));
const outputDirectory = path.join(root, 'build/native');
const output = path.join(outputDirectory, 'macos-selected-text');
const swiftArchitectures = { arm64: 'arm64', x64: 'x86_64' };

// `--arch=<arm64|x64>` matches the packaged app's architecture so release
// builds skip compiling a slice they never ship; `--universal` keeps
// compile coverage for every supported architecture.
function requestedArchitectures(argv) {
  if (argv.includes('--universal')) return Object.values(swiftArchitectures);
  const requested = argv.find(function isArchFlag(argument) {
    return argument.startsWith('--arch=');
  });
  const arch = requested ? requested.slice('--arch='.length) : process.arch;
  const swiftArchitecture = swiftArchitectures[arch];
  if (!swiftArchitecture)
    throw new Error(`Unsupported macOS helper architecture: ${arch}`);
  return [swiftArchitecture];
}

const architectures = requestedArchitectures(process.argv);

fs.mkdirSync(outputDirectory, { recursive: true });
const slices = architectures.map((architecture) => {
  const slice = `${output}-${architecture}`;
  execFileSync(
    'xcrun',
    [
      'swiftc',
      ...sources,
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
