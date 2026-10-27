/**
 * Builds `src/generated/docs-data.json` from the desktop app's own sources so
 * the public docs can never drift and nobody maintains them by hand:
 *
 * - API reference: TSDoc in `src/app/resources/nevermind-extension-api.d.ts`
 * - Built-in features: extension manifests in `src/app/extensions/*.ts`
 *
 * It never fails the build. A section that cannot be generated is skipped with
 * a warning and the docs pages render without it.
 */
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import type {
  ApiEntry,
  ApiMember,
  DocsData,
  FeatureCommand,
  FeatureExtension,
} from '../src/lib/docs-data';

const backendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const appRoot = resolve(backendRoot, '../app');
const apiDeclarationPath = join(
  appRoot,
  'resources/nevermind-extension-api.d.ts',
);
const extensionsDir = join(appRoot, 'extensions');
const outputPath = join(backendRoot, 'src/generated/docs-data.json');

const MAX_INLINE_SOURCE_LINES = 60;
const MAX_SIGNATURE_LENGTH = 320;
const MAX_MEMBER_DEPTH = 3;
const EXTENSION_ID_PREFIX = 'nevermind.';
const FEATURE_MARKER_KEYS = [
  'commands',
  'capabilities',
  'rootItems',
  'searchItems',
  'actions',
];

function warn(message: string, error?: unknown) {
  const detail = error instanceof Error ? `: ${error.message}` : '';
  console.warn(`[docs] ${message}${detail}`);
}

function collapse(text: string) {
  const flat = text.replace(/\s+/g, ' ').trim();
  return flat.length > MAX_SIGNATURE_LENGTH
    ? `${flat.slice(0, MAX_SIGNATURE_LENGTH)}…`
    : flat;
}

// ── API reference ──────────────────────────────────────────────────────────

function readIntro(sourceText: string) {
  const block = sourceText.match(/\/\*\*[\s\S]*?\*\//)?.[0] ?? '';
  return block
    .replace(/^\/\*\*|\*\/$/g, '')
    .split('\n')
    .map((line) => line.replace(/^\s*\* ?/, ''))
    .join('\n')
    .trim()
    .replace(/^.*\n+/, '')
    .replace(/^@example$/m, '**Example**');
}

function unwrapType(node: ts.TypeNode): ts.TypeNode[] {
  if (ts.isParenthesizedTypeNode(node)) return unwrapType(node.type);
  if (ts.isIntersectionTypeNode(node)) return node.types.flatMap(unwrapType);
  return [node];
}

function memberElements(node: ts.Node): readonly ts.TypeElement[] {
  if (ts.isInterfaceDeclaration(node)) return node.members;
  if (ts.isTypeAliasDeclaration(node)) {
    return unwrapType(node.type).flatMap(function literalMembers(type) {
      return ts.isTypeLiteralNode(type) ? type.members : [];
    });
  }
  return [];
}

function nestedLiteral(type: ts.TypeNode | undefined) {
  if (!type) return undefined;
  return unwrapType(type).find(ts.isTypeLiteralNode);
}

function documentationOf(checker: ts.TypeChecker, node: ts.Node | undefined) {
  const symbol = node ? checker.getSymbolAtLocation(node) : undefined;
  return {
    doc: symbol
      ? ts.displayPartsToString(symbol.getDocumentationComment(checker))
      : '',
    tags: symbol
      ? symbol.getJsDocTags(checker).map(function toTag(tag) {
          return { name: tag.name, text: ts.displayPartsToString(tag.text) };
        })
      : [],
  };
}

function collectMembers(
  checker: ts.TypeChecker,
  elements: readonly ts.TypeElement[],
  prefix: string,
  depth: number,
): ApiMember[] {
  const members: ApiMember[] = [];
  for (const element of elements) {
    if (!ts.isPropertySignature(element) && !ts.isMethodSignature(element)) {
      continue;
    }
    const name = element.name.getText();
    const path = prefix ? `${prefix}.${name}` : name;
    const { doc, tags } = documentationOf(checker, element.name);
    const deprecated = tags.some((tag) => tag.name === 'deprecated');
    const literal = ts.isPropertySignature(element)
      ? nestedLiteral(element.type)
      : undefined;
    const signature = literal
      ? `${name}${element.questionToken ? '?' : ''}: {…}`
      : collapse(element.getText());
    members.push({ path, doc, signature, deprecated });
    if (literal && depth < MAX_MEMBER_DEPTH) {
      members.push(
        ...collectMembers(checker, literal.members, path, depth + 1),
      );
    }
  }
  return members;
}

function declarationSource(node: ts.Node) {
  const source = node.getText().replace(/^export\s+/, '');
  return source.split('\n').length <= MAX_INLINE_SOURCE_LINES
    ? source
    : undefined;
}

function buildApiEntries(program: ts.Program, sourceFile: ts.SourceFile) {
  const checker = program.getTypeChecker();
  const entries: ApiEntry[] = [];
  for (const statement of sourceFile.statements) {
    const exported = ts
      .getModifiers(statement as ts.HasModifiers)
      ?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword);
    if (
      !exported ||
      !(
        ts.isTypeAliasDeclaration(statement) ||
        ts.isInterfaceDeclaration(statement)
      )
    ) {
      continue;
    }
    const { doc, tags } = documentationOf(checker, statement.name);
    const members = collectMembers(checker, memberElements(statement), '', 0);
    entries.push({
      name: statement.name.text,
      doc,
      tags,
      deprecated: tags.some((tag) => tag.name === 'deprecated'),
      source: members.length ? undefined : declarationSource(statement),
      members,
    });
  }
  return entries;
}

