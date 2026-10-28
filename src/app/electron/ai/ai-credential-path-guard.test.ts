import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import test from 'node:test';
import {
  CREDENTIAL_PATH_BLOCK_REASON,
  type CredentialPathPolicy,
  createCredentialPathGuardExtension,
  isCredentialPath,
  resolveToolPath,
} from './ai-credential-path-guard';

type Handler = (event: unknown, ctx: { cwd: string }) => unknown;

function createFixture() {
  const root = fs.realpathSync(
    fs.mkdtempSync(path.join(os.tmpdir(), 'nvm-credential-guard-')),
  );
  const home = path.join(root, 'home');
  const documents = path.join(home, 'Documents');
  fs.mkdirSync(path.join(home, '.ssh'), { recursive: true });
  fs.mkdirSync(documents, { recursive: true });
  fs.writeFileSync(path.join(home, '.ssh', 'id_ed25519'), 'PRIVATE KEY');
  fs.writeFileSync(path.join(documents, 'notes.md'), 'hello');
  return { root, home, documents };
}

function withFixture(run: (fixture: ReturnType<typeof createFixture>) => void) {
  const fixture = createFixture();
  try {
    run(fixture);
  } finally {
    fs.rmSync(fixture.root, { recursive: true, force: true });
  }
}

function installGuard(policy: CredentialPathPolicy) {
  const handlers = new Map<string, Handler>();
  const { factory } = createCredentialPathGuardExtension(policy);
  factory({
    on: (name: string, handler: Handler) => handlers.set(name, handler),
  } as never);
  return handlers;
}

function toolCallEvent(toolName: string, input: unknown) {
  return { type: 'tool_call', toolName, toolCallId: 'call-1', input };
}

function toolResultEvent(
  toolName: string,
  input: Record<string, unknown>,
  text: string,
) {
  return {
    type: 'tool_result',
    toolName,
    toolCallId: 'call-1',
    input,
    content: [{ type: 'text', text }],
    details: undefined,
    isError: false,
  };
}

function resolvedIsCredential(
  rawPath: string,
  { cwd, home }: { cwd: string; home: string },
) {
  return isCredentialPath(resolveToolPath(rawPath, { cwd, homeDir: home }), {
    homeDir: home,
  });
}

test('credential reads are denied in every spelling pi accepts', () => {
  withFixture(({ home, documents }) => {
    const keyPath = path.join(home, '.ssh', 'id_ed25519');
    const cases: Array<[string, string]> = [
      ['~/.ssh/id_ed25519', documents],
      ['@~/.ssh/id_ed25519', documents],
      [pathToFileURL(keyPath).href, documents],
      [keyPath, documents],
      ['../.ssh/id_ed25519', documents],
    ];
    for (const [rawPath, cwd] of cases)
      assert.equal(resolvedIsCredential(rawPath, { cwd, home }), true, rawPath);
  });
});

test('symlinks that resolve into ~/.ssh are denied', () => {
  withFixture(({ root, home }) => {
    const elsewhere = path.join(root, 'elsewhere');
    fs.mkdirSync(elsewhere);
    fs.symlinkSync(
      path.join(home, '.ssh'),
      path.join(elsewhere, 'keys'),
      'dir',
    );
    assert.equal(
      resolvedIsCredential('keys/id_ed25519', { cwd: elsewhere, home }),
      true,
    );
  });
});

test('credential directory names match case-insensitively only on darwin', () => {
  withFixture(({ home }) => {
    const upperCaseKey = path.join(home, '.SSH', 'id_ed25519');
    assert.equal(
      isCredentialPath(upperCaseKey, { homeDir: home, platform: 'darwin' }),
      true,
    );
    assert.equal(
      isCredentialPath(upperCaseKey, { homeDir: home, platform: 'linux' }),
      false,
    );
  });
});

test('env files are denied wherever they live', () => {
  withFixture(({ home, documents }) => {
    for (const name of ['.env', '.env.local', '.env.production'])
      assert.equal(
        resolvedIsCredential(name, { cwd: documents, home }),
        true,
        name,
      );
  });
});

test('ordinary notes stay readable', () => {
  withFixture(({ home, documents }) => {
    assert.equal(
      resolvedIsCredential('~/Documents/notes.md', { cwd: documents, home }),
      false,
    );
    assert.equal(
      resolvedIsCredential('notes.md', { cwd: documents, home }),
      false,
    );
  });
});

