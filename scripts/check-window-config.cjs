#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const OS_FILE = path.join(ROOT, 'src', 'electron', 'os.ts');

/**
 * Contract points from src/docs/window-management.md.
 * Each entry: [functionName, requiredSubstrings[]]
 *
 * The doc requires that the main palette window on macOS combines:
 *   1. type: 'panel'         (paletteBrowserWindowOptions)
 *   2. activationPolicy       (prepareAppWindowPolicy)
 *   3. visibleOnAllWorkspaces (applyPaletteWindowPolicy)
 *   4. non-normal always-on-top (applyPaletteWindowPolicy)
 */
const CONTRACTS = [
  {
    name: 'paletteBrowserWindowOptions',
    required: ["type: 'panel'", 'darwin'],
    message: "must return { type: 'panel' } on darwin",
    docRef: 'src/docs/window-management.md:7',
  },
  {
    name: 'applyPaletteWindowPolicy',
    required: [
      'setAlwaysOnTop(true',
      'screen-saver',
      'setVisibleOnAllWorkspaces(true',
      'visibleOnFullScreen: true',
      'darwin',
    ],
    message:
      'must set always-on-top (screen-saver) and visible-on-all-workspaces on macOS',
    docRef: 'src/docs/window-management.md:7-10',
  },
  {
    name: 'prepareAppWindowPolicy',
    required: ["setActivationPolicy('accessory')", 'darwin'],
    message: 'must set activationPolicy to accessory on macOS',
    docRef: 'src/docs/window-management.md:8',
  },
];

// ── helpers ─────────────────────────────────────────────────────────────────

function fail(message) {
  console.error(`Window config check failed: ${message}`);
  process.exitCode = 1;
}

function relative(filePath) {
  return path.relative(ROOT, filePath);
}

/**
 * Find a named function declaration or exported arrow function in a TS source file.
 * Returns the source text substring for that function.
 */
function findFunctionSource(source, sf, name) {
  let found = null;

  function visit(node) {
    // Named function: function applyPaletteWindowPolicy(...) { ... }
    if (
      ts.isFunctionDeclaration(node) &&
      node.name &&
      node.name.text === name
    ) {
      found = { start: node.pos, end: node.end };
      return;
    }
    // Exported arrow: export function applyPaletteWindowPolicy(...) { ... }
    if (ts.isVariableStatement(node)) {
      for (const decl of node.declarationList.declarations) {
        if (ts.isIdentifier(decl.name) && decl.name.text === name) {
          found = { start: node.pos, end: node.end };
          return;
        }
      }
    }
    ts.forEachChild(node, visit);
  }

  visit(sf);
  if (!found) return null;
  return source.substring(found.start, found.end);
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const source = fs.readFileSync(OS_FILE, 'utf8');
  const sf = ts.createSourceFile('os.ts', source, ts.ScriptTarget.Latest, true);

  for (const contract of CONTRACTS) {
    const fnSource = findFunctionSource(source, sf, contract.name);
    if (!fnSource) {
      fail(
        `Function "${contract.name}" not found in ${relative(OS_FILE)}.\n` +
          `  Contract: ${contract.message} (${contract.docRef})`,
      );
      continue;
    }

    for (const required of contract.required) {
      if (!fnSource.includes(required)) {
        fail(
          `"${required}" missing from ${contract.name}() in ${relative(OS_FILE)}.\n` +
            `  Contract: ${contract.message} (${contract.docRef})`,
        );
      }
    }
  }

  if (process.exitCode) {
    console.error('\nWindow config checks failed. See errors above.');
    process.exit(1);
  }

  console.log('Window config checks passed');
  console.log(`  Functions verified: ${CONTRACTS.length}`);
}

main();
