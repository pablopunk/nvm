import assert from 'node:assert/strict';
import test from 'node:test';
import { createBuilderWriteTools } from './ai-builder-write-tools';
import type { BuilderToolOptions } from './ai-builder-tool-shared';
import type { PreviewRunOutcome } from './ai-preview-run-observer';

const fakePi = { defineTool: (tool: unknown) => tool } as never;
const fakeType = {
  Object: () => ({}),
  String: () => ({}),
} as never;

const previewWithCommand = {
  extensionId: 'dev.summarizer',
  rootItems: [],
  commands: [{ id: 'summarize' }],
  actions: [],
};

function builderTools(overrides: Partial<BuilderToolOptions> = {}) {
  const staged: string[] = [];
  const tools = createBuilderWriteTools(
    fakePi,
    fakeType,
    {
      extensionsDir: '/extensions',
      extensionApiPath: '/api.d.ts',
      getCapabilities: () => ({}),
      stageGeneratedExtension: async ({ filename }) => {
        staged.push(filename);
        return {
          filePath: `/extensions/${filename}`,
          filename,
          preview: previewWithCommand,
        };
      },
      validateGeneratedExtension: async () => {},
      safeExtensionPath: (root, filename) => `${root}/${filename}`,
      isExtensionSourceFile: () => true,
      recordRead: () => {},
      canWriteExtension: () => true,
      ...overrides,
    },
    new Set(),
  ) as unknown as Array<{
    name: string;
    execute: (id: string, params: unknown) => Promise<any>;
  }>;
  const tool = (name: string) => {
    const found = tools.find((item) => item.name === name);
    assert.ok(found, name);
    return found;
  };
  return {
    staged,
    write: () =>
      tool('write_extension').execute('call-1', {
        filename: 'summarizer.ts',
        code: 'export {}',
      }),
    readRun: () =>
      tool('read_preview_run').execute('call-2', {
        filename: 'summarizer.ts',
      }),
  };
}

const reader = (outcome: PreviewRunOutcome) => async () => outcome;

test('write_extension installs without running or verifying the extension', async () => {
  const { write, staged } = builderTools();

  const result = await write();

  assert.deepEqual(staged, ['summarizer.ts']);
  assert.match(
    result.content[0].text,
    /^Installed and enabled summarizer\.ts\./,
  );
  assert.match(result.content[0].text, /not been run or verified/);
  assert.match(result.content[0].text, /read_preview_run/);
  assert.doesNotMatch(result.content[0].text, /without errors/);
});

test('write_extension for an extension without commands has nothing to verify', async () => {
  const { write } = builderTools({
    stageGeneratedExtension: async ({ filename }) => ({
      filePath: `/extensions/${filename}`,
      filename,
      preview: { ...previewWithCommand, commands: [] },
    }),
  });

  const result = await write();

  assert.equal(result.content[0].text, 'Installed and enabled summarizer.ts.');
});

test('read_preview_run reports a clean run without claiming more than was checked', async () => {
  const { readRun } = builderTools({
    readPreviewRun: reader({ status: 'ran' }),
  });

  const result = await readRun();

  assert.match(result.content[0].text, /ran in the preview without errors/);
  assert.match(result.content[0].text, /not covered/);
  assert.equal(result.details.status, 'ran');
});

test('read_preview_run errors with the underlying failure so the AI repairs the extension', async () => {
  const { readRun } = builderTools({
    readPreviewRun: reader({
      status: 'failed',
      message: 'TypeError: text.trim is not a function',
    }),
  });

  await assert.rejects(readRun(), (error: Error) => {
    assert.match(error.message, /does NOT work yet/);
    assert.match(error.message, /call write_extension again/);
    assert.match(error.message, /text\.trim is not a function/);
    return true;
  });
});

test('read_preview_run reports an unverified install when the preview never ran the command', async () => {
  const { readRun } = builderTools({
    readPreviewRun: reader({ status: 'not-run' }),
  });

  const result = await readRun();

  assert.match(result.content[0].text, /NOT verified/);
  assert.match(result.content[0].text, /unverified/);
  assert.doesNotMatch(result.content[0].text, /without errors/);
});

test('read_preview_run without a host reader never claims verification', async () => {
  const { readRun } = builderTools();

  const result = await readRun();

  assert.equal(result.details.status, 'not-run');
});

test('read_preview_run refuses extensions this chat does not own', async () => {
  const { readRun } = builderTools({
    canWriteExtension: () => false,
    readPreviewRun: reader({ status: 'failed', message: 'secret' }),
  });

  await assert.rejects(readRun(), /does not own that extension/);
});
