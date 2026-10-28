// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  customWindowCommandItems,
  customWindowCommandSearchItems,
  customWindowCommandsView,
  describeCustomWindowCommand,
} from './window-management-custom-commands';
import {
  CUSTOM_WINDOW_COMMANDS_KEY,
  extensionWindowCustomCommandsStore,
  MAX_CUSTOM_WINDOW_COMMANDS,
} from './window-management-custom-store';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const OK_RESULT = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: { x: 0, y: 0, width: 500, height: 900 },
};

const LEFT_THIRD = {
  id: 'custom-left-third',
  title: 'Left third',
  x: { value: 0, unit: 'points' },
  width: { value: 33, unit: 'percent' },
};

function seededContext(commands: unknown[] = [LEFT_THIRD]) {
  return fakeWindowManagementContext({
    storage: { [CUSTOM_WINDOW_COMMANDS_KEY]: commands },
    apply: () => Promise.resolve(OK_RESULT),
  });
}

function rowsOf(view: any) {
  return view.items.slice(1);
}

function panelActionsOf(row: any) {
  return row.actionPanel.sections[0].actions;
}

function fieldValue(form: any, id: string) {
  return form.fields.find((field: any) => field.id === id);
}

function formValuesFor(overrides: Record<string, string>) {
  return {
    title: 'Half',
    x: '',
    xUnit: 'points',
    y: '',
    yUnit: 'points',
    width: '50',
    widthUnit: 'percent',
    height: '',
    heightUnit: 'points',
    ...overrides,
  };
}

test('describes only the dimensions a command sets', () => {
  assert.equal(
    describeCustomWindowCommand(LEFT_THIRD),
    'X offset 0 pt · Width 33%',
  );
});

test('the list starts with a create row followed by one row per command', async () => {
  const { ctx } = seededContext();
  const view = (await customWindowCommandsView(ctx)) as any;

  assert.equal(view.title, 'Custom Window Commands');
  assert.equal(view.items[0].id, 'create-custom-command');
  assert.equal(view.items[0].title, 'Create Custom Command…');
  assert.deepEqual(
    rowsOf(view).map((row: any) => [row.id, row.title]),
    [[LEFT_THIRD.id, 'Left third']],
  );
});

test('Enter on a row applies its exact spec with the stored options', async () => {
  const { ctx, applyCalls } = seededContext();
  const [row] = rowsOf(await customWindowCommandsView(ctx));

  await row.primaryAction.handler(ctx);

  assert.deepEqual(applyCalls, [
    [
      {
        kind: 'exact',
        x: { value: 0, unit: 'points' },
        width: { value: 33, unit: 'percent' },
      },
      { gap: 0, cycle: true },
    ],
  ]);
});

test('Edit opens the form prefilled with the command values', async () => {
  const { ctx } = seededContext();
  const [row] = rowsOf(await customWindowCommandsView(ctx));
  const [editAction] = panelActionsOf(row);

  const form = await editAction.handler(ctx);

  assert.equal(form.type, 'form');
  assert.equal(form.title, 'Edit Custom Window Command');
  assert.equal(fieldValue(form, 'title').value, 'Left third');
  assert.equal(fieldValue(form, 'width').value, '33');
  assert.equal(fieldValue(form, 'widthUnit').value, 'percent');
  assert.equal(fieldValue(form, 'height').value, '');
});

test('saving an edit keeps the id and replaces the command', async () => {
  const { ctx, storage, invalidations } = seededContext();
  const [row] = rowsOf(await customWindowCommandsView(ctx));
  const form = await panelActionsOf(row)[0].handler(ctx);

  const result = await form.submitAction.handler(ctx, {
    formValues: formValuesFor({ title: 'Half', width: '50' }),
  });

  assert.equal(result.navigation, 'pop');
  const saved = storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY) as any[];
  assert.deepEqual(saved, [
    {
      id: LEFT_THIRD.id,
      title: 'Half',
      width: { value: 50, unit: 'percent' },
    },
  ]);
  assert.deepEqual(invalidations, ['root-items']);
});

