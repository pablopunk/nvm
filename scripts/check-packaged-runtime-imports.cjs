#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

const root = process.cwd();
const runtimeDirs = [
  path.join(root, 'dist', 'main'),
  path.join(root, 'dist', 'preload'),
];

const forbiddenPackages = [
  'react',
  'react-dom',
  'lucide-react',
  'cmdk',
  'react-markdown',
  'remark-gfm',
  'vite',
  '@vitejs/plugin-react',
  '@ai-sdk/anthropic',
  '@ai-sdk/google',
  '@ai-sdk/openai',
];

const allowedPackages = [
  '@earendil-works/pi-ai',
  '@earendil-works/pi-coding-agent',
  '@sentry/electron',
  'electron',
  'electron-log',
  'electron-updater',
  'file-icon',
  'typescript',
];

function fail(message) {
  console.error(`Packaged runtime import check failed: ${message}`);
  process.exitCode = 1;
}

function packageName(specifier) {
  if (
    !specifier ||
    specifier.startsWith('.') ||
    specifier.startsWith('/') ||
    specifier.startsWith('node:') ||
    specifier.startsWith('file:')
  )
    return null;
  const parts = specifier.split('/');
  if (specifier.startsWith('@'))
    return parts.length >= 2 ? `${parts[0]}/${parts[1]}` : specifier;
  return parts[0];
}

function matchesPackage(specifier, packagePrefix) {
  return (
    specifier === packagePrefix || specifier.startsWith(`${packagePrefix}/`)
  );
}

function findRuntimeFiles(dir) {
  if (!fs.existsSync(dir))
    fail(`missing ${path.relative(root, dir)}; run the build first`);
  if (!fs.existsSync(dir)) return [];
  const files = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) files.push(...findRuntimeFiles(fullPath));
    else if (/\.(?:c?m?js)$/.test(entry.name)) files.push(fullPath);
  }
  return files;
}

function importSpecifiers(source) {
  const specifiers = new Set();
  const patterns = [
    /\bimport\s+(?:[^'"()]+?\s+from\s*)?['"]([^'"]+)['"]/g,
    /\bexport\s+[^'"]*?\s+from\s+['"]([^'"]+)['"]/g,
    /\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
    /\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
    /\bcreateRequire\([^)]*\)\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
  ];
  for (const pattern of patterns) {
    for (const match of source.matchAll(pattern)) specifiers.add(match[1]);
  }
  return Array.from(specifiers).sort();
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function forbiddenStringReferences(source) {
  const references = [];
  for (const packagePrefix of forbiddenPackages) {
    const pattern = new RegExp(
      `["']${escapeRegExp(packagePrefix)}(?:/[A-Za-z0-9._/-]*)?["']`,
      'g',
    );
    if (pattern.test(source)) references.push(packagePrefix);
  }
  return references;
}

const violations = [];
const externalImports = new Map();

for (const dir of runtimeDirs) {
  for (const filePath of findRuntimeFiles(dir)) {
    const source = fs.readFileSync(filePath, 'utf8');
    const relativePath = path.relative(root, filePath);
    for (const specifier of importSpecifiers(source)) {
      const pkg = packageName(specifier);
      if (!pkg) continue;
      if (!externalImports.has(specifier))
        externalImports.set(specifier, new Set());
      externalImports.get(specifier).add(relativePath);
      const forbidden = forbiddenPackages.find((packagePrefix) =>
        matchesPackage(specifier, packagePrefix),
      );
      if (forbidden)
        violations.push({ filePath: relativePath, specifier, forbidden });
    }
    for (const forbidden of forbiddenStringReferences(source)) {
      violations.push({
        filePath: relativePath,
        specifier: forbidden,
        forbidden,
      });
    }
  }
}

for (const violation of violations) {
  fail(
    `${violation.filePath} imports ${violation.specifier}; ${violation.forbidden} must stay out of packaged main/preload runtime`,
  );
}

if (process.exitCode) process.exit();

const unexpected = Array.from(externalImports.keys())
  .filter(
    (specifier) =>
      !allowedPackages.some((allowed) => matchesPackage(specifier, allowed)),
  )
  .sort();

if (unexpected.length) {
  console.warn(
    'Packaged runtime import check warning: unclassified external runtime imports:',
  );
  for (const specifier of unexpected) {
    const files = Array.from(externalImports.get(specifier) || []).join(', ');
    console.warn(`  - ${specifier} (${files})`);
  }
}

console.log('Packaged runtime import checks passed');
