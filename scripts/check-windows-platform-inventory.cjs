#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

const root = process.cwd();
const sourceRoot = path.join(root, 'src');
const inventoryPath = path.join(
  sourceRoot,
  'docs',
  'windows-platform-inventory.json',
);
const inventory = JSON.parse(fs.readFileSync(inventoryPath, 'utf8'));
const osLabels = [
  'Finder',
  'Quick Look',
  'Spotlight',
  'System Settings',
  'Windows Settings',
  'Start Menu',
  'Taskbar',
  'File Explorer',
  'Control Panel',
  'Dock',
];

function fail(message) {
  console.error(`Windows platform inventory check failed: ${message}`);
  process.exitCode = 1;
}

function walkSourceFiles(directory) {
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== 'fixtures') files.push(...walkSourceFiles(fullPath));
    } else if (
      /\.(?:ts|tsx)$/.test(entry.name) &&
      !entry.name.includes('.test.') &&
      !entry.name.endsWith('.d.ts')
    ) {
      files.push(fullPath);
    }
  }
  return files;
}

function propertyName(node) {
  if (ts.isIdentifier(node)) return node.text;
  if (ts.isStringLiteral(node)) return node.text;
  return '';
}

function categoriesForFile(filePath) {
  const relativePath = path.relative(root, filePath).replaceAll(path.sep, '/');
  const sourceText = fs.readFileSync(filePath, 'utf8');
  const sourceFile = ts.createSourceFile(
    filePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    filePath.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const categories = new Set();

  function visit(node) {
    if (
      ts.isPropertyAccessExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === 'process' &&
      node.name.text === 'platform'
    )
      categories.add('direct-platform');
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      ts.isIdentifier(node.expression.expression) &&
      node.expression.expression.text === 'os' &&
      node.expression.name.text === 'platform'
    )
      categories.add('direct-platform');
    if (
      (ts.isIdentifier(node) && node.text === 'processPlatform') ||
      (ts.isPropertyAccessExpression(node) &&
        node.name.text === 'processPlatform')
    )
      categories.add('injected-platform');
    if (
      relativePath === 'src/electron/os.ts' &&
      (ts.isPropertyAssignment(node) || ts.isMethodDeclaration(node)) &&
      ['darwin', 'linux', 'win32'].includes(propertyName(node.name))
    )
      categories.add('platform-selector');
    if (
      (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) &&
      osLabels.some((label) => node.text.includes(label))
    )
      categories.add('os-label');
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);

  if (
    /knownUserFolder(?:Path)?/.test(sourceText) ||
    /['"]Desktop['"]\s*,\s*['"]Documents['"]\s*,\s*['"]Downloads['"]/.test(
      sourceText,
    )
  )
    categories.add('known-folder');
  return { categories, relativePath };
}

function validateInventoryShape() {
  if (inventory.schemaVersion !== 1) fail('schemaVersion must be 1');
  if (inventory.supportStatus !== 'unverified')
    fail('supportStatus must remain unverified until the manual gate passes');
  const allowedStatuses = new Set([
    'implemented',
    'intentionally-unsupported',
    'missing',
    'unverified',
  ]);
  for (const surface of inventory.productSurfaces || []) {
    for (const field of [
      'key',
      'status',
      'source',
      'automatedCheck',
      'manualGate',
      'owner',
    ])
      if (!surface[field])
        fail(
          `product surface ${surface.key || '<unknown>'} is missing ${field}`,
        );
    if (!allowedStatuses.has(surface.status))
      fail(
        `product surface ${surface.key} has invalid status ${surface.status}`,
      );
  }
  if ((inventory.productSurfaces || []).length < 18)
    fail('product surface inventory is incomplete');
}

function validateResolvedEntries() {
  for (const entry of inventory.resolvedEntries || []) {
    const fullPath = path.join(root, entry.file);
    if (!fs.existsSync(fullPath))
      fail(`resolved entry ${entry.key} has no file`);
    else if (!fs.readFileSync(fullPath, 'utf8').includes(entry.evidencePattern))
      fail(
        `resolved entry ${entry.key} lost evidence pattern ${entry.evidencePattern}`,
      );
  }
}

function validateObservedCategories() {
  const observed = new Set();
  const rules = inventory.sourceRules || [];
  for (const filePath of walkSourceFiles(sourceRoot)) {
    const { categories, relativePath } = categoriesForFile(filePath);
    for (const category of categories) {
      const matchingRules = rules.filter(
        (rule) => rule.file === relativePath && rule.category === category,
      );
      if (matchingRules.length !== 1)
        fail(
          `${relativePath} has unowned ${category} sites (expected exactly one inventory rule, found ${matchingRules.length})`,
        );
      else observed.add(matchingRules[0].key);
    }
  }
  for (const rule of rules) {
    if (!observed.has(rule.key))
      fail(`stale inventory rule ${rule.key} (${rule.file}, ${rule.category})`);
  }
}

validateInventoryShape();
validateResolvedEntries();
validateObservedCategories();
if (process.exitCode) process.exit(1);
console.log(
  `Windows platform inventory checks passed (${inventory.sourceRules.length} source rules, ${inventory.productSurfaces.length} product surfaces)`,
);
