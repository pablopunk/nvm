#!/usr/bin/env node
const fs = require('node:fs');
const path = require('node:path');
const ts = require('./lib/parse-typescript.cjs');

// ── config ──────────────────────────────────────────────────────────────────

const ROOT = process.cwd();
const ELECTRON_DIR = path.join(ROOT, 'src', 'app', 'electron');
const ROOT_FOLDER = '(root)';
const ENTRY_FILES = new Set(['main.ts', 'preload.ts']);
const RESOLVABLE_SUFFIXES = [
  '',
  '.ts',
  '.tsx',
  '.d.ts',
  '/index.ts',
  '/index.tsx',
];

/** A folder may import its own files or files in a strictly lower layer. */
const FOLDER_LAYERS = {
  platform: 0,
  diagnostics: 1,
  backend: 2,
  os: 3,
  files: 4,
  search: 4,
  dictation: 4,
  clipboard: 5,
  state: 6,
  'extension-host': 7,
  ai: 8,
  shortcuts: 8,
  actions: 8,
  app: 9,
  [ROOT_FOLDER]: 10,
  ipc: 11,
};

const ELECTRON_SINGLETONS = new Set([
  'app',
  'ipcMain',
  'protocol',
  'nativeTheme',
  'screen',
  'powerMonitor',
  'globalShortcut',
  'session',
  'autoUpdater',
]);

// ── file discovery ──────────────────────────────────────────────────────────

function walkFiles(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(directory, entry.name);
    return entry.isDirectory() ? walkFiles(entryPath) : [entryPath];
  });
}

function isTypeScriptFile(file) {
  return /\.tsx?$/.test(file);
}

function isTestFile(file) {
  return /\.test\.tsx?$/.test(file);
}

function isEntryFile(file) {
  return (
    path.dirname(file) === ELECTRON_DIR && ENTRY_FILES.has(path.basename(file))
  );
}

function folderOf(file) {
  const [first, ...rest] = path.relative(ELECTRON_DIR, file).split(path.sep);
  return rest.length === 0 ? ROOT_FOLDER : first;
}

function relative(file) {
  return path.relative(ROOT, file);
}

function parseSourceFile(file) {
  const text = fs.readFileSync(file, 'utf8');
  return ts.parseTypeScriptSource(file, text);
}

function lineOf(sourceFile, node) {
  return (
    sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1
  );
}

// ── import graph ────────────────────────────────────────────────────────────

function stringLiteralText(node) {
  if (!node) return null;
  return ts.isStringLiteralLikeNode(node) ? node.text : null;
}

function isMockModuleCallee(callee) {
  if (!ts.isPropertyAccessExpression(callee) || callee.name.text !== 'module') {
    return false;
  }
  const target = callee.expression;
  return (
    (ts.isIdentifier(target) && target.text === 'mock') ||
    (ts.isPropertyAccessExpression(target) && target.name.text === 'mock')
  );
}

function isModuleLoadingCall(node) {
  const callee = node.expression;
  return (
    callee.kind === ts.SyntaxKind.ImportKeyword ||
    (ts.isIdentifier(callee) && callee.text === 'require') ||
    isMockModuleCallee(callee)
  );
}

function moduleSpecifierOf(node) {
  if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
    return stringLiteralText(node.moduleSpecifier);
  }
  if (
    ts.isImportEqualsDeclaration(node) &&
    ts.isExternalModuleReference(node.moduleReference)
  ) {
    return stringLiteralText(node.moduleReference.expression);
  }
  if (ts.isCallExpression(node) && isModuleLoadingCall(node)) {
    return stringLiteralText(node.arguments[0]);
  }
  if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)) {
    return stringLiteralText(node.argument.literal);
  }
  return null;
}

function collectModuleSpecifiers(sourceFile) {
  const found = [];
  function visit(node) {
    const specifier = moduleSpecifierOf(node);
    if (specifier !== null) {
      found.push({ specifier, line: lineOf(sourceFile, node) });
    }
    node.forEachChild(visit);
  }
  visit(sourceFile);
  return found;
}

function isInsideElectronDir(target) {
  const fromElectron = path.relative(ELECTRON_DIR, target);
  return !fromElectron.startsWith('..') && !path.isAbsolute(fromElectron);
}

