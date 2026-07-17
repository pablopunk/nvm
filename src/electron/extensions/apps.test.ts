import assert from 'node:assert/strict';
import test from 'node:test';
import { initExtensionContext } from './_context';
import { createAppsExtension } from './apps';

test('force quit paints its view before running the OS process scan', async () => {
  let refreshCount = 0;
  initExtensionContext({
    appIndexService: {
      get: () => [
        { id: 'notes', name: 'Notes', path: '/Applications/Notes.app' },
      ],
    },
    runningAppStatus: {
      refresh: () => {
        refreshCount += 1;
        return Promise.resolve(new Set(['/applications/notes.app']));
      },
    },
  });

  const command = createAppsExtension().commands[0];
  let loader: (() => Promise<unknown[]>) | undefined;
  const view = command.run({
    ui: { list: (input: unknown) => input },
    data: {
      loader: (work: () => Promise<unknown[]>) => {
        loader = work;
        return { _loader: true };
      },
    },
  }) as { items: { _loader: boolean }; emptyView: { title: string } };

  assert.equal(refreshCount, 0);
  assert.equal(view.items._loader, true);
  assert.equal(view.emptyView.title, 'No running apps');
  assert.ok(loader);

  const items = await loader();
  assert.equal(refreshCount, 1);
  assert.deepEqual(
    items.map((item) => (item as { title: string }).title),
    ['Notes'],
  );
});
