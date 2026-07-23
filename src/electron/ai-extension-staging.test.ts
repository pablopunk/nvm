import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { getPath: () => os.tmpdir() },
    safeStorage: {
      isEncryptionAvailable: () => false,
      encryptString: (value: string) => Buffer.from(value),
      decryptString: (value: Buffer) => value.toString(),
    },
  },
});

const { stageAiGeneratedExtension } = await import('./ai');
const repoRoot =
  path.basename(process.cwd()) === 'backend'
    ? path.dirname(process.cwd())
    : process.cwd();

const TYPECHECK_FAILURE = /TypeScript validation failed/;
const ACTIVATION_FAILURE = /activation failed/;

const validSource = `import type { NevermindExtension } from './nevermind-extension-api'

export default {
  id: 'test.ai-live',
  title: 'AI Live',
  capabilities: [],
  commands: [{ id: 'open', title: 'AI Live', run: () => undefined }],
} satisfies NevermindExtension
`;

function options(root: string) {
  return {
    extensionsDir: path.join(root, 'extensions'),
    extensionTypesPath: path.join(
      repoRoot,
      'src/resources/nevermind-extension-api.d.ts',
    ),
    canWriteExtension: () => true,
  };
}

test('write_extension validates then activates the staged source', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ai-live-'));
  const received: Array<{ filename: string; source: string }> = [];
  try {
    const result = await stageAiGeneratedExtension(
      {
        ...options(root),
        activateGeneratedExtension: (filename, source) => {
          received.push({ filename, source });
          return Promise.resolve({
            filename,
            preview: {
              extensionId: 'test.ai-live',
              rootItems: [],
              actions: [],
            },
          });
        },
      },
      { filename: 'ai-live.ts', code: validSource },
    );

    assert.deepEqual(received, [
      { filename: 'ai-live.ts', source: validSource },
    ]);
    assert.equal(result.filename, 'ai-live.ts');
    assert.deepEqual(result.preview, {
      extensionId: 'test.ai-live',
      rootItems: [],
      actions: [],
    });
    await assert.rejects(
      fs.access(path.join(root, 'extension-drafts', 'ai-live.ts')),
    );
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('write_extension removes invalid staging and does not activate', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ai-invalid-'));
  let activated = false;
  try {
    await assert.rejects(
      stageAiGeneratedExtension(
        {
          ...options(root),
          activateGeneratedExtension: () => {
            activated = true;
            return Promise.reject(new Error('unexpected activation'));
          },
        },
        { filename: 'invalid.ts', code: 'export default {' },
      ),
      TYPECHECK_FAILURE,
    );
    assert.equal(activated, false);
    await assert.rejects(
      fs.access(path.join(root, 'extension-drafts', 'invalid.ts')),
    );
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('write_extension removes staging when live activation fails', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ai-activate-'));
  try {
    await assert.rejects(
      stageAiGeneratedExtension(
        {
          ...options(root),
          activateGeneratedExtension: () =>
            Promise.reject(new Error('activation failed')),
        },
        { filename: 'failed.ts', code: validSource },
      ),
      ACTIVATION_FAILURE,
    );
    await assert.rejects(
      fs.access(path.join(root, 'extension-drafts', 'failed.ts')),
    );
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});
