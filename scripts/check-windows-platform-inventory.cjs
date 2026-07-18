#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { createHash } = require('node:crypto');
const ts = require('typescript');

const OS_LABELS = [
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
const CATEGORIES = new Set([
  'direct-platform',
  'injected-platform',
  'known-folder',
  'os-label',
  'platform-selector',
]);

function normalizeRepositoryPath(filePath) {
  return filePath.replaceAll('\\', '/');
}

function normalizedSource(text) {
  return text.replace(/\s+/g, ' ').trim();
}

function stableDigest(text) {
  return createHash('sha256').update(text).digest('hex').slice(0, 12);
}

function conciseEvidence(text) {
  if (text.length <= 120) return text;
  return `${text.slice(0, 96)}…#${stableDigest(text)}`;
}

function propertyName(node) {
  if (ts.isIdentifier(node) || ts.isStringLiteral(node)) return node.text;
  return '';
}

function declarationName(node) {
  if ('name' in node && node.name) return propertyName(node.name);
  if (
    ts.isVariableDeclaration(node) &&
    (ts.isIdentifier(node.name) || ts.isStringLiteral(node.name))
  )
    return node.name.text;
  return '';
}

function structuralScope(node, sourceFile) {
  const segments = [];
  for (let current = node.parent; current; current = current.parent) {
    let kind = '';
    if (ts.isIfStatement(current))
      segments.unshift(
        `if:${conciseEvidence(normalizedSource(current.expression.getText(sourceFile)))}`,
      );
    if (ts.isClassDeclaration(current)) kind = 'class';
    else if (ts.isFunctionDeclaration(current)) kind = 'function';
    else if (ts.isMethodDeclaration(current)) kind = 'method';
    else if (ts.isVariableDeclaration(current)) kind = 'variable';
    else if (ts.isPropertyAssignment(current)) kind = 'property';
    else if (ts.isGetAccessorDeclaration(current)) kind = 'getter';
    else if (ts.isSetAccessorDeclaration(current)) kind = 'setter';
    if (ts.isCaseClause(current))
      segments.unshift(
        `case:${conciseEvidence(normalizedSource(current.expression.getText(sourceFile)))}`,
      );
    if (ts.isCallExpression(current)) {
      const identifyingArguments = current.arguments
        .filter(
          (argument) =>
            ts.isStringLiteral(argument) ||
            ts.isNumericLiteral(argument) ||
            argument.kind === ts.SyntaxKind.TrueKeyword ||
            argument.kind === ts.SyntaxKind.FalseKeyword,
        )
        .map((argument) => normalizedSource(argument.getText(sourceFile)));
      if (identifyingArguments.length) {
        segments.unshift(
          `call:${conciseEvidence(normalizedSource(current.expression.getText(sourceFile)))}(${identifyingArguments.join(',')})`,
        );
      }
    }
    if (!kind) continue;
    const name = declarationName(current);
    if (name) segments.unshift(`${kind}:${name}`);
  }
  return segments.join('/') || 'module';
}

function evidenceNode(node) {
  let current = node;
  while (current.parent) {
    const parent = current.parent;
    if (
      ts.isSourceFile(parent) ||
      ts.isBlock(parent) ||
      ts.isCaseBlock(parent) ||
      ts.isObjectLiteralExpression(parent)
    )
      break;
    if (
      ts.isFunctionDeclaration(parent) ||
      ts.isMethodDeclaration(parent) ||
      ts.isClassDeclaration(parent)
    )
      break;
    current = parent;
  }
  return current;
}

function childRole(parent, child) {
  for (const [key, value] of Object.entries(parent)) {
    if (key === 'parent') continue;
    if (value === child) return key;
    if (Array.isArray(value)) {
      const index = value.indexOf(child);
      if (index !== -1) return `${key}[${index}]`;
    }
  }
  return ts.SyntaxKind[child.kind];
}

function structuralRole(node, rootNode) {
  const roles = [];
  let current = node;
  while (current !== rootNode && current.parent) {
    const parent = current.parent;
    roles.unshift(childRole(parent, current));
    current = parent;
  }
  return roles.join('.') || 'self';
}

function siteSelector(category, node, sourceFile) {
  const evidenceRoot = evidenceNode(node);
  const evidence = conciseEvidence(
    normalizedSource(evidenceRoot.getText(sourceFile)),
  );
  const role = structuralRole(node, evidenceRoot);
  return `${structuralScope(node, sourceFile)}::${category}::${evidence}::${role}`;
}

function isProcessPlatform(node) {
  return (
    ts.isPropertyAccessExpression(node) &&
    ts.isIdentifier(node.expression) &&
    node.expression.text === 'process' &&
    node.name.text === 'platform'
  );
}

function isOsPlatformCall(node) {
  return (
    ts.isCallExpression(node) &&
    ts.isPropertyAccessExpression(node.expression) &&
    ts.isIdentifier(node.expression.expression) &&
    node.expression.expression.text === 'os' &&
    node.expression.name.text === 'platform'
  );
}

function isInjectedPlatformSite(node) {
  if (
    ts.isPropertyAccessExpression(node) &&
    node.name.text === 'processPlatform'
  )
    return true;
  if (!ts.isIdentifier(node) || node.text !== 'processPlatform') return false;
  return !(
    (ts.isPropertyAccessExpression(node.parent) && node.parent.name === node) ||
    (ts.isPropertySignature(node.parent) && node.parent.name === node) ||
    (ts.isPropertyAssignment(node.parent) && node.parent.name === node)
  );
}

function isKnownFolderCall(node) {
  if (!ts.isCallExpression(node)) return false;
  const expression = node.expression;
  return (
    (ts.isIdentifier(expression) &&
      /^knownUserFolder(?:Path)?$/.test(expression.text)) ||
    (ts.isPropertyAccessExpression(expression) &&
      /^knownUserFolder(?:Path)?$/.test(expression.name.text))
  );
}

function isKnownFolderConstructor(node) {
  return (
    ts.isFunctionDeclaration(node) &&
    node.name &&
    /^knownUserFolder(?:Path)?$/.test(node.name.text)
  );
}

function isKnownFolderArray(node) {
  if (!ts.isArrayLiteralExpression(node)) return false;
  return ['Desktop', 'Documents', 'Downloads'].every((folder) =>
    node.elements.some(
      (element) => ts.isStringLiteral(element) && element.text === folder,
    ),
  );
}

function detectSitesInSource(relativePath, sourceText) {
  const sourceFile = ts.createSourceFile(
    relativePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    relativePath.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const sites = [];

  function add(category, node) {
    const selector = siteSelector(category, node, sourceFile);
    sites.push({
      file: relativePath,
      category,
      selector,
      evidence: normalizedSource(node.getText(sourceFile)),
    });
  }

  function visit(node) {
    if (isProcessPlatform(node) || isOsPlatformCall(node))
      add('direct-platform', node);
    if (isInjectedPlatformSite(node)) add('injected-platform', node);
    if (
      relativePath === 'src/electron/os.ts' &&
      (ts.isPropertyAssignment(node) || ts.isMethodDeclaration(node)) &&
      ['darwin', 'linux', 'win32'].includes(propertyName(node.name))
    )
      add('platform-selector', node);
    if (
      (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) &&
      OS_LABELS.some((label) => node.text.includes(label))
    )
      add('os-label', node);
    if (
      isKnownFolderCall(node) ||
      isKnownFolderConstructor(node) ||
      isKnownFolderArray(node)
    )
      add('known-folder', node);
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return sites;
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

function collectObservedSites(rootDirectory) {
  const sourceRoot = path.join(rootDirectory, 'src');
  return walkSourceFiles(sourceRoot).flatMap((filePath) => {
    const relativePath = normalizeRepositoryPath(
      path.relative(rootDirectory, filePath),
    );
    return detectSitesInSource(relativePath, fs.readFileSync(filePath, 'utf8'));
  });
}

function validateInventoryShape(inventory) {
  const errors = [];
  if (inventory.schemaVersion !== 2) errors.push('schemaVersion must be 2');
  if (inventory.supportStatus !== 'unverified')
    errors.push(
      'supportStatus must remain unverified until the manual gate passes',
    );
  const allowedStatuses = new Set([
    'implemented',
    'intentionally-unsupported',
    'missing',
    'unverified',
  ]);
  const surfaces = inventory.productSurfaces || [];
  const surfaceKeys = new Set(surfaces.map((surface) => surface.key));
  for (const surface of surfaces) {
    for (const field of [
      'key',
      'status',
      'source',
      'automatedCheck',
      'manualGate',
      'owner',
    ])
      if (!surface[field])
        errors.push(
          `product surface ${surface.key || '<unknown>'} is missing ${field}`,
        );
    if (!allowedStatuses.has(surface.status))
      errors.push(
        `product surface ${surface.key} has invalid status ${surface.status}`,
      );
  }
  if (surfaces.length < 18)
    errors.push('product surface inventory is incomplete');

  const seenKeys = new Set();
  for (const rule of inventory.sourceRules || []) {
    for (const field of [
      'key',
      'file',
      'category',
      'selector',
      'disposition',
      'surface',
    ])
      if (!rule[field])
        errors.push(
          `source rule ${rule.key || '<unknown>'} is missing ${field}`,
        );
    if (seenKeys.has(rule.key))
      errors.push(`duplicate source rule key ${rule.key}`);
    seenKeys.add(rule.key);
    if (!CATEGORIES.has(rule.category))
      errors.push(
        `source rule ${rule.key} has invalid category ${rule.category}`,
      );
    if (rule.surface && !surfaceKeys.has(rule.surface))
      errors.push(
        `source rule ${rule.key} maps to unknown surface ${rule.surface}`,
      );
  }
  return errors;
}

function validateObservedSites(inventory, observedSites, options = {}) {
  const errors = [];
  const observedKeys = new Set();
  const rules = inventory.sourceRules || [];
  const sitesByIdentity = new Map();
  for (const site of observedSites) {
    const identity = `${site.file}\0${site.category}\0${site.selector}`;
    const matchingSites = sitesByIdentity.get(identity) || [];
    matchingSites.push(site);
    sitesByIdentity.set(identity, matchingSites);
  }
  for (const matchingSites of sitesByIdentity.values()) {
    const [site] = matchingSites;
    const matchingRules = rules.filter(
      (rule) =>
        rule.file === site.file &&
        rule.category === site.category &&
        rule.selector === site.selector,
    );
    if (matchingSites.length !== 1 || matchingRules.length !== 1) {
      errors.push(
        `${site.file} has unowned ${site.category} site ${JSON.stringify(site.selector)} (expected one detected site and one inventory rule, found ${matchingSites.length} sites and ${matchingRules.length} rules)`,
      );
    } else {
      observedKeys.add(matchingRules[0].key);
    }
  }
  if (options.checkStale !== false) {
    for (const rule of rules) {
      if (!observedKeys.has(rule.key))
        errors.push(
          `stale inventory rule ${rule.key} (${rule.file}, ${rule.category}, ${JSON.stringify(rule.selector)})`,
        );
    }
  }
  return errors;
}

function validateResolvedEntries(rootDirectory, inventory) {
  const errors = [];
  for (const entry of inventory.resolvedEntries || []) {
    const fullPath = path.join(rootDirectory, entry.file);
    if (!fs.existsSync(fullPath))
      errors.push(`resolved entry ${entry.key} has no file`);
    else if (!fs.readFileSync(fullPath, 'utf8').includes(entry.evidencePattern))
      errors.push(
        `resolved entry ${entry.key} lost evidence pattern ${entry.evidencePattern}`,
      );
  }
  return errors;
}

function main() {
  const rootDirectory = process.cwd();
  const inventoryPath = path.join(
    rootDirectory,
    'src',
    'docs',
    'windows-platform-inventory.json',
  );
  const inventory = JSON.parse(fs.readFileSync(inventoryPath, 'utf8'));
  const errors = [
    ...validateInventoryShape(inventory),
    ...validateResolvedEntries(rootDirectory, inventory),
    ...validateObservedSites(inventory, collectObservedSites(rootDirectory)),
  ];
  if (errors.length) {
    for (const error of errors)
      console.error(`Windows platform inventory check failed: ${error}`);
    process.exit(1);
  }
  console.log(
    `Windows platform inventory checks passed (${inventory.sourceRules.length} source sites, ${inventory.productSurfaces.length} product surfaces)`,
  );
}

if (require.main === module) main();

module.exports = {
  collectObservedSites,
  detectSitesInSource,
  normalizeRepositoryPath,
  validateInventoryShape,
  validateObservedSites,
};
