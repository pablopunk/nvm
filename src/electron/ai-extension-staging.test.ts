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

const validSource = `import type { NevermindExtension } from './nevermind-extension-api'

export default {
  id: 'test.ai-staged',
  title: 'AI Staged',
  capabilities: [],
  commands: [{ id: 'open', title: 'AI Staged', run: () => undefined }],
} satisfies NevermindExtension
`;
const TYPECHECK_FAILURE = /TypeScript validation failed/;

test('write_extension path validates and persists a proposal without importing or enabling it', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ai-stage-'));
  const extensionsDir = path.join(root, 'extensions');
  const persistedDraft = path.join(root, 'managed-drafts', 'ai-staged.ts');
  let staged: { filename: string; source: string } | undefined;
  try {
    const result = await stageAiGeneratedExtension(
      {
        extensionsDir,
        extensionTypesPath: path.join(
          repoRoot,
          'src/resources/nevermind-extension-api.d.ts',
        ),
        canWriteExtension: () => true,
        stageExtensionProposal: async (filename, source) => {
          staged = { filename, source };
          await fs.mkdir(path.dirname(persistedDraft), { recursive: true });
          await fs.writeFile(persistedDraft, source);
          return { draftFile: persistedDraft };
        },
      },
      { filename: 'ai-staged.ts', code: validSource },
    );

    assert.equal(result.draftPath, persistedDraft);
    assert.deepEqual(staged, {
      filename: 'ai-staged.ts',
      source: validSource,
    });
    await assert.rejects(fs.stat(path.join(extensionsDir, 'ai-staged.ts')));
    assert.equal(await fs.readFile(persistedDraft, 'utf8'), validSource);
    await assert.rejects(
      fs.stat(path.join(root, 'extension-drafts', 'ai-staged.ts')),
    );
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('write_extension path removes an invalid draft before proposal persistence', async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'nvm-ai-stage-'));
  const extensionsDir = path.join(root, 'extensions');
  let proposalCalls = 0;
  try {
    await assert.rejects(
      stageAiGeneratedExtension(
        {
          extensionsDir,
          extensionTypesPath: path.join(
            repoRoot,
            'src/resources/nevermind-extension-api.d.ts',
          ),
          stageExtensionProposal: () => {
            proposalCalls += 1;
            return Promise.resolve({ draftFile: 'unused' });
          },
        },
        { filename: 'invalid.ts', code: 'export default {' },
      ),
      TYPECHECK_FAILURE,
    );
    assert.equal(proposalCalls, 0);
    await assert.rejects(
      fs.stat(path.join(root, 'extension-drafts', 'invalid.ts')),
    );
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});
