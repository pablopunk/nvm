import assert from 'node:assert/strict';
import test from 'node:test';
import { aiChatPreviewFiles, prepareAiChatPreview } from './ai-chat-previews';

test('aiChatPreviewFiles restores owned existing files when saved previews are absent', () => {
  assert.deepEqual(
    aiChatPreviewFiles(
      {
        touchedExtensionFiles: ['older.ts', 'missing.ts'],
        generatedExtensionFile: 'latest.ts',
        contextExtensionFile: 'older.ts',
      },
      ['older.ts', 'latest.ts'],
    ),
    {
      files: ['older.ts', 'latest.ts'],
      selectedBuilderPreviewFilename: 'older.ts',
    },
  );
});

test('aiChatPreviewFiles ignores stale previews and selects the latest owned file', () => {
  assert.deepEqual(
    aiChatPreviewFiles(
      {
        builderPreviewFiles: ['missing.ts', 'unowned.ts'],
        touchedExtensionFiles: ['older.ts', 'latest.ts'],
        generatedExtensionFile: 'latest.ts',
      },
      ['older.ts', 'latest.ts', 'unowned.ts'],
    ),
    {
      files: ['older.ts', 'latest.ts'],
      selectedBuilderPreviewFilename: 'latest.ts',
    },
  );
});

test('prepareAiChatPreview refreshes execution handles for restored actions', () => {
  let executionId = 0;
  const preview = {
    filename: 'live.ts',
    preview: {
      extensionId: 'live',
      rootItems: [{ id: 'root' }],
      actions: [{ id: 'action' }],
    },
  };

  const first = prepareAiChatPreview(preview, (action) => ({
    ...(action as object),
    executionId: `execution-${++executionId}`,
  }));
  const second = prepareAiChatPreview(preview, (action) => ({
    ...(action as object),
    executionId: `execution-${++executionId}`,
  }));

  assert.notEqual(
    (first.preview.rootItems[0] as unknown as { executionId: string })
      .executionId,
    (second.preview.rootItems[0] as unknown as { executionId: string })
      .executionId,
  );
  assert.notEqual(
    (first.preview.actions[0] as unknown as { executionId: string })
      .executionId,
    (second.preview.actions[0] as unknown as { executionId: string })
      .executionId,
  );
});

test('prepareAiChatPreview removes persisted nested execution handles before rehydrating', () => {
  const preview = {
    filename: 'live.ts',
    preview: {
      extensionId: 'live',
      rootItems: [
        {
          id: 'root',
          executionId: 'stale-root',
          primaryAction: { executionId: 'stale-primary' },
        },
      ],
      actions: [],
    },
  };
  const restored = prepareAiChatPreview(preview, (action) => action) as any;

  assert.equal(restored.preview.rootItems[0].executionId, undefined);
  assert.equal(
    restored.preview.rootItems[0].primaryAction.executionId,
    undefined,
  );
});

test('aiChatPreviewFiles retains a valid saved preview selection', () => {
  assert.deepEqual(
    aiChatPreviewFiles(
      {
        builderPreviewFiles: ['older.ts', 'latest.ts'],
        selectedBuilderPreviewFilename: 'older.ts',
        touchedExtensionFiles: ['older.ts', 'latest.ts'],
      },
      ['older.ts', 'latest.ts'],
    ),
    {
      files: ['older.ts', 'latest.ts'],
      selectedBuilderPreviewFilename: 'older.ts',
    },
  );
});
