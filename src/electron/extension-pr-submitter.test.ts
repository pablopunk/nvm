import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import {
  createExtensionPrSubmitter,
  type ExtensionPrSubmitterDeps,
} from './extension-pr-submitter';

function createDeps(
  overrides: Partial<ExtensionPrSubmitterDeps> & {
    stubs?: Record<string, string>;
    authOk?: boolean;
    ghMissing?: boolean;
    currentUser?: string;
  } = {},
) {
  const calls: Array<{ command: string; args: string[] }> = [];
  const stubs = overrides.stubs || {};
  const authOk = overrides.authOk ?? true;
  const ghMissing = overrides.ghMissing ?? false;
  const currentUser = overrides.currentUser ?? 'testuser';

  const execFileText = async (
    command: string,
    args: string[] = [],
  ): Promise<string> => {
    const key = `${command} ${args.join(' ')}`;
    calls.push({ command, args });
    if (key.startsWith('gh --version')) {
      if (ghMissing)
        throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' });
      return 'gh version 2.60.0';
    }
    if (key.startsWith('gh auth status')) {
      if (!authOk) throw new Error('You are not logged into any GitHub hosts.');
      return '';
    }
    if (key.startsWith('gh api user ')) return `${currentUser}\n`;
    if (key.startsWith('gh api repos/')) {
      if (key.includes('/branches/main')) return 'abc123def456';
      if (key.includes('/contents/')) {
        if (key.includes('index.ts')) {
          const slug = 'my-extension';
          const factoryFunc = 'createMyExtensionExtension';
          return JSON.stringify({
            content: Buffer.from(
              `import { createSystemExtension } from './system';\nexport const INTERNAL_EXTENSION_FACTORIES = [\n  createSystemExtension,\n];\n`,
            ).toString('base64'),
            sha: 'index-sha-001',
          });
        }
        return '';
      }
      if (key.includes('/git/refs')) return '';
      if (key.includes('/contents/')) return '';
      return JSON.stringify({ sha: 'test-sha' });
    }
    if (key.startsWith('gh repo fork')) return '';
    if (key.startsWith('gh pr create'))
      return 'https://github.com/pablopunk/nvm/pull/123\n';
    return stubs[key] || '';
  };

  const logCalls: Array<{ message: string; data?: unknown }> = [];
  const deps: ExtensionPrSubmitterDeps = {
    execFileText,
    extensionsDir: '/tmp/test-extensions',
    repoOwner: 'pablopunk',
    repoName: 'nvm',
    logInfo: (message, data) => logCalls.push({ message, data }),
    logWarn: (message, data) => logCalls.push({ message, data }),
  };

  return {
    deps,
    calls,
    logCalls,
  };
}

test('probeGh returns installed=false when gh is missing', async () => {
  const { deps } = createDeps({ ghMissing: true });
  const submitter = createExtensionPrSubmitter(deps);
  const result = await submitter.probe();
  assert.deepStrictEqual(result, { installed: false, authed: false });
});

test('probeGh returns installed=true, authed=false when gh not signed in', async () => {
  const { deps } = createDeps({ authOk: false });
  const submitter = createExtensionPrSubmitter(deps);
  const result = await submitter.probe();
  assert.deepStrictEqual(result, { installed: true, authed: false });
});

test('probeGh returns installed=true, authed=true when gh is ready', async () => {
  const { deps } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);
  const result = await submitter.probe();
  assert.deepStrictEqual(result, { installed: true, authed: true });
});

test('probeGh caches results (second call does not re-exec)', async () => {
  const { deps, calls } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);
  const first = await submitter.probe();
  assert.strictEqual(first.installed, true);
  assert.strictEqual(first.authed, true);
  const versionCalls = calls.filter((c) => c.args.includes('--version'));
  const firstCount = versionCalls.length;
  const second = await submitter.probe();
  assert.strictEqual(second.installed, true);
  assert.strictEqual(second.authed, true);
  assert.strictEqual(
    calls.filter((c) => c.args.includes('--version')).length,
    firstCount,
  );
});

test('submitExtensionPr rejects non-generated actions', async () => {
  const { deps, calls } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);
  const result = await submitter.submitExtensionPr({
    targetAction: { kind: 'builtin', title: 'Not generated' },
  });
  assert.strictEqual(result.ok, false);
  assert.ok(
    result.message.toLowerCase().includes('generated'),
    'should mention generated',
  );
  assert.strictEqual(calls.length, 0, 'should not invoke gh');
});

test('submitExtensionPr rejects when extensionFile is missing', async () => {
  const { deps } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);
  const result = await submitter.submitExtensionPr({
    targetAction: {
      kind: 'extension-root-item',
      removable: true,
      title: 'No File',
    },
  });
  assert.strictEqual(result.ok, false);
});

test('submitExtensionPr rejects path traversal outside extensions dir', async () => {
  const { deps, calls } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);

  const result = await submitter.submitExtensionPr({
    targetAction: {
      kind: 'extension-root-item',
      removable: true,
      extensionFile: '../secret.ts',
      title: 'Escape Attempt',
    },
  });
  assert.strictEqual(result.ok, false);
  assert.strictEqual(
    calls.length,
    0,
    'should not invoke any gh commands for path traversal',
  );
});

