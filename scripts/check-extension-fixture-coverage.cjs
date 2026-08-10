#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

// ── allowlist: host-rendered UI methods that require a fixture ──────────────
// Keep in sync with src/docs/extension-api-ui-fixtures.md § "Current Fixture".
const FIXTURE_REQUIRED = new Set([
  'ui.list',
  'ui.collection',
  'ui.grid',
  'ui.preview',
  'ui.chat',
  'ui.form',
  'input.prompt',
  'ui.editor',
  'ui.progress',
  'ui.webview',
  'ui.camera',
  'ui.confirm',
  'ui.toast',
]);

// Helpers (ui.item, ui.actions, ui.empty, ui.loading, ui.error) are
// pass-through constructors.  They are exercised by other fixtures and do
// not render host-owned UI independently — the doc explicitly omits them.

const ROOT = process.cwd();
const API_DTS = path.join(
  ROOT,
  'src',
  'resources',
  'nevermind-extension-api.d.ts',
);
const FIXTURE_FILE = path.join(ROOT, 'src', 'fixtures', 'ui-fixtures.ts');
const DOC_FILE = path.join(ROOT, 'src', 'docs', 'extension-api-ui-fixtures.md');

// ── helpers ─────────────────────────────────────────────────────────────────

function fail(message) {
  console.error(`Fixture coverage check failed: ${message}`);
  process.exitCode = 1;
}

function readFile(filePath) {
  return fs.readFileSync(filePath, 'utf8');
}

/** Context variable names used to access extension APIs in fixture code. */
function isContextVar(name) {
  return name === 'ctx' || name === 'innerCtx' || name === '_ctx';
}

// ── 1.  Extract ui.* / input.* method names from the public API type ────────

function extractUIMethods(dtsSource) {
  const sf = ts.createSourceFile(
    'api.d.ts',
    dtsSource,
    ts.ScriptTarget.Latest,
    true,
  );
  const ctxType = findTypeOrInterface(sf, 'ExtensionContext');
  if (!ctxType) {
    fail('ExtensionContext type not found in nevermind-extension-api.d.ts');
    return new Map();
  }
  return extractNamespaceMembers(ctxType, ['ui', 'input']);
}

/**
 * Walk an AST to find a named type alias or interface declaration.
 */
function findTypeOrInterface(node, name) {
  if (
    (ts.isTypeAliasDeclaration(node) || ts.isInterfaceDeclaration(node)) &&
    node.name.text === name
  ) {
    return node;
  }
  let result = undefined;
  ts.forEachChild(node, (child) => {
    const found = findTypeOrInterface(child, name);
    if (found) result = found;
  });
  return result;
}

/**
 * Given a type alias or interface declaration (ExtensionContext), find
 * properties named `ui` / `input` whose type is an object literal, then
 * collect the member *names* (collapsing overloads so `preview` counts once).
 *
 * Returns a Map<namespace, Set<methodName>>.
 */
function extractNamespaceMembers(decl, namespaces) {
  const result = new Map();
  for (const ns of namespaces) result.set(ns, new Set());

  // For type aliases the members are nested inside the type literal
  const typeLiteral = ts.isTypeAliasDeclaration(decl) ? decl.type : decl;
  if (!typeLiteral || !ts.isTypeLiteralNode(typeLiteral)) {
    fail('ExtensionContext must be a type literal (object type)');
    return result;
  }

  for (const member of typeLiteral.members) {
    if (!ts.isPropertySignature(member)) continue;
    const name =
      member.name && ts.isIdentifier(member.name)
        ? member.name.text
        : undefined;
    if (!name || !result.has(name)) continue;

    const type = member.type;
    if (!type || !ts.isTypeLiteralNode(type)) continue;

    for (const typeMember of type.members) {
      if (!typeMember.name) continue;
      let methodName;
      if (ts.isIdentifier(typeMember.name)) {
        methodName = typeMember.name.text;
      } else {
        // Computed name — skip
        continue;
      }
      result.get(name).add(methodName);
    }
  }
  return result;
}

// ── 2.  Collect fixture call sites (namespace + method) ─────────────────────

function collectFixtureCalls(fixtureSource) {
  const sf = ts.createSourceFile(
    'fixture.ts',
    fixtureSource,
    ts.ScriptTarget.Latest,
    true,
  );
  const calls = new Set();

  function visit(node) {
    // ctx.ui.X(...) or ctx.input.X(...)
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression)
    ) {
      const methodAccess = node.expression; // ctx.ui.list
      const nsAccess = methodAccess.expression; // ctx.ui

      if (
        ts.isPropertyAccessExpression(nsAccess) &&
        ts.isIdentifier(nsAccess.expression) &&
        isContextVar(nsAccess.expression.text) &&
        ts.isIdentifier(nsAccess.name) &&
        ts.isIdentifier(methodAccess.name)
      ) {
        const ns = nsAccess.name.text; // 'ui' | 'input'
        const method = methodAccess.name.text; // 'list' | 'prompt' | ...
        calls.add(`${ns}.${method}`);
      }
    }
    ts.forEachChild(node, visit);
  }

  visit(sf);
  return calls;
}

