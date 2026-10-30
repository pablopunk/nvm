import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  parseTypeScriptSource,
  SyntaxKind,
  typecheckTypeScriptFiles,
} from './typescript-api';

const compilerOptions = { target: 'ES2022', strict: true, noEmit: true };
const TYPE_MISMATCH = /TS2322/;

async function withTempFile(
  source: string,
  run: (filePath: string) => void,
): Promise<void> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ts-api-'));
  try {
    const filePath = path.join(dir, 'sample.ts');
    await fs.writeFile(filePath, source);
    run(filePath);
  } finally {
    await fs.rm(dir, { recursive: true, force: true });
  }
}

test('parses source text that never touches disk', () => {
  const source = 'export const answer = 42;\n';
  const sourceFile = parseTypeScriptSource('virtual.ts', source);
  const [statement] = sourceFile.statements;
  assert.equal(statement.kind, SyntaxKind.VariableStatement);
  assert.equal(
    source.slice(statement.pos, statement.end).trim(),
    'export const answer = 42;',
  );
});

test('parses repeated sources with the same file name independently', () => {
  const first = parseTypeScriptSource('same.ts', 'const a = 1;\n');
  const second = parseTypeScriptSource(
    'same.ts',
    'const b = 2;\nconst c = 3;\n',
  );
  assert.equal(first.statements.length, 1);
  assert.equal(second.statements.length, 2);
});

test('typechecks a clean file without errors', async () => {
  await withTempFile('export const value: number = 1;\n', (filePath) => {
    assert.deepEqual(typecheckTypeScriptFiles([filePath], compilerOptions), []);
  });
});

test('reports located errors for a file that fails typechecking', async () => {
  await withTempFile("export const value: number = 'text';\n", (filePath) => {
    const errors = typecheckTypeScriptFiles([filePath], compilerOptions);
    assert.equal(errors.length, 1);
    assert.match(errors[0], TYPE_MISMATCH);
    assert.ok(errors[0].includes('sample.ts(1,14)'), errors[0]);
  });
});

test('typechecks nothing when given no files', () => {
  assert.deepEqual(typecheckTypeScriptFiles([], compilerOptions), []);
});
