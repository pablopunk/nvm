#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

const configPath = path.join(process.cwd(), 'electron-builder.yml');
const config = fs.readFileSync(configPath, 'utf8');
// The packaged ASAR is `packageRoot`; build tooling and installed checker
// packages are only present in the repository checkout.
const repoRoot = process.cwd();
const packageRoot = path.resolve(process.argv[2] || repoRoot);
const packageJson = JSON.parse(
  fs.readFileSync(path.join(packageRoot, 'package.json'), 'utf8'),
);

function fail(message) {
  console.error(`Packaged resource check failed: ${message}`);
  process.exit(1);
}

if (
  !fs.existsSync(
    path.join(
      packageRoot,
      'src',
      'app',
      'resources',
      'nevermind-extension-api.d.ts',
    ),
  )
) {
  fail('missing src/app/resources/nevermind-extension-api.d.ts');
}

if (
  !fs.existsSync(path.join(repoRoot, 'scripts', 'stage-typescript-native.cjs'))
) {
  fail('missing scripts/stage-typescript-native.cjs');
}

const { createRequire } = require('node:module');
const typescriptRoot = path.dirname(
  require.resolve('typescript/package.json', { paths: [repoRoot] }),
);
const checkerPackage = `@typescript/typescript-${process.platform}-${process.arch}`;
let checkerLib;
try {
  const requireFromTypeScript = createRequire(
    path.join(typescriptRoot, 'package.json'),
  );
  checkerLib = path.join(
    path.dirname(
      requireFromTypeScript.resolve(`${checkerPackage}/package.json`),
    ),
    'lib',
  );
} catch {
  fail(`missing ${checkerPackage}; run mise exec -- pnpm install`);
}

for (const checkerFile of [
  process.platform === 'win32' ? 'tsc.exe' : 'tsc',
  'lib.es2022.full.d.ts',
]) {
  if (!fs.existsSync(path.join(checkerLib, checkerFile))) {
    fail(
      `missing ${checkerPackage}/lib/${checkerFile}; run mise exec -- pnpm install`,
    );
  }
}

const stagingScript = fs.readFileSync(
  path.join(repoRoot, 'scripts', 'stage-typescript-native.cjs'),
  'utf8',
);
const typecheckModule = fs.readFileSync(
  path.join(
    repoRoot,
    'src',
    'app',
    'electron',
    'extension-host',
    'typescript-api.ts',
  ),
  'utf8',
);
if (
  !/typescript-native/.test(stagingScript) ||
  !/typescript-native/.test(typecheckModule)
) {
  fail(
    'scripts/stage-typescript-native.cjs and src/app/electron/extension-host/typescript-api.ts must agree on the Resources/typescript-native staging path',
  );
}

if (!packageJson.dependencies?.['dom-serializer']) {
  fail(
    'dom-serializer must be a direct dependency so Electron packages pi-web-access runtime imports',
  );
}

if (
  !fs.existsSync(
    path.join(packageRoot, 'node_modules', 'dom-serializer', 'package.json'),
  )
) {
  fail(
    'missing node_modules/dom-serializer/package.json; run mise exec -- pnpm install',
  );
}

if (/^\s*-\s+src\/app\/resources\/\*\*\s*$/m.test(config)) {
  fail(
    'src/app/resources must be packaged with a FileSet; electron-builder default app-root ignores drop .d.ts files',
  );
}

if (
  !/from:\s*src\/app\/resources[\s\S]*to:\s*src\/app\/resources[\s\S]*filter:[\s\S]*['"]?\*\*\/\*['"]?/m.test(
    config,
  )
) {
  fail(
    'electron-builder.yml must include src/app/resources as a FileSet so the extension API declaration is packaged',
  );
}

if (
  /from:\s*node_modules\/typescript\/lib[\s\S]*to:\s*node_modules\/typescript\/lib/.test(
    config,
  )
) {
  fail(
    'electron-builder.yml must not package node_modules/typescript/lib; TypeScript 7 default libs are staged with the native checker by scripts/stage-typescript-native.cjs',
  );
}

if (
  !/afterPack:\s*scripts\/after-pack\.cjs/.test(config) ||
  !/stageTypeScriptNative/.test(
    fs.readFileSync(path.join(repoRoot, 'scripts', 'after-pack.cjs'), 'utf8'),
  )
) {
  fail(
    'electron-builder.yml must stage the native TypeScript checker via afterPack so generated extension validation works in the app',
  );
}

if (/src\/app\/fixtures/.test(config)) {
  fail(
    'electron-builder.yml must not include src/app/fixtures; fixtures are dev-only and excluded from shipped product',
  );
}

console.log('Packaged resource checks passed');