function resolveRelativeImport(importer, specifier) {
  const base = path.resolve(path.dirname(importer), specifier);
  if (!isInsideElectronDir(base)) return { outside: true };
  const resolved = RESOLVABLE_SUFFIXES.map((suffix) => base + suffix).find(
    (candidate) => fs.existsSync(candidate) && fs.statSync(candidate).isFile(),
  );
  return resolved ? { resolved } : { unresolved: true };
}

function buildImportEdges(files, parsedFiles, violations) {
  const edges = [];
  for (const file of files) {
    for (const { specifier, line } of collectModuleSpecifiers(
      parsedFiles.get(file),
    )) {
      if (!specifier.startsWith('.')) continue;
      const result = resolveRelativeImport(file, specifier);
      if (result.outside) continue;
      if (result.unresolved) {
        violations.push(
          `[unresolved] ${relative(file)}:${line} imports "${specifier}", which does not resolve to a .ts/.tsx file inside src/app/electron`,
        );
        continue;
      }
      edges.push({ from: file, to: result.resolved, specifier, line });
    }
  }
  return edges;
}

// ── rule 1: folder-level import cycles ──────────────────────────────────────

function buildFolderGraph(edges) {
  const graph = new Map();
  for (const { from, to } of edges) {
    const fromFolder = folderOf(from);
    const toFolder = folderOf(to);
    if (!graph.has(fromFolder)) graph.set(fromFolder, new Set());
    if (!graph.has(toFolder)) graph.set(toFolder, new Set());
    if (fromFolder !== toFolder) graph.get(fromFolder).add(toFolder);
  }
  return graph;
}

function findStronglyConnectedComponents(graph) {
  let nextIndex = 0;
  const indexOf = new Map();
  const lowLinkOf = new Map();
  const stack = [];
  const onStack = new Set();
  const components = [];

  function connect(node) {
    indexOf.set(node, nextIndex);
    lowLinkOf.set(node, nextIndex);
    nextIndex += 1;
    stack.push(node);
    onStack.add(node);
    for (const next of graph.get(node)) {
      if (!indexOf.has(next)) {
        connect(next);
        lowLinkOf.set(node, Math.min(lowLinkOf.get(node), lowLinkOf.get(next)));
      } else if (onStack.has(next)) {
        lowLinkOf.set(node, Math.min(lowLinkOf.get(node), indexOf.get(next)));
      }
    }
    if (lowLinkOf.get(node) === indexOf.get(node)) {
      components.push(popComponent(node));
    }
  }

  function popComponent(root) {
    const component = [];
    let member;
    do {
      member = stack.pop();
      onStack.delete(member);
      component.push(member);
    } while (member !== root);
    return component;
  }

  for (const node of graph.keys()) {
    if (!indexOf.has(node)) connect(node);
  }
  return components;
}

function describeCycleEdges(cycleFolders, edges) {
  const members = new Set(cycleFolders);
  return edges
    .filter(({ from, to }) => {
      const fromFolder = folderOf(from);
      const toFolder = folderOf(to);
      return (
        fromFolder !== toFolder &&
        members.has(fromFolder) &&
        members.has(toFolder)
      );
    })
    .map(
      ({ from, to, line }) =>
        `      ${relative(from)}:${line} -> ${relative(to)}`,
    );
}

function checkFolderCycles(edges) {
  return findStronglyConnectedComponents(buildFolderGraph(edges))
    .filter((component) => component.length > 1)
    .map((component) => {
      const folders = component.sort();
      return [
        `[cycle] folders import each other: ${folders.join(' <-> ')}`,
        ...describeCycleEdges(folders, edges),
      ].join('\n');
    });
}

// ── rule 2: layer order ─────────────────────────────────────────────────────

function describeLayerViolation({ from, to, specifier, line }) {
  const fromFolder = folderOf(from);
  const toFolder = folderOf(to);
  const fromLayer = FOLDER_LAYERS[fromFolder];
  const toLayer = FOLDER_LAYERS[toFolder];
  if (fromLayer === undefined || toLayer === undefined) {
    const unknown = fromLayer === undefined ? fromFolder : toFolder;
    return `[layer] ${relative(from)}:${line} imports "${specifier}", but folder "${unknown}" has no layer in FOLDER_LAYERS`;
  }
  if (toLayer < fromLayer) return null;
  return `[layer] ${relative(from)}:${line} imports "${specifier}" (${fromFolder} layer ${fromLayer} -> ${toFolder} layer ${toLayer}); a file may import only its own folder or a strictly lower layer`;
}

