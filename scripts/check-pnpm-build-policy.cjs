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
    .filter((line) => !(key === 'allowBuilds' && /: false$/.test(line)))
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
if (fs.existsSync(path.join(root, 'pnpm-workspace.yaml'))) {
  throw new Error(
    'The root has no dependencies with build scripts; remove pnpm-workspace.yaml or extend this check.',
  );
}

const backendDependencies = ['esbuild', 'sharp'];
const backendDirectory = path.join(root, 'src', 'backend');
assertDependencies(backendDirectory, 'allowBuilds', backendDependencies);
assertDependencies(
  backendDirectory,
  'onlyBuiltDependencies',
  backendDependencies,
);

console.log('pnpm build-script policy checks passed');
