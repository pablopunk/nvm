import ts from 'typescript';

interface StaticExtensionManifest {
  id?: string;
  title?: string;
  capabilities: readonly string[];
  provenance: 'capabilities' | 'legacy-permissions' | 'undeclared';
  dynamic: boolean;
  idStart?: number;
  idEnd?: number;
}

function undeclaredManifest(): StaticExtensionManifest {
  return { capabilities: [], provenance: 'undeclared', dynamic: true };
}

function unwrap(expression: ts.Expression): ts.Expression {
  let value = expression;
  while (
    ts.isParenthesizedExpression(value) ||
    ts.isAsExpression(value) ||
    ts.isTypeAssertionExpression(value) ||
    ts.isSatisfiesExpression(value)
  ) {
    value = value.expression;
  }
  return value;
}

function literalString(node: ts.Expression | undefined) {
  return node && ts.isStringLiteral(node) ? node.text : undefined;
}

function literalStrings(node: ts.Expression | undefined) {
  if (!(node && ts.isArrayLiteralExpression(node))) {
    return;
  }
  const values: string[] = [];
  for (const element of node.elements) {
    if (!ts.isStringLiteral(element)) {
      return;
    }
    values.push(element.text);
  }
  return values;
}

function defaultExport(source: string) {
  const file = ts.createSourceFile(
    'extension.ts',
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  return file.statements.find(
    (statement): statement is ts.ExportAssignment =>
      ts.isExportAssignment(statement) && !statement.isExportEquals,
  );
}

function manifestProperties(expression: ts.ObjectLiteralExpression) {
  const values = new Map<string, ts.Expression>();
  let dynamic = false;
  for (const property of expression.properties) {
    if (
      !(ts.isPropertyAssignment(property) || ts.isMethodDeclaration(property))
    ) {
      continue;
    }
    if (
      !(ts.isIdentifier(property.name) || ts.isStringLiteral(property.name))
    ) {
      continue;
    }
    const name = property.name.text;
    if (!['id', 'title', 'capabilities', 'permissions'].includes(name)) {
      continue;
    }
    if (!ts.isPropertyAssignment(property)) {
      dynamic = true;
      continue;
    }
    if (values.has(name)) {
      dynamic = true;
    }
    values.set(name, property.initializer);
  }
  return { values, dynamic };
}

function declaredValues(values: Map<string, ts.Expression>, dynamic: boolean) {
  const hasCapabilities = values.has('capabilities');
  const capabilities = literalStrings(values.get('capabilities'));
  const permissions = literalStrings(values.get('permissions'));
  const invalidDeclaration =
    (hasCapabilities && !capabilities) ||
    (!hasCapabilities && values.has('permissions') && !permissions);
  let provenance: StaticExtensionManifest['provenance'] = 'undeclared';
  if (hasCapabilities) {
    provenance = 'capabilities';
  } else if (values.has('permissions')) {
    provenance = 'legacy-permissions';
  }
  return {
    capabilities: hasCapabilities ? capabilities || [] : permissions || [],
    provenance,
    dynamic: dynamic || invalidDeclaration,
  };
}

/**
 * Reads reviewable manifest metadata without evaluating user source. Dynamic,
 * duplicated, spread, or computed declarations become undeclared rather than
 * being guessed or imported.
 */
function inspectExtensionManifest(source: string): StaticExtensionManifest {
  const declaration = defaultExport(source);
  if (!declaration) {
    return undeclaredManifest();
  }
  const expression = unwrap(declaration.expression);
  if (!ts.isObjectLiteralExpression(expression)) {
    return undeclaredManifest();
  }
  const { values, dynamic } = manifestProperties(expression);
  const declared = declaredValues(values, dynamic);
  return {
    id: literalString(values.get('id')),
    title: literalString(values.get('title')),
    idStart: values.get('id')?.getStart(),
    idEnd: values.get('id')?.getEnd(),
    ...declared,
  };
}

export type { StaticExtensionManifest };
export { inspectExtensionManifest };