test('submitExtensionPr preflight fails when gh not authed', async () => {
  const { deps } = createDeps({ authOk: false });
  const submitter = createExtensionPrSubmitter(deps);

  const tmpDir = path.join('/tmp/test-extensions');
  await fs.mkdir(tmpDir, { recursive: true });
  const extFile = path.join(tmpDir, 'my-extension.ts');
  await fs.writeFile(
    extFile,
    `export default { id: 'my-extension', title: 'My Extension' }`,
  );

  try {
    const result = await submitter.submitExtensionPr({
      targetAction: {
        kind: 'extension-root-item',
        removable: true,
        extensionFile: 'my-extension.ts',
        title: 'My Extension',
      },
    });
    assert.strictEqual(result.ok, false);
    assert.ok(
      result.message.toLowerCase().includes('gh auth login'),
      'should mention gh auth login',
    );
  } finally {
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
});

test('submitExtensionPr happy path invokes correct gh commands', async () => {
  const { deps, calls } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);

  const tmpDir = path.join('/tmp/test-extensions');
  await fs.mkdir(tmpDir, { recursive: true });
  const extFile = path.join(tmpDir, 'my-extension.ts');
  await fs.writeFile(
    extFile,
    `export default { id: 'my-extension', title: 'My Extension' }`,
  );

  try {
    const result = await submitter.submitExtensionPr({
      targetAction: {
        kind: 'extension-root-item',
        removable: true,
        extensionFile: 'my-extension.ts',
        title: 'My Extension',
      },
    });
    assert.strictEqual(result.ok, true);
    assert.ok(result.prUrl, 'should return a PR URL');

    const forkCall = calls.find(
      ({ command, args }) =>
        command === 'gh' &&
        args[0] === 'repo' &&
        args[1] === 'fork',
    );
    assert.deepStrictEqual(forkCall, {
      command: 'gh',
      args: ['repo', 'fork', 'pablopunk/nvm'],
    });

    const joinedCalls = calls.map((c) => `${c.command} ${c.args.join(' ')}`);
    assert.ok(
      joinedCalls.some((c) => c.includes('branches/main')),
      'should fetch main sha',
    );
    assert.ok(
      joinedCalls.some(
        (c) =>
          c.includes('git/refs') && c.includes('submit-extension-my-extension'),
      ),
      'should create branch ref',
    );
    assert.ok(
      joinedCalls.some((c) =>
        c.includes('/contents/src/electron/extensions/my-extension.ts'),
      ),
      'should PUT extension file',
    );
    assert.ok(
      joinedCalls.some((c) =>
        c.includes('/contents/src/electron/extensions/index.ts'),
      ),
      'should PUT index.ts barrel',
    );
    assert.ok(
      joinedCalls.some((c) => c.startsWith('gh pr create')),
      'should create PR',
    );
  } finally {
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
});

test('submitExtensionPr uses the upstream directly for its owner', async () => {
  const { deps, calls } = createDeps({ currentUser: 'pablopunk' });
  const submitter = createExtensionPrSubmitter(deps);
  const tmpDir = path.join('/tmp/test-extensions');
  await fs.mkdir(tmpDir, { recursive: true });
  await fs.writeFile(
    path.join(tmpDir, 'my-extension.ts'),
    `export default { id: 'my-extension', title: 'My Extension' }`,
  );

  try {
    const result = await submitter.submitExtensionPr({
      targetAction: {
        kind: 'extension-root-item',
        removable: true,
        extensionFile: 'my-extension.ts',
        title: 'My Extension',
      },
    });
    assert.strictEqual(result.ok, true);
    assert.ok(
      !calls.some(({ args }) => args[0] === 'repo' && args[1] === 'fork'),
      'repository owner must not attempt to fork the upstream',
    );
    assert.ok(
      calls.some(({ args }) =>
        args.includes('repos/pablopunk/nvm/git/refs'),
      ),
      'repository owner must create the branch on the upstream',
    );
    const prCreateCall = calls.find(
      ({ args }) => args[0] === 'pr' && args[1] === 'create',
    );
    assert.ok(prCreateCall);
    assert.strictEqual(
      prCreateCall.args[prCreateCall.args.indexOf('--head') + 1],
      'submit-extension-my-extension',
    );
  } finally {
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
});

test('clone-safe payload: action with run fields is not cloneable', () => {
  const run = () => {};
  const action = {
    type: 'submitExtensionPr',
    title: 'Submit as PR',
    targetAction: {
      kind: 'extension-root-item',
      removable: true,
      extensionFile: 'my-extension.ts',
      title: 'Test',
      run,
    },
  };
  assert.throws(
    () => structuredClone(action),
    /could not be cloned/,
    'action with closure should fail structuredClone',
  );

  const cloned = JSON.parse(JSON.stringify(action));
  assert.strictEqual(cloned.type, 'submitExtensionPr');
  assert.strictEqual(cloned.targetAction.removable, true);
  assert.strictEqual(cloned.targetAction.run, undefined);
});

test('no auth token appears in gh argv', async () => {
  const { deps, calls } = createDeps({ authOk: true });
  const submitter = createExtensionPrSubmitter(deps);

  const tmpDir = path.join('/tmp/test-extensions');
  await fs.mkdir(tmpDir, { recursive: true });
  const extFile = path.join(tmpDir, 'my-extension.ts');
  await fs.writeFile(
    extFile,
    `export default { id: 'my-extension', title: 'My Extension' }`,
  );

  try {
    await submitter.submitExtensionPr({
      targetAction: {
        kind: 'extension-root-item',
        removable: true,
        extensionFile: 'my-extension.ts',
        title: 'My Extension',
      },
    });

    for (const call of calls) {
      const argv = [call.command, ...call.args].join(' ');
      assert.ok(
        !/(ghp_|gho_|github_pat_)/.test(argv),
        `token-like string in argv: ${argv.slice(0, 100)}`,
      );
    }
  } finally {
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
});
