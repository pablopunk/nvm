import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  applyBuilderPreviewActionResult,
  builderPreviewAutoRunAction,
  builderPreviewResultIsCurrent,
  builderPreviewRootActions,
  builderPreviewSelectedItemId,
  builderPreviewShouldAutoRun,
  hydrateBuilderPreviewViewById,
  patchBuilderPreviewState,
  patchBuilderPreviewViewById,
  resetBuilderPreviewState,
  retryBuilderPreviewHydration,
  upsertBuilderPreview,
} from './builder-preview';
import type { BuilderPreview, CommandView } from './model';

test('builder preview does not auto-run standalone actions', () => {
  const preview: BuilderPreview = {
    filename: 'quit.ts',
    preview: {
      extensionId: 'dev.quit',
      rootItems: [],
      commands: [],
      actions: [{ id: 'quit' }],
    },
  };
  assert.equal(builderPreviewShouldAutoRun(preview), false);
  assert.equal(builderPreviewAutoRunAction(preview), undefined);
});

test('builder preview defaults to the first command when other contributions exist', () => {
  const preview: BuilderPreview = {
    filename: 'timer.ts',
    preview: {
      extensionId: 'dev.timer',
      rootItems: [{ id: 'status' }],
      commands: [{ id: 'start' }],
      actions: [{ id: 'stop' }],
    },
  };
  assert.equal(builderPreviewShouldAutoRun(preview), true);
  assert.deepEqual(builderPreviewAutoRunAction(preview), { id: 'start' });
  assert.equal(builderPreviewRootActions(preview).length, 3);
});

test('builder preview ignores action results from an obsolete preview version', () => {
  assert.equal(builderPreviewResultIsCurrent(2, 2), true);
  assert.equal(builderPreviewResultIsCurrent(1, 2), false);
});

test('builder preview inserts before auto-run state updates', () => {
  const preview = {
    filename: 'timer.ts',
    view: 'root',
    rootView: 'root',
    backStack: [],
  };
  assert.deepEqual(upsertBuilderPreview([], preview), [preview]);
  assert.deepEqual(
    applyBuilderPreviewActionResult([preview], 'timer.ts', { view: 'running' }),
    [{ ...preview, view: 'running', backStack: ['root'] }],
  );
});

test('restored preview resets stale navigation to its actual root', () => {
  const preview = {
    filename: 'timer.ts',
    view: 'stale-child',
    rootView: 'actual-root',
    backStack: ['actual-root'],
  };
  assert.deepEqual(resetBuilderPreviewState([preview], 'timer.ts'), [
    { ...preview, view: 'actual-root', backStack: [] },
  ]);
});

test('builder preview transitions support root, replace, and pop', () => {
  const preview = {
    filename: 'timer.ts',
    view: 'child',
    rootView: 'root',
    backStack: ['root'],
  };
  assert.deepEqual(
    applyBuilderPreviewActionResult([preview], 'timer.ts', {
      view: 'replacement',
      navigation: 'replace',
    }),
    [{ ...preview, view: 'replacement' }],
  );
  assert.deepEqual(
    applyBuilderPreviewActionResult([preview], 'timer.ts', {
      navigation: 'pop',
    }),
    [{ ...preview, view: 'root', backStack: [] }],
  );
  assert.deepEqual(
    applyBuilderPreviewActionResult([preview], 'timer.ts', {
      view: 'new-root',
      navigation: 'root',
    }),
    [{ ...preview, rootView: 'new-root', view: 'new-root', backStack: [] }],
  );
});

test('builder preview patches retain local selection', () => {
  const preview = {
    view: {
      type: 'list' as const,
      title: 'Timer',
      items: [
        { id: 'one', title: 'One' },
        { id: 'two', title: 'Two' },
      ],
    },
    selectedItemId: 'two',
  };
  assert.equal(builderPreviewSelectedItemId(preview.view, 'two'), 'two');
  assert.deepEqual(
    patchBuilderPreviewState(preview, {
      removeItemIds: ['two'],
    }).selectedItemId,
    'one',
  );
});

test('builder preview routes hydration and patches by its current view id', () => {
  const previews = [
    {
      filename: 'github-issues.ts',
      view: {
        id: 'issues',
        type: 'list' as const,
        title: 'Issues',
        isLoading: true,
        items: [{ id: 'one', title: 'One' }],
      },
      selectedItemId: 'one',
    },
    {
      filename: 'other.ts',
      view: {
        id: 'other',
        type: 'list' as const,
        title: 'Other',
        items: [{ id: 'other', title: 'Other' }],
      },
      selectedItemId: 'other',
    },
  ];
  const hydrated = hydrateBuilderPreviewViewById(previews, {
    viewId: 'issues',
    items: [
      { id: 'one', title: 'One' },
      { id: 'two', title: 'Two' },
    ],
  });
  assert.equal(hydrated[0].view.isLoading, false);
  assert.equal(hydrated[0].selectedItemId, 'one');
  assert.equal(hydrated[1], previews[1]);
  const patched = patchBuilderPreviewViewById(hydrated, 'issues', {
    items: [{ id: 'two', title: 'Updated' }],
  });
  assert.equal(patched[0].view.items?.[1]?.title, 'Updated');
  assert.equal(patched[0].selectedItemId, 'one');
  assert.equal(patchBuilderPreviewViewById(patched, 'missing', {}), patched);
});

test('builder preview keeps hydrate retry feedback local to the matching view', () => {
  const previews = [
    {
      filename: 'github-issues.ts',
      view: {
        id: 'issues',
        type: 'list' as const,
        title: 'Issues',
        items: [],
      } as CommandView,
      selectedItemId: '',
    },
  ];
  const hydrated = hydrateBuilderPreviewViewById(previews, {
    viewId: 'issues',
    error: { message: 'offline' },
    retry: true,
  });
  const retry = hydrated[0].view.items?.[1]?.primaryAction;
  assert.deepEqual(retry, {
    type: 'nativeAction',
    title: 'Retry',
    nativeAction: { kind: 'view-hydrate-retry', viewId: 'issues' },
  });
  const retrying = retryBuilderPreviewHydration(hydrated, 'issues');
  assert.equal(retrying[0].view.id, 'issues');
  assert.equal(retrying[0].view.isLoading, true);
  assert.equal(retrying[0].view.items?.length, 0);
});