function generateApi(): Pick<DocsData, 'intro' | 'api'> {
  try {
    const program = ts.createProgram([apiDeclarationPath], {
      noLib: true,
      noResolve: true,
      skipLibCheck: true,
      target: ts.ScriptTarget.ESNext,
    });
    const sourceFile = program.getSourceFile(apiDeclarationPath);
    if (!sourceFile) throw new Error('declaration file not found');
    return {
      intro: readIntro(readFileSync(apiDeclarationPath, 'utf8')),
      api: buildApiEntries(program, sourceFile),
    };
  } catch (error) {
    warn('Skipping extension API reference', error);
    return { intro: '', api: [] };
  }
}

// ── Built-in features ──────────────────────────────────────────────────────

function literalText(node: ts.Node | undefined) {
  return node &&
    (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
    ? node.text
    : undefined;
}

function propertyOf(literal: ts.ObjectLiteralExpression, key: string) {
  return literal.properties.find(function matches(property) {
    return (
      (ts.isPropertyAssignment(property) ||
        ts.isMethodDeclaration(property)) &&
      property.name.getText() === key
    );
  });
}

function stringProperty(literal: ts.ObjectLiteralExpression, key: string) {
  const property = propertyOf(literal, key);
  return property && ts.isPropertyAssignment(property)
    ? literalText(property.initializer)
    : undefined;
}

function unwrapExpression(node: ts.Expression): ts.Expression {
  if (ts.isAsExpression(node) || ts.isParenthesizedExpression(node)) {
    return unwrapExpression(node.expression);
  }
  return node;
}

function stringList(literal: ts.ObjectLiteralExpression, key: string) {
  const property = propertyOf(literal, key);
  if (!property || !ts.isPropertyAssignment(property)) return [];
  const value = unwrapExpression(property.initializer);
  return ts.isArrayLiteralExpression(value)
    ? value.elements.flatMap(function text(element) {
        return literalText(element) ?? [];
      })
    : [];
}

function arrayElements(literal: ts.ObjectLiteralExpression, key: string) {
  const property = propertyOf(literal, key);
  if (!property || !ts.isPropertyAssignment(property)) return [];
  const value = unwrapExpression(property.initializer);
  return ts.isArrayLiteralExpression(value) ? [...value.elements] : [];
}

function toCommand(literal: ts.ObjectLiteralExpression): FeatureCommand | null {
  const title = stringProperty(literal, 'title');
  if (!title) return null;
  return {
    title,
    subtitle: stringProperty(literal, 'subtitle'),
    aliases: stringList(literal, 'aliases'),
  };
}

/** Arrays returned straight from an `actions(ctx)` method, ignoring nested functions. */
function returnedActionLiterals(literal: ts.ObjectLiteralExpression) {
  const property = propertyOf(literal, 'actions');
  const found: ts.ObjectLiteralExpression[] = [];
  if (!property || !ts.isMethodDeclaration(property) || !property.body) {
    return found;
  }
  function visit(node: ts.Node) {
    if (ts.isFunctionLike(node) && node !== property) return;
    if (ts.isReturnStatement(node) && node.expression) {
      const value = unwrapExpression(node.expression);
      const array = ts.isObjectLiteralExpression(value)
        ? arrayElements(value, 'actions')
        : ts.isArrayLiteralExpression(value)
          ? [...value.elements]
          : [];
      for (const element of array) {
        if (ts.isObjectLiteralExpression(element)) found.push(element);
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(property.body);
  return found;
}

function toFeature(literal: ts.ObjectLiteralExpression): FeatureExtension | null {
  const id = stringProperty(literal, 'id');
  const title = stringProperty(literal, 'title');
  const isManifest = FEATURE_MARKER_KEYS.some((key) =>
    propertyOf(literal, key),
  );
  if (!id?.startsWith(EXTENSION_ID_PREFIX) || !title || !isManifest) {
    return null;
  }
  const commandLiterals = [
    ...arrayElements(literal, 'commands'),
    ...returnedActionLiterals(literal),
  ].filter(ts.isObjectLiteralExpression);
  const seen = new Set<string>();
  const commands = commandLiterals.flatMap(function unique(commandLiteral) {
    const command = toCommand(commandLiteral);
    if (!command || seen.has(command.title)) return [];
    seen.add(command.title);
    return [command];
  });
  return {
    id,
    title,
    description:
      stringProperty(literal, 'description') ??
      stringProperty(literal, 'subtitle'),
    capabilities: stringList(literal, 'capabilities'),
    commands,
  };
}

function featuresInFile(filePath: string) {
  const sourceFile = ts.createSourceFile(
    filePath,
    readFileSync(filePath, 'utf8'),
    ts.ScriptTarget.ESNext,
    true,
  );
  const features: FeatureExtension[] = [];
  function visit(node: ts.Node) {
    if (ts.isObjectLiteralExpression(node)) {
      const feature = toFeature(node);
      if (feature) features.push(feature);
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return features;
}

function isExtensionSource(fileName: string) {
  return (
    fileName.endsWith('.ts') &&
    !fileName.endsWith('.test.ts') &&
    !fileName.startsWith('_') &&
    !fileName.includes('data')
  );
}

function generateFeatures(): FeatureExtension[] {
  try {
    const byId = new Map<string, FeatureExtension>();
    for (const fileName of readdirSync(extensionsDir).filter(isExtensionSource)) {
      try {
        for (const feature of featuresInFile(join(extensionsDir, fileName))) {
          byId.set(feature.id, feature);
        }
      } catch (error) {
        warn(`Skipping features in ${fileName}`, error);
      }
    }
    return [...byId.values()]
      .filter((feature) => feature.description || feature.commands.length)
      .sort((a, b) => a.title.localeCompare(b.title));
  } catch (error) {
    warn('Skipping built-in features', error);
    return [];
  }
}

// ── Output ─────────────────────────────────────────────────────────────────

const data: DocsData = { ...generateApi(), features: generateFeatures() };
mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, `${JSON.stringify(data, null, 2)}\n`);
console.log(
  `[docs] ${data.api.length} API entries, ${data.features.length} built-in features`,
);
