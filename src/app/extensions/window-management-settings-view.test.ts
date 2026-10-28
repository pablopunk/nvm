// biome-ignore-all lint/suspicious/noExplicitAny: settings view rows are untyped bridge payloads in tests.
import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_MANAGEMENT_SETTINGS_KEY } from './window-management-settings';
import { windowManagementSettingsView } from './window-management-settings-view';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const SETTING_ROW_IDS = [
  'gap',
  'cycle',
  'almostMaximizePercent',
  'reasonableSizePercent',
  'resizeStepPercent',
  'reset-defaults',
];

async function settingsRootOf(storage: Record<string, unknown> = {}) {
  const harness = fakeWindowManagementContext({ storage });
  const view = (await windowManagementSettingsView(harness.ctx)) as any;
  return { ...harness, view, rows: view.items as any[] };
}

function rowById(rows: any[], id: string) {
  const row = rows.find((item) => item.id === id);
  if (!row) {
    throw new Error(`expected row ${id}`);
  }
  return row;
}

function accessoryText(row: any) {
  return row.accessories?.[0]?.text;
}

test('root view lists one row per setting plus reset, in order', async () => {
  const { view } = await settingsRootOf();
  assert.equal(view.title, 'Window Management Settings');
  assert.equal(view.presentation, 'root');
  assert.deepEqual(
    view.items.map((item: any) => item.id),
    SETTING_ROW_IDS,
  );
});

test('rows show the current stored values', async () => {
  const { rows } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { gap: 12, cycle: false },
  });
  assert.equal(accessoryText(rowById(rows, 'gap')), '12 px');
  assert.equal(accessoryText(rowById(rows, 'cycle')), 'Off');
  assert.equal(accessoryText(rowById(rows, 'almostMaximizePercent')), '90%');
  assert.equal(accessoryText(rowById(rows, 'reasonableSizePercent')), '60%');
  assert.equal(accessoryText(rowById(rows, 'resizeStepPercent')), '5%');
});

test('a numeric row opens a prompt pre-filled with its value', async () => {
  const { rows } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { gap: 12 },
  });
  const prompt = rowById(rows, 'gap').primaryAction;
  assert.equal(prompt.type, 'prompt-action');
  assert.equal(prompt.title, 'Window Gap');
  assert.deepEqual(
    prompt.fields.map((field: any) => [field.type, field.value]),
    [['number', '12']],
  );
});

test('a valid prompt value saves, pops the prompt, and refreshes the root view', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const save = rowById(rows, 'gap').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '24' } });

  assert.equal(result.navigation, 'pop');
  assert.deepEqual(result.action, { type: 'refresh-action' });
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).gap,
    24,
  );
});

test('an invalid prompt value replaces the prompt with a field error and saves nothing', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const save = rowById(rows, 'gap').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '150' } });

  assert.equal(result.navigation, 'replace');
  assert.equal(result.view.type, 'prompt');
  assert.equal(result.view.fields[0].value, '150');
  assert.equal(
    result.view.fields[0].error,
    'Enter a whole number from 0 to 100.',
  );
  assert.equal(await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY), undefined);
});

test('the cycle toggle flips the stored value and patches only the setting rows', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const toggle = rowById(rows, 'cycle').primaryAction;

  const result = await toggle.handler(ctx);

  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).cycle,
    false,
  );
  const patchedIds = result.patch.items.map((item: any) => item.id);
  assert.deepEqual(patchedIds, SETTING_ROW_IDS.slice(0, 5));
  assert.equal(accessoryText(result.patch.items[1]), 'Off');
});

test('reset restores defaults, patches every setting row, and confirms with success feedback', async () => {
  const { rows, ctx, storage, indicators } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { gap: 40, cycle: false },
  });
  const reset = rowById(rows, 'reset-defaults').primaryAction;

  const result = await reset.handler(ctx);

  assert.deepEqual(await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY), {
    gap: 0,
    cycle: true,
    almostMaximizePercent: 90,
    reasonableSizePercent: 60,
    resizeStepPercent: 5,
  });
  assert.equal(accessoryText(result.patch.items[0]), '0 px');
  assert.equal(accessoryText(result.patch.items[1]), 'On');
  assert.equal(indicators.at(-1).status, 'success');
});
