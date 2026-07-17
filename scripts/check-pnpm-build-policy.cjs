#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const root = process.cwd();

function configuredBuildDependencies(directory, key) {
  const configPath = path.join(directory, 'pnpm-workspace.yaml');
  const config = fs.readFileSync(configPath, 'utf8');
  const entries = config.match(
    new RegExp(`^${key}:\\n((?:[ \\t]+.+\\n?)+)$`, 'm'),
  )?.[1];
  if (!entries) {
    throw new Error(`${configPath} must configure ${key}.`);
  }
  return entries
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const match = line.match(
        key === 'allowBuilds' ? /^\s+('?[^':]+'?): true$/ : /^\s+-\s+(.+)$/,
      );
      if (!match)
        throw new Error(`${configPath} must explicitly allow builds.`);
      return match[1].replace(/^'|'$/g, '');
    });
}

function assertDependencies(directory, key, expected) {
  const actual = configuredBuildDependencies(directory, key);
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(
      `${path.relative(root, directory) || '.'} ${key} must be ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`,
    );
  }
}

const manifest = JSON.parse(
  fs.readFileSync(path.join(root, 'package.json'), 'utf8'),
);
if ('pnpm' in manifest) {
  throw new Error(
    'package.json must not contain pnpm build-script settings; pnpm reads them from pnpm-workspace.yaml.',
  );
}
if (manifest.scripts.predev !== 'pnpm rebuild electron') {
  throw new Error(
    'package.json predev must rebuild Electron so pnpm run dev repairs a skipped binary automatically.',
  );
}

const rootDependencies = [
  '@google/genai',
  '@sentry/cli',
  'electron',
  'electron-winstaller',
  'esbuild',
  'protobufjs',
  'sharp',
];
const rootLegacyDependencies = rootDependencies.filter(
  (dependency) => dependency !== '@sentry/cli' && dependency !== 'sharp',
);
const backendDependencies = ['@sentry/cli', 'esbuild', 'sharp'];

assertDependencies(root, 'allowBuilds', rootDependencies);
assertDependencies(root, 'onlyBuiltDependencies', rootLegacyDependencies);
assertDependencies(
  path.join(root, 'backend'),
  'allowBuilds',
  backendDependencies,
);
assertDependencies(
  path.join(root, 'backend'),
  'onlyBuiltDependencies',
  backendDependencies,
);

console.log('pnpm build-script policy checks passed');
