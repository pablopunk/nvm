#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const SRC = path.join(ROOT, 'src');

const inventory = JSON.parse(
  fs.readFileSync(
    path.join(SRC, 'docs', 'windows-platform-inventory.json'),
    'utf8',
  ),
);
const PLATFORM_ALLOWLIST = new Set(
  inventory.sourceRules
    .filter((rule) => rule.category === 'direct-platform')
    .map((rule) => rule.file),
);
const LABEL_ALLOWLIST = new Set(
  inventory.sourceRules
    .filter((rule) => rule.category === 'os-label')
    .map((rule) => rule.file),
);

/**
 * OS-specific UI labels that should not appear in renderer/shared code.
 * The OS capability layer (src/electron/os.ts) owns these; shared code
 * should use capability names instead.
 */
const OS_UI_LABELS = [
  'Finder',
  'Quick Look',
  'Spotlight',
  'System Settings',
  'Start Menu',
  'Taskbar',
  'File Explorer',
  'Control Panel',
  'Dock',
];

// ── helpers ─────────────────────────────────────────────────────────────────

function fail(message) {
  console.error(`OS platform boundary check failed: ${message}`);
  process.exitCode = 1;
}

function normalizeRepositoryPath(filePath) {
  return filePath.replaceAll('\\', '/');
}

function relative(filePath) {
  return normalizeRepositoryPath(path.relative(ROOT, filePath));
}

function isElectronFile(filePath) {
  return (
    filePath.startsWith(path.join(SRC, 'electron') + path.sep) ||
    filePath === path.join(SRC, 'electron')
  );
}

function isAllowedPlatformFile(filePath) {
  const rel = relative(filePath);
  return PLATFORM_ALLOWLIST.has(rel);
}

function isRendererOrShared(filePath) {
  const rel = relative(filePath);
  // Anything under src/ that is NOT under src/electron/ and NOT a .d.ts
  if (rel.endsWith('.d.ts')) return false;
  if (rel.startsWith('src/electron/')) return false;
  if (rel.startsWith('src/fixtures/')) return false;
  return (
    rel.startsWith('src/') && (rel.endsWith('.ts') || rel.endsWith('.tsx'))
  );
}

// ── file walking ────────────────────────────────────────────────────────────

function walkTsFiles(dir) {
  if (!fs.existsSync(dir)) return [];
  const results = [];
  const skip = new Set([
    'node_modules',
    'dist',
    '.git',
    'backend',
    'build',
    'release',
  ]);
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (skip.has(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      results.push(...walkTsFiles(full));
    } else if (
      /\.(ts|tsx)$/.test(entry.name) &&
      !entry.name.includes('.test.')
    ) {
      results.push(full);
    }
  }
  return results;
}

// ── checks ──────────────────────────────────────────────────────────────────

function checkFile(filePath) {
  const source = fs.readFileSync(filePath, 'utf8');
  const lines = source.split('\n');
  const rel = relative(filePath);

  // Check for process.platform
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const lineNum = i + 1;

    if (
      /\bprocess\.platform\b/.test(line) ||
      /\bos\.platform\(\)\b/.test(line)
    ) {
      if (isAllowedPlatformFile(filePath)) continue;
      fail(
        `${rel}:${lineNum} — unowned process.platform site; add an injected capability or explicit inventory disposition`,
      );
    }
  }

  // Check for OS-specific UI labels in renderer/shared code
  if (isRendererOrShared(filePath)) {
    for (const label of OS_UI_LABELS) {
      if (source.includes(label)) {
        if (!LABEL_ALLOWLIST.has(rel))
          fail(
            `${rel} — unowned OS-specific label "${label}" in shared code; use OS capability labels instead`,
          );
      }
    }
  }
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const files = walkTsFiles(SRC);

  for (const filePath of files) {
    checkFile(filePath);
  }

  if (process.exitCode) {
    console.error(`\nOS platform boundary checks failed. See errors above.`);
    process.exit(1);
  }

  console.log('OS platform boundary checks passed');
  console.log(`  Files scanned: ${files.length}`);
}

if (require.main === module) main();

module.exports = { normalizeRepositoryPath };