function checkLayerOrder(edges) {
  return edges
    .filter(
      ({ from, to }) => !isEntryFile(from) && folderOf(from) !== folderOf(to),
    )
    .map(describeLayerViolation)
    .filter(Boolean);
}

// ── rules 3-5: module-scope hygiene ─────────────────────────────────────────

function isFunctionOrClass(node) {
  return ts.isFunctionLikeDeclaration(node) || ts.isClassLikeDeclaration(node);
}

function isRuntimeFreeNode(node) {
  return (
    ts.isTypeNode(node) ||
    ts.isInterfaceDeclaration(node) ||
    ts.isTypeAliasDeclaration(node) ||
    ts.isImportDeclaration(node) ||
    (ts.isExportDeclaration(node) && node.moduleSpecifier !== undefined)
  );
}

function isPropertyNameRatherThanReference(identifier) {
  const parent = identifier.parent;
  return (
    (ts.isPropertyAccessExpression(parent) && parent.name === identifier) ||
    (ts.isPropertyAssignment(parent) && parent.name === identifier) ||
    (ts.isBindingElement(parent) && parent.propertyName === identifier)
  );
}

function findModuleScopeSingletonUses(sourceFile) {
  const uses = [];
  function visit(node) {
    if (isFunctionOrClass(node) || isRuntimeFreeNode(node)) return;
    if (
      ts.isIdentifier(node) &&
      ELECTRON_SINGLETONS.has(node.text) &&
      !isPropertyNameRatherThanReference(node)
    ) {
      uses.push({ name: node.text, line: lineOf(sourceFile, node) });
    }
    node.forEachChild(visit);
  }
  visit(sourceFile);
  return uses;
}

function checkTopLevelStatements(file, sourceFile) {
  return sourceFile.statements
    .filter(ts.isExpressionStatement)
    .map(
      (statement) =>
        `[top-level-statement] ${relative(file)}:${lineOf(sourceFile, statement)} runs a statement at import time; wrap it in an exported function and call it from main.ts`,
    );
}

function checkSingletonUses(file, sourceFile) {
  return findModuleScopeSingletonUses(sourceFile).map(
    ({ name, line }) =>
      `[module-scope-singleton] ${relative(file)}:${line} uses Electron "${name}" outside a function; read it inside a function so import order cannot matter`,
  );
}

function declaredNames(bindingName) {
  if (ts.isIdentifier(bindingName)) return [bindingName.text];
  return bindingName.elements.flatMap((element) =>
    ts.isOmittedExpression(element) ? [] : declaredNames(element.name),
  );
}

function checkModuleScopeRequire(file, sourceFile) {
  return sourceFile.statements
    .filter(ts.isVariableStatement)
    .flatMap((statement) => statement.declarationList.declarations)
    .filter((declaration) =>
      declaredNames(declaration.name).includes('require'),
    )
    .map(
      (declaration) =>
        `[module-scope-require] ${relative(file)}:${lineOf(sourceFile, declaration)} declares "require" at module scope, which shadows the bundler's require; declare it inside a function`,
    );
}

function checkModuleScopeHygiene(file, sourceFile) {
  if (isTestFile(file) || isEntryFile(file)) return [];
  return [
    ...checkTopLevelStatements(file, sourceFile),
    ...checkSingletonUses(file, sourceFile),
    ...checkModuleScopeRequire(file, sourceFile),
  ];
}

// ── main ────────────────────────────────────────────────────────────────────

function main() {
  const files = walkFiles(ELECTRON_DIR).filter(isTypeScriptFile).sort();
  const parsedFiles = new Map(
    files.map((file) => [file, parseSourceFile(file)]),
  );
  const violations = [];
  const edges = buildImportEdges(files, parsedFiles, violations);

  violations.push(...checkFolderCycles(edges));
  violations.push(...checkLayerOrder(edges));
  for (const file of files) {
    violations.push(...checkModuleScopeHygiene(file, parsedFiles.get(file)));
  }

  if (violations.length > 0) {
    console.error(`Electron structure check failed (${violations.length}):`);
    for (const violation of violations) console.error(`  ${violation}`);
    process.exit(1);
  }

  const folderCount = new Set(files.map(folderOf)).size;
  console.log(
    `Electron structure checks passed (${files.length} files, ${folderCount} folders, ${edges.length} imports)`,
  );
}

main();
