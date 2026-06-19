#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const API_DTS = path.join(
  ROOT,
  'src',
  'resources',
  'nevermind-extension-api.d.ts',
);
const MAIN_TS = path.join(ROOT, 'src', 'electron', 'main.ts');

/**
 * Per src/docs/solutions/extension-result-appearance-contract.md,
 * `appearance` must exist on:
 *   - ExtensionItem          (the original type)
 *   - ExtensionActionContribution  (durable actions)
 *   - ExtensionCommand             (command declarations)
 *
 * And runtime command registration must propagate command.appearance
 * into the registered action item.
 */
const TYPES_REQUIRING_APPEARANCE = [
  'ExtensionActionContribution',
  'ExtensionCommand',
  'ExtensionItem',
];

// ── helpers ─────────────────────────────────────────────────────────────────

function fail(message) {
  console.error(`Appearance contract check failed: ${message}`);
  process.exitCode = 1;
}

function relative(filePath) {
  return path.relative(ROOT, filePath);
}

function findTypeAlias(sf, name) {
  let result = null;
  function visit(node) {
    if (ts.isTypeAliasDeclaration(node) && node.name.text === name) {
      result = node;
      return;
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);
  return result;
}

function typeLiteralHasMember(typeLiteral, memberName) {
  if (!typeLiteral || !ts.isTypeLiteralNode(typeLiteral)) return false;
  for (const member of typeLiteral.members) {
    if (
      member.name &&
      ts.isIdentifier(member.name) &&
      member.name.text === memberName
    ) {
      return true;
    }
  }
  return false;
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const dtsSource = fs.readFileSync(API_DTS, 'utf8');
  const sf = ts.createSourceFile(
    'api.d.ts',
    dtsSource,
    ts.ScriptTarget.Latest,
    true,
  );

  // 1. Verify appearance exists on each required type
  for (const typeName of TYPES_REQUIRING_APPEARANCE) {
    const typeDecl = findTypeAlias(sf, typeName);
    if (!typeDecl) {
      fail(`Type "${typeName}" not found in ${relative(API_DTS)}`);
      continue;
    }
    if (!typeLiteralHasMember(typeDecl.type, 'appearance')) {
      fail(
        `"appearance" field missing from ${typeName} in ${relative(API_DTS)}.\n` +
          `  Per src/docs/solutions/extension-result-appearance-contract.md, ` +
          `all result-shaped extension surfaces must support appearance.foreground.`,
      );
    }
  }

  // 2. Verify runtime command appearance propagation in main.ts
  const mainSource = fs.readFileSync(MAIN_TS, 'utf8');
  if (
    !mainSource.includes(
      'appearance: normalizeItemAppearance(command.appearance)',
    )
  ) {
    fail(
      `Command appearance propagation missing from ${relative(MAIN_TS)}.\n` +
        `  Command registration must copy command.appearance into the registered action item.\n` +
        `  Expected: appearance: normalizeItemAppearance(command.appearance)`,
    );
  }

  if (process.exitCode) {
    console.error('\nAppearance contract checks failed. See errors above.');
    process.exit(1);
  }

  console.log('Appearance contract checks passed');
  console.log(`  Types verified: ${TYPES_REQUIRING_APPEARANCE.length}`);
  console.log(`  Runtime propagation: ok`);
}

main();