test('creating a command stores a new id with only the set dimensions', async () => {
  const { ctx, storage } = seededContext([]);
  const view = (await customWindowCommandsView(ctx)) as any;
  const form = await view.items[0].primaryAction.handler(ctx);

  assert.equal(form.title, 'Create Custom Window Command');
  await form.submitAction.handler(ctx, {
    formValues: formValuesFor({ title: 'Half screen', width: '50' }),
  });

  const [created] = storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY) as any[];
  assert.ok(created.id.startsWith('custom-'));
  assert.deepEqual(created, {
    id: created.id,
    title: 'Half screen',
    width: { value: 50, unit: 'percent' },
  });
});

test('an empty title and an out-of-range width re-show the form with errors', async () => {
  const { ctx, storage } = seededContext([]);
  const view = (await customWindowCommandsView(ctx)) as any;
  const form = await view.items[0].primaryAction.handler(ctx);

  const result = await form.submitAction.handler(ctx, {
    formValues: formValuesFor({ title: '  ', width: '0' }),
  });

  assert.equal(result.navigation, 'replace');
  assert.equal(fieldValue(result.view, 'title').error, 'Enter a title.');
  assert.equal(
    fieldValue(result.view, 'width').error,
    'Enter a number above 0, up to 100%.',
  );
  assert.equal(storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY), undefined);
});

test('creating past the limit is refused with a title error', async () => {
  const full = Array.from(
    { length: MAX_CUSTOM_WINDOW_COMMANDS },
    (_, index) => ({
      id: `custom-${index}`,
      title: `Command ${index}`,
      width: { value: 50, unit: 'percent' },
    }),
  );
  const { ctx, storage } = seededContext(full);
  const view = (await customWindowCommandsView(ctx)) as any;
  const form = await view.items[0].primaryAction.handler(ctx);

  const result = await form.submitAction.handler(ctx, {
    formValues: formValuesFor({ title: 'One too many', width: '50' }),
  });

  assert.equal(
    fieldValue(result.view, 'title').error,
    'Limit reached: 100 custom commands.',
  );
  assert.equal(
    (storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY) as unknown[]).length,
    MAX_CUSTOM_WINDOW_COMMANDS,
  );
});

test('delete asks for confirmation and removes the command from the list', async () => {
  const { ctx, storage } = seededContext();
  const [row] = rowsOf(await customWindowCommandsView(ctx));
  const deleteAction = panelActionsOf(row)[1];

  assert.equal(deleteAction.title, 'Delete Command');
  assert.equal(deleteAction.requiresConfirmation, true);
  assert.equal(deleteAction.style, 'destructive');

  const patch = await deleteAction.handler(ctx);

  assert.deepEqual(storage.values.get(CUSTOM_WINDOW_COMMANDS_KEY), []);
  assert.deepEqual(
    patch.patch.items.map((item: any) => item.id),
    ['create-custom-command'],
  );
});

test('root search matches command titles and runs the match', async () => {
  const { ctx, applyCalls } = seededContext();

  const items = await customWindowCommandSearchItems(ctx, ' LEFT ');
  await items[0].primaryAction.handler(ctx);

  assert.deepEqual(
    items.map((item: any) => item.id),
    [`custom:${LEFT_THIRD.id}`],
  );
  assert.equal(applyCalls.length, 1);
});

test('root search stays silent for an empty query or unsupported systems', async () => {
  const { ctx } = seededContext();
  assert.deepEqual(await customWindowCommandSearchItems(ctx, '  '), []);

  const unsupported = fakeWindowManagementContext({
    storage: { [CUSTOM_WINDOW_COMMANDS_KEY]: [LEFT_THIRD] },
    withWindowManagementCapability: false,
  });
  assert.deepEqual(
    await customWindowCommandSearchItems(unsupported.ctx, 'left'),
    [],
  );
});

test('row items reflect the store contents at build time', async () => {
  const { ctx } = seededContext([]);
  const store = extensionWindowCustomCommandsStore(ctx);
  await store.save([LEFT_THIRD]);

  const items = await customWindowCommandItems(ctx, store);

  assert.equal(items.length, 2);
});
