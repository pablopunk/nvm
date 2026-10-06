const path = require('node:path');
const asar = require('@electron/asar');

function readManifest(archive, directory) {
  try {
    return JSON.parse(
      asar
        .extractFile(archive, path.posix.join(directory, 'package.json'))
        .toString(),
    );
  } catch {
    return undefined;
  }
}

function requiredDependencies(manifest) {
  return Object.keys(manifest.dependencies ?? {}).filter(
    function isRequired(name) {
      return !Object.hasOwn(manifest.optionalDependencies ?? {}, name);
    },
  );
}

function resolvePackagedDependency(archive, parent, name) {
  let directory = parent;
  for (;;) {
    const candidate = path.posix.join(directory, 'node_modules', name);
    if (readManifest(archive, candidate)) return candidate;
    if (!directory || directory === '.') return;
    directory = path.posix.dirname(directory);
  }
}

function verifyPackagedDependencies(archive) {
  const root = readManifest(archive, '');
  if (!root)
    throw new Error('Packaged application manifest is missing or invalid');
  const pending = [{ directory: '', manifest: root }];
  const visited = new Set();
  const missing = new Set();
  while (pending.length) {
    const { directory, manifest } = pending.pop();
    if (visited.has(directory)) continue;
    visited.add(directory);
    for (const name of requiredDependencies(manifest)) {
      const resolved = resolvePackagedDependency(archive, directory, name);
      if (!resolved)
        missing.add(`${manifest.name ?? directory} requires ${name}`);
      else
        pending.push({
          directory: resolved,
          manifest: readManifest(archive, resolved),
        });
    }
  }
  if (missing.size)
    throw new Error(
      `Packaged runtime dependencies are missing:\n${[...missing].sort().join('\n')}`,
    );
  return { packagesChecked: visited.size - 1 };
}

module.exports = { verifyPackagedDependencies };

if (require.main === module) {
  console.log(
    JSON.stringify(verifyPackagedDependencies(path.resolve(process.argv[2]))),
  );
}