// ── 3.  Verify the doc allowlist is in sync with the actual API surface ─────

function verifyAllowlistAgainstAPI(uiMethods) {
  for (const qualified of FIXTURE_REQUIRED) {
    const [ns, method] = qualified.split('.');
    const apiMethods = uiMethods.get(ns);
    if (!apiMethods || !apiMethods.has(method)) {
      fail(
        `allowlist entry "${qualified}" is not present in ExtensionContext.${ns}. ` +
          `Remove it from the FIXTURE_REQUIRED set in this script and from ${path.relative(ROOT, DOC_FILE)}.`,
      );
    }
  }
}

// ── 4.  Check for new API methods that might need fixtures ──────────────────

function checkNewAPIMethods(uiMethods) {
  // Methods returning ExtensionView, ExtensionAction, or ExtensionToastResult
  // are "render-ish" and should trigger a review.  We can't easily inspect
  // return types with a shallow walk, so we flag any method on `ui` / `input`
  // that is NOT in the allowlist and NOT in a known-skip set.
  const SKIP_METHODS = new Set([
    'ui.item', // pass‑through helper
    'ui.actions', // pass‑through helper
    'ui.empty', // pass‑through helper, renders a view but is auxiliary
    'ui.loading', // pass‑through helper
    'ui.error', // pass‑through helper
  ]);

  for (const [ns, methods] of uiMethods) {
    for (const method of methods) {
      const qualified = `${ns}.${method}`;
      if (!FIXTURE_REQUIRED.has(qualified) && !SKIP_METHODS.has(qualified)) {
        fail(
          `New API method "${qualified}" found in ExtensionContext but not in the fixture allowlist. ` +
            `If it renders host-owned UI, add it to FIXTURE_REQUIRED in this script, ` +
            `add a fixture command in ${path.relative(ROOT, FIXTURE_FILE)}, ` +
            `and update ${path.relative(ROOT, DOC_FILE)}. ` +
            `If it is a pass‑through helper, add "${qualified}" to SKIP_METHODS.`,
        );
      }
    }
  }
}

// ── 5.  Check that every required method is exercised in the fixture ────────

function checkFixtureCoverage(calls) {
  for (const qualified of FIXTURE_REQUIRED) {
    if (!calls.has(qualified)) {
      fail(
        `"${qualified}" is required by the fixture doc but not called in ${path.relative(ROOT, FIXTURE_FILE)}. ` +
          `Add a command that exercises ${qualified} and update ${path.relative(ROOT, DOC_FILE)}.`,
      );
    }
  }
}

// ── 6.  Verify the doc lists the same surface as the allowlist ──────────────

function verifyDocList() {
  const docText = readFile(DOC_FILE);
  // Extract the bullet list under "## Current Fixture"
  const section = docText.match(/## Current Fixture\n\n([\s\S]*?)\n\n## /);
  if (!section) {
    fail(
      `Could not find "## Current Fixture" section in ${path.relative(ROOT, DOC_FILE)}`,
    );
    return;
  }
  const bullets = section[1].match(/`ctx\.(ui\.\w+|input\.\w+)`/g) || [];
  const docMethods = new Set(
    bullets.map((b) => b.replace(/`/g, '').replace('ctx.', '')),
  );

  for (const qualified of FIXTURE_REQUIRED) {
    if (!docMethods.has(qualified)) {
      fail(
        `"${qualified}" is in the FIXTURE_REQUIRED allowlist but missing from ${path.relative(ROOT, DOC_FILE)} § "Current Fixture". ` +
          `Add \`- \`ctx.${qualified}\`\` to the doc.`,
      );
    }
  }
  for (const docMethod of docMethods) {
    if (!FIXTURE_REQUIRED.has(docMethod)) {
      console.warn(
        `"${docMethod}" is listed in ${path.relative(ROOT, DOC_FILE)} § "Current Fixture" ` +
          `but not in the FIXTURE_REQUIRED allowlist. Consider removing it from the doc.`,
      );
    }
  }
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const dtsSource = readFile(API_DTS);
  const fixtureSource = readFile(FIXTURE_FILE);

  const uiMethods = extractUIMethods(dtsSource);
  const calls = collectFixtureCalls(fixtureSource);

  verifyAllowlistAgainstAPI(uiMethods);
  checkNewAPIMethods(uiMethods);
  checkFixtureCoverage(calls);
  verifyDocList();

  if (process.exitCode) {
    console.error('\nFixture coverage check failed. See errors above.');
    process.exit(1);
  }

  console.log('Extension fixture coverage checks passed');
  console.log(`  API methods tracked:   ${FIXTURE_REQUIRED.size}`);
  console.log(`  Fixture call sites:    ${calls.size}`);
}

main();
