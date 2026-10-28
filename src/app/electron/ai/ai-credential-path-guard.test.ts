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

async function withFixture(
  run: (fixture: ReturnType<typeof createFixture>) => void | Promise<void>,
) {
  const fixture = createFixture();
  try {
    await run(fixture);
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

async function resolvedIsCredential(
  rawPath: string,
  { cwd, home }: { cwd: string; home: string },
) {
  return isCredentialPath(
    await resolveToolPath(rawPath, { cwd, homeDir: home }),
    { homeDir: home },
  );
}

test('credential reads are denied in every spelling pi accepts', async () => {
  await withFixture(async ({ home, documents }) => {
    const keyPath = path.join(home, '.ssh', 'id_ed25519');
    const cases: Array<[string, string]> = [
      ['~/.ssh/id_ed25519', documents],
      ['@~/.ssh/id_ed25519', documents],
      [pathToFileURL(keyPath).href, documents],
      [keyPath, documents],
      ['../.ssh/id_ed25519', documents],
    ];
    for (const [rawPath, cwd] of cases)
      assert.equal(
        await resolvedIsCredential(rawPath, { cwd, home }),
        true,
        rawPath,
      );
  });
});

test('symlinks that resolve into ~/.ssh are denied', async () => {
  await withFixture(async ({ root, home }) => {
    const elsewhere = path.join(root, 'elsewhere');
    fs.mkdirSync(elsewhere);
    fs.symlinkSync(
      path.join(home, '.ssh'),
      path.join(elsewhere, 'keys'),
      'dir',
    );
    assert.equal(
      await resolvedIsCredential('keys/id_ed25519', { cwd: elsewhere, home }),
      true,
    );
  });
});

test('credential directory names match case-insensitively', async () => {
  await withFixture(({ home }) => {
    const upperCaseKey = path.join(home, '.SSH', 'id_ed25519');
    assert.equal(isCredentialPath(upperCaseKey, { homeDir: home }), true);
  });
});

test('env files are denied wherever they live', async () => {
  await withFixture(async ({ home, documents }) => {
    for (const name of ['.env', '.env.local', '.env.production'])
      assert.equal(
        await resolvedIsCredential(name, { cwd: documents, home }),
        true,
        name,
      );
  });
});

test('env template files stay readable', async () => {
  await withFixture(async ({ home, documents }) => {
    for (const name of ['.env.example', '.env.sample', '.env.template'])
      assert.equal(
        await resolvedIsCredential(name, { cwd: documents, home }),
        false,
        name,
      );
  });
});

test('ordinary notes stay readable', async () => {
  await withFixture(async ({ home, documents }) => {
    assert.equal(
      await resolvedIsCredential('~/Documents/notes.md', {
        cwd: documents,
        home,
      }),
      false,
    );
    assert.equal(
      await resolvedIsCredential('notes.md', { cwd: documents, home }),
      false,
    );
  });
});

test('browser profiles and Nevermind userData are denied except the AI workspace', async () => {
  await withFixture(({ home }) => {
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

test('tool_call guard blocks credential reads and lets normal reads through', async () => {
  await withFixture(async ({ home, documents }) => {
    const toolCall = installGuard({ homeDir: home }).get('tool_call');
    assert.ok(toolCall);
    assert.deepEqual(
      await toolCall(toolCallEvent('read', { path: '~/.ssh/id_ed25519' }), {
        cwd: documents,
      }),
      { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON },
    );
    assert.equal(
      await toolCall(toolCallEvent('read', { path: 'notes.md' }), {
        cwd: documents,
      }),
      undefined,
    );
  });
});

test('tool_call guard blocks grep, find and ls only when the search root is a credential location', async () => {
  await withFixture(async ({ home, documents }) => {
    const toolCall = installGuard({ homeDir: home }).get('tool_call');
    assert.ok(toolCall);
    const sshRoot = path.join(home, '.ssh');
    for (const toolName of ['grep', 'find', 'ls'])
      assert.deepEqual(
        await toolCall(toolCallEvent(toolName, { path: sshRoot }), {
          cwd: documents,
        }),
        { block: true, reason: CREDENTIAL_PATH_BLOCK_REASON },
      );
    assert.equal(
      await toolCall(toolCallEvent('grep', { pattern: 'x', path: home }), {
        cwd: documents,
      }),
      undefined,
    );
  });
});

test('tool_result drops search lines that point into credential locations', async () => {
  await withFixture(async ({ home }) => {
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
      await toolResult(
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
      await toolResult(
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
      await toolResult(
        toolResultEvent('ls', { path: home }, '.ssh/\nDocuments/'),
        { cwd: home },
      ),
      { content: [{ type: 'text', text: 'Documents/' }] },
    );
    assert.equal(
      await toolResult(
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

test('guard handlers fail open and never throw on garbage input', async () => {
  await withFixture(async ({ home, documents }) => {
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
        assert.equal(
          await handlers.get(handlerName)?.(event, { cwd: documents }),
          undefined,
        );
    assert.doesNotThrow(() =>
      createCredentialPathGuardExtension({ homeDir: 42 as never }).factory({
        on: () => undefined,
      } as never),
    );
  });
});
