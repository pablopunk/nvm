#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');

const root = process.cwd();
const runtimeRoot = path.join(root, 'src', 'electron');
const forbiddenCalls = [
  'accessSync',
  'appendFileSync',
  'closeSync',
  'copyFileSync',
  'execFileSync',
  'execSync',
  'existsSync',
  'lstatSync',
  'mkdirSync',
  'openSync',
  'readFileSync',
  'readlinkSync',
  'readdirSync',
  'realpathSync',
  'renameSync',
  'rmSync',
  'spawnSync',
  'statSync',
  'symlinkSync',
  'truncateSync',
  'unlinkSync',
  'writeFileSync',
];
const blockingProviderPatterns = [
  /\b(?:rootItems|searchItems)\s*:\s*async\b/,
  /\basync\s+(?:rootItems|searchItems)\s*\(/,
];

function runtimeFiles(directory) {
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const filePath = path.join(directory, entry.name);
    if (entry.isDirectory()) files.push(...runtimeFiles(filePath));
    else if (
      entry.name.endsWith('.ts') &&
      !entry.name.includes('.test.') &&
      entry.name !== 'test-mode.ts'
    )
      files.push(filePath);
  }
  return files;
}

const violations = [];
for (const filePath of runtimeFiles(runtimeRoot)) {
  const lines = fs.readFileSync(filePath, 'utf8').split('\n');
  for (let index = 0; index < lines.length; index += 1) {
    for (const call of forbiddenCalls) {
      if (new RegExp(`\\b${call}\\s*\\(`).test(lines[index]))
        violations.push({ filePath, line: index + 1, call });
    }
    if (blockingProviderPatterns.some((pattern) => pattern.test(lines[index])))
      violations.push({
        filePath,
        line: index + 1,
        call: 'async root/search provider',
      });
  }
}

if (violations.length) {
  console.error(
    'Runtime non-blocking check failed: synchronous I/O is forbidden in shipped Electron code.',
  );
  for (const violation of violations)
    console.error(
      `  ${path.relative(root, violation.filePath)}:${violation.line} uses ${violation.call}`,
    );
  process.exit(1);
}

console.log('Runtime non-blocking checks passed');
