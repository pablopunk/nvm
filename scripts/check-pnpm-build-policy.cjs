#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const root = process.cwd();

function configuredBuildDependencies(directory) {
  const configPath = path.join(directory, 'pnpm-workspace.yaml');
  const config = fs.readFileSync(configPath, 'utf8');
  const dependencies = config.match(
    /^onlyBuiltDependencies:\n((?:\s+-\s+.+\n?)+)$/m,
  )?.[1];
  if (!dependencies) {
    throw new Error(`${configPath} must configure onlyBuiltDependencies.`);
  }
  return dependencies
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      return line.replace(/^\s+-\s+/, '').replace(/^'|'$/g, '');
    });
}

function assertDependencies(directory, expected) {
  const actual = configuredBuildDependencies(directory);
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    throw new Error(
      `${path.relative(root, directory) || '.'} onlyBuiltDependencies must be ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}`,
    );
  }
}

const manifest = JSON.parse(
  fs.readFileSync(path.join(root, 'package.json'), 'utf8'),
);
if ('pnpm' in manifest) {
  throw new Error(
    'package.json must not contain pnpm build-script settings; pnpm 10 reads them from pnpm-workspace.yaml.',
  );
}

assertDependencies(root, [
  '@google/genai',
  'electron',
  'electron-winstaller',
  'esbuild',
  'protobufjs',
]);
assertDependencies(path.join(root, 'backend'), [
  '@sentry/cli',
  'esbuild',
  'sharp',
]);

console.log('pnpm build-script policy checks passed');
