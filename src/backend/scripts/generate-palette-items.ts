/**
 * Exposes the desktop app's built-in palette items to the landing page.
 *
 * Every item the app ranks in palette results carries a `score`, so scanning
 * `src/app/extensions` for scored items with a literal title yields the real
 * commands (title, subtitle, icon, aliases) without a hand-written list.
 * Writes `src/generated/palette-items.json`. It never fails the build: if the
 * scan breaks, the landing palette falls back to its sample content.
 */
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import type { PaletteItem } from '../src/components/landing-palette/palette-types';

const backendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const extensionsDir = resolve(backendRoot, '../app/extensions');
const outputPath = join(backendRoot, 'src/generated/palette-items.json');

/** Account items depend on sign-in state and include developer-only controls. */
const STATE_DEPENDENT_SOURCES = new Set(['account.ts']);

function literalText(node: ts.Node | undefined) {
  return node &&
    (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
    ? node.text
    : undefined;
}

function propertyOf(literal: ts.ObjectLiteralExpression, key: string) {
  return literal.properties.find(function matches(property) {
    return property.name?.getText() === key;
  });
}

function stringProperty(literal: ts.ObjectLiteralExpression, key: string) {
  const property = propertyOf(literal, key);
  return property && ts.isPropertyAssignment(property)
    ? literalText(property.initializer)
    : undefined;
}

function aliasesOf(literal: ts.ObjectLiteralExpression) {
  const property = propertyOf(literal, 'aliases');
  const value =
    property && ts.isPropertyAssignment(property)
      ? property.initializer
      : undefined;
  return value && ts.isArrayLiteralExpression(value)
    ? value.elements.flatMap((element) => literalText(element) ?? [])
    : [];
}

function scoredItemsIn(filePath: string) {
  const sourceFile = ts.createSourceFile(
    filePath,
    readFileSync(filePath, 'utf8'),
    ts.ScriptTarget.ESNext,
    true,
  );
  const items: PaletteItem[] = [];
  function visit(node: ts.Node) {
    if (ts.isObjectLiteralExpression(node)) {
      const title = stringProperty(node, 'title');
      if (title && propertyOf(node, 'score')) {
        items.push({
          id: `builtin:${title}`,
          title,
          subtitle: stringProperty(node, 'subtitle') ?? '',
          icon: stringProperty(node, 'icon'),
          aliases: aliasesOf(node),
          kind: 'command',
        });
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return items;
}

function isScannedSource(fileName: string) {
  return (
    fileName.endsWith('.ts') &&
    !fileName.endsWith('.test.ts') &&
    !fileName.startsWith('_') &&
    !fileName.includes('data') &&
    !STATE_DEPENDENT_SOURCES.has(fileName)
  );
}

function generate(): PaletteItem[] {
  const byId = new Map<string, PaletteItem>();
  try {
    for (const fileName of readdirSync(extensionsDir).filter(isScannedSource)) {
      for (const item of scoredItemsIn(join(extensionsDir, fileName))) {
        byId.set(item.id, item);
      }
    }
  } catch (error) {
    console.warn('[palette] Skipping built-in items:', error);
  }
  return [...byId.values()];
}

const items = generate();
mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, `${JSON.stringify(items, null, 2)}\n`);
console.log(`[palette] ${items.length} built-in palette items`);
