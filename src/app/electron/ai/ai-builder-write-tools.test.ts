import assert from 'node:assert/strict';
import test from 'node:test';
import { createBuilderWriteTools } from './ai-builder-write-tools';
import type { BuilderAiEvent } from './ai-builder-tool-shared';
import type { PreviewAutoRun } from './ai-preview-auto-run';

const fakePi = { defineTool: (tool: unknown) => tool } as never;
const fakeType = {
  Object: () => ({}),
  String: () => ({}),
} as never;

const preview = {
  extensionId: 'dev.summarizer',
  rootItems: [],
  commands: [{ id: 'summarize' }],
  actions: [],
};

function writeExtensionTool(autoRun: PreviewAutoRun | undefined) {
  const events: BuilderAiEvent[] = [];
  const tools = createBuilderWriteTools(
    fakePi,
    fakeType,
    {
      extensionsDir: '/extensions',
      extensionApiPath: '/api.d.ts',
      getCapabilities: () => ({}),
      stageGeneratedExtension: async ({ filename }) => ({
        filePath: `/extensions/${filename}`,
        filename,
        preview,
        autoRun,
      }),
      validateGeneratedExtension: async () => {},
      safeExtensionPath: (root, filename) => `${root}/${filename}`,
      isExtensionSourceFile: () => true,
      recordRead: () => {},
      emitEvent: (event) => events.push(event),
    },
    new Set(),
  ) as unknown as Array<{
    name: string;
    execute: (id: string, params: unknown) => Promise<any>;
  }>;
  const tool = tools.find((item) => item.name === 'write_extension');
  assert.ok(tool);
  return {
    events,
    write: () =>
      tool.execute('call-1', { filename: 'summarizer.ts', code: 'export {}' }),
  };
}

const activationEvents = (events: BuilderAiEvent[]) =>
  events.filter((event) => event.type === 'extension_activated');

test('write_extension fails when the preview command failed, so the AI cannot claim success', async () => {
  const { events, write } = writeExtensionTool({
    result: { view: { id: 'extension-action-error' } },
    failureMessage: 'TypeError: text.trim is not a function',
  });

  await assert.rejects(write(), (error: Error) => {
    assert.match(error.message, /summarizer\.ts was installed/);
    assert.match(error.message, /does NOT work yet/);
    assert.match(error.message, /call write_extension again/);
    assert.match(error.message, /text\.trim is not a function/);
    return true;
  });
  const [activation] = activationEvents(events);
  assert.deepEqual(activation?.data, {
    filename: 'summarizer.ts',
    preview,
    autoRunResult: { view: { id: 'extension-action-error' } },
  });
});

test('write_extension reports a verified install when the preview command ran cleanly', async () => {
  const { events, write } = writeExtensionTool({
    result: { view: { id: 'summary' } },
  });

  const result = await write();

  assert.match(
    result.content[0].text,
    /^Installed and enabled summarizer\.ts\./,
  );
  assert.match(result.content[0].text, /ran in the preview without errors/);
  assert.deepEqual(activationEvents(events)[0]?.data, {
    filename: 'summarizer.ts',
    preview,
    autoRunResult: { view: { id: 'summary' } },
  });
});

test('write_extension does not claim verification when the preview command timed out', async () => {
  const { write } = writeExtensionTool({ result: {}, timedOut: true });

  const result = await write();

  assert.match(result.content[0].text, /not verified/);
  assert.doesNotMatch(result.content[0].text, /without errors/);
});

test('write_extension for an extension without commands installs without claiming a run', async () => {
  const { events, write } = writeExtensionTool(undefined);

  const result = await write();

  assert.equal(result.content[0].text, 'Installed and enabled summarizer.ts.');
  assert.equal(
    (activationEvents(events)[0]?.data as { autoRunResult?: unknown })
      .autoRunResult,
    undefined,
  );
});
