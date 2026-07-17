const fs = require('node:fs/promises');
const path = require('node:path');

const [appImagePath] = process.argv.slice(2);
if (!appImagePath)
  throw new Error('Usage: generate-linux-appimage-blockmap <AppImage path>');
if (!appImagePath.endsWith('.AppImage'))
  throw new Error(`Expected an AppImage path, received ${appImagePath}`);

function electronBuilderDependency(specifier) {
  const electronBuilderPath = path.dirname(
    require.resolve('electron-builder/package.json'),
  );
  return require.resolve(specifier, { paths: [electronBuilderPath] });
}

async function main() {
  const input = path.resolve(appImagePath);
  const output = `${input}.blockmap`;
  const inputStat = await fs.stat(input);
  if (!inputStat.isFile() || inputStat.size === 0)
    throw new Error(`Expected a non-empty AppImage at ${input}`);

  const { executeAppBuilderAsJson } = require(
    electronBuilderDependency('app-builder-lib/out/util/appBuilder'),
  );
  await fs.rm(output, { force: true });
  await executeAppBuilderAsJson([
    'blockmap',
    '--input',
    input,
    '--output',
    output,
  ]);

  const outputStat = await fs.stat(output);
  if (!outputStat.isFile() || outputStat.size === 0)
    throw new Error(`Failed to create a non-empty blockmap at ${output}`);
  console.log(`Generated ${path.basename(output)}`);
}

void main();