test('browser profiles and Nevermind userData are denied except the AI workspace', () => {
  withFixture(({ home }) => {
    const userData = path.join(
      home,
      'Library',
      'Application Support',
      'Nevermind',
    );
    const policy = {
      homeDir: home,
      extraDeniedPaths: [path.join(userData, 'pi-agent'), userData],
      allowedPaths: [path.join(userData, 'ai-workspace')],
    };
    const chromeCookies = path.join(
      home,
      'Library',
      'Application Support',
      'Google',
      'Chrome',
      'Default',
      'Cookies',
    );
    assert.equal(isCredentialPath(chromeCookies, policy), true);
    assert.equal(
      isCredentialPath(path.join(userData, 'pi-agent', 'auth.json'), policy),
      true,
    );
    assert.equal(
      isCredentialPath(path.join(userData, 'byo-key.json'), policy),
      true,
    );
    assert.equal(
      isCredentialPath(path.join(userData, 'ai-workspace', 'notes.md'), policy),
      false,
    );
  });
});

test('tool_call guard blocks credential reads and lets normal reads through', () => {
  withFixture(({ home, documents }) => {
    const toolCall = installGuard({ homeDir: home }).get('tool_call');
    assert.ok(toolCall);
    assert.deepEqual(
      toolCall(toolCallEvent('read', { path: '~/.ssh/id_ed25519' }), {
        cwd: documents,
      }),
      { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON },
    );
    assert.equal(
      toolCall(toolCallEvent('read', { path: 'notes.md' }), { cwd: documents }),
      undefined,
    );
  });
});

test('tool_call guard blocks grep, find and ls only when the search root is a credential location', () => {
  withFixture(({ home, documents }) => {
    const toolCall = installGuard({ homeDir: home }).get('tool_call');
    assert.ok(toolCall);
    const sshRoot = path.join(home, '.ssh');
    for (const toolName of ['grep', 'find', 'ls'])
      assert.deepEqual(
        toolCall(toolCallEvent(toolName, { path: sshRoot }), {
          cwd: documents,
        }),
        { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON },
      );
    assert.equal(
      toolCall(toolCallEvent('grep', { pattern: 'x', path: home }), {
        cwd: documents,
      }),
      undefined,
    );
  });
});

test('tool_result drops search lines that point into credential locations', () => {
  withFixture(({ home }) => {
    const toolResult = installGuard({ homeDir: home }).get('tool_result');
    assert.ok(toolResult);
    const grepText = [
      '.ssh/id_ed25519:1: PRIVATE KEY',
      'Documents/notes.md:1: hello',
      'Documents/notes.md-2- context',
      '.env:1: TOKEN=abc',
      '[100 matches limit reached]',
    ].join('\n');
    assert.deepEqual(
      toolResult(
        toolResultEvent('grep', { pattern: 'x', path: home }, grepText),
        { cwd: home },
      ),
      {
        content: [
          {
            type: 'text',
            text: 'Documents/notes.md:1: hello\nDocuments/notes.md-2- context\n[100 matches limit reached]',
          },
        ],
      },
    );
    assert.deepEqual(
      toolResult(
        toolResultEvent(
          'find',
          { path: home },
          '.ssh/\n.ssh/id_ed25519\nDocuments/',
        ),
        { cwd: home },
      ),
      { content: [{ type: 'text', text: 'Documents/' }] },
    );
    assert.deepEqual(
      toolResult(toolResultEvent('ls', { path: home }, '.ssh/\nDocuments/'), {
        cwd: home,
      }),
      { content: [{ type: 'text', text: 'Documents/' }] },
    );
    assert.equal(
      toolResult(
        toolResultEvent(
          'grep',
          { pattern: 'x', path: home },
          'Documents/notes.md:1: hello',
        ),
        { cwd: home },
      ),
      undefined,
    );
  });
});

test('guard handlers fail open and never throw on garbage input', () => {
  withFixture(({ home, documents }) => {
    const handlers = installGuard({ homeDir: home });
    const garbageEvents = [
      null,
      'read',
      { toolName: 'read', input: null },
      { toolName: 'read', input: { path: { nested: true } } },
      { toolName: 'grep', input: 42 },
    ];
    for (const handlerName of ['tool_call', 'tool_result'])
      for (const event of garbageEvents)
        assert.doesNotThrow(() =>
          assert.equal(
            handlers.get(handlerName)?.(event, { cwd: documents }),
            undefined,
          ),
        );
    assert.doesNotThrow(() =>
      createCredentialPathGuardExtension({ homeDir: 42 as never }).factory({
        on: () => undefined,
      } as never),
    );
  });
});
