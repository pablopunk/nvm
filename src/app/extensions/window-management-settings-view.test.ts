// biome-ignore-all lint/suspicious/noExplicitAny: settings view rows are untyped bridge payloads in tests.
// biome-ignore-all lint/style/noMagicNumbers: row indices and counts describe the fixed settings layout.
import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_MANAGEMENT_SETTINGS_KEY } from './window-management-settings';
import { windowManagementSettingsView } from './window-management-settings-view';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const SECTION_TITLES = ['Gaps', 'Sizes', 'Behavior', 'Displays'];

const GAP_ROW_IDS = [
  'outerGap',
  'outerGapTop',
  'outerGapRight',
  'outerGapBottom',
  'outerGapLeft',
  'innerGap',
  'applyGapToMaximize',
  'reset-gaps',
];

async function settingsRootOf(storage: Record<string, unknown> = {}) {
  const harness = fakeWindowManagementContext({ storage });
  const view = (await windowManagementSettingsView(harness.ctx)) as any;
  const rows = view.sections.flatMap((section: any) => section.items) as any[];
  return { ...harness, view, rows };
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

test('root view groups settings into Gaps, Sizes, Behavior, and Displays', async () => {
  const { view } = await settingsRootOf();
  assert.equal(view.title, 'Window Management Settings');
  assert.equal(view.presentation, 'root');
  assert.equal(view.items, undefined);
  const titles = view.sections.map((section: any) => section.title);
  assert.deepEqual(titles, [...SECTION_TITLES, undefined]);
});

test('each section lists its settings then its reset row', async () => {
  const { view } = await settingsRootOf();
  const [gaps, sizes, behavior, displays, global] = view.sections;
  assert.deepEqual(
    gaps.items.map((item: any) => item.id),
    GAP_ROW_IDS,
  );
  assert.deepEqual(
    sizes.items.map((item: any) => item.id),
    [
      'almostMaximizeWidthPercent',
      'almostMaximizeHeightPercent',
      'reasonableSizeWidthPercent',
      'reasonableSizeHeightPercent',
      'centerHalfWidthPercent',
      'resizeStepPercent',
      'reset-sizes',
    ],
  );
  assert.deepEqual(
    behavior.items.map((item: any) => item.id),
    ['cycle', 'cycleOrder', 'reset-behavior'],
  );
  assert.deepEqual(
    displays.items.map((item: any) => item.id),
    ['displayOrder', 'displayMoveSizing', 'reset-displays'],
  );
  assert.deepEqual(
    global.items.map((item: any) => item.id),
    ['reset-defaults'],
  );
});

test('rows show the current stored values', async () => {
  const { rows } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: {
      outerGap: 12,
      cycle: false,
      cycleOrder: 'half-third-two-thirds',
    },
  });
  assert.equal(accessoryText(rowById(rows, 'outerGap')), '12 px');
  assert.equal(accessoryText(rowById(rows, 'innerGap')), '4 px');
  assert.equal(
    accessoryText(rowById(rows, 'almostMaximizeWidthPercent')),
    '90%',
  );
  assert.equal(
    accessoryText(rowById(rows, 'reasonableSizeWidthPercent')),
    '70%',
  );
  assert.equal(accessoryText(rowById(rows, 'resizeStepPercent')), '5%');
  assert.equal(accessoryText(rowById(rows, 'cycle')), 'Off');
  assert.equal(accessoryText(rowById(rows, 'cycleOrder')), '½ → ⅓ → ⅔');
  assert.equal(accessoryText(rowById(rows, 'displayOrder')), 'Left to right');
  assert.equal(accessoryText(rowById(rows, 'applyGapToMaximize')), 'On');
});

test('an edge override with no value says it follows the outer gap', async () => {
  const { rows } = await settingsRootOf();
  const top = rowById(rows, 'outerGapTop');
  assert.equal(accessoryText(top), 'Same as outer gap');
  assert.equal(top.accessories[0].tone, 'muted');
});

test('an edge override with a value shows that value', async () => {
  const { rows } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { outerGapTop: 0 },
  });
  assert.equal(accessoryText(rowById(rows, 'outerGapTop')), '0 px');
});

test('a numeric row opens a prompt pre-filled with its value', async () => {
  const { rows } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { outerGap: 12 },
  });
  const prompt = rowById(rows, 'outerGap').primaryAction;
  assert.equal(prompt.type, 'prompt-action');
  assert.equal(prompt.title, 'Outer Gap');
  assert.deepEqual(
    prompt.fields.map((field: any) => [
      field.type,
      field.value,
      field.required,
    ]),
    [['number', '12', true]],
  );
});

test('an edge override prompt is pre-filled blank and optional', async () => {
  const { rows } = await settingsRootOf();
  const prompt = rowById(rows, 'outerGapLeft').primaryAction;
  assert.deepEqual(
    prompt.fields.map((field: any) => [field.value, field.required]),
    [['', false]],
  );
});

test('a valid prompt value saves, pops the prompt, and refreshes the root view', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const save = rowById(rows, 'outerGap').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '24' } });

  assert.equal(result.navigation, 'pop');
  assert.deepEqual(result.action, { type: 'refresh-action' });
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).outerGap,
    24,
  );
});

test('an invalid prompt value replaces the prompt with a field error and saves nothing', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const save = rowById(rows, 'outerGap').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '150' } });

  assert.equal(result.navigation, 'replace');
  assert.equal(result.view.type, 'prompt');
  assert.equal(result.view.fields[0].value, '150');
  assert.equal(
    result.view.fields[0].error,
    'Enter a whole number from 0 to 40.',
  );
  assert.equal(await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY), undefined);
});

test('a blank edge override saves as null so the edge follows the outer gap', async () => {
  const { rows, ctx, storage } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { outerGapRight: 20 },
  });
  const save = rowById(rows, 'outerGapRight').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '' } });

  assert.equal(result.navigation, 'pop');
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).outerGapRight,
    null,
  );
});

test('a blank value for a required setting is rejected with the range message', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const save = rowById(rows, 'innerGap').primaryAction.action;

  const result = await save.handler(ctx, { formValues: { value: '' } });

  assert.equal(result.navigation, 'replace');
  assert.equal(
    result.view.fields[0].error,
    'Enter a whole number from 0 to 40.',
  );
  assert.equal(await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY), undefined);
});

test('the gap-on-maximize toggle flips the stored value and patches only its row', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const toggle = rowById(rows, 'applyGapToMaximize').primaryAction;

  const result = await toggle.handler(ctx);

  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any)
      .applyGapToMaximize,
    false,
  );
  assert.deepEqual(
    result.patch.items.map((item: any) => item.id),
    ['applyGapToMaximize'],
  );
  assert.equal(accessoryText(result.patch.items[0]), 'Off');
});

test('the cycle-order choice advances to the next option and wraps', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  const change = rowById(rows, 'cycleOrder').primaryAction;

  const first = await change.handler(ctx);
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).cycleOrder,
    'half-third-two-thirds',
  );
  assert.equal(accessoryText(first.patch.items[0]), '½ → ⅓ → ⅔');

  await change.handler(ctx);
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any).cycleOrder,
    'half-two-thirds-third',
  );
});

test('the display-move-sizing choice cycles between its two options', async () => {
  const { rows, ctx, storage } = await settingsRootOf();
  await rowById(rows, 'displayMoveSizing').primaryAction.handler(ctx);
  assert.equal(
    ((await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any)
      .displayMoveSizing,
    'keep-size',
  );
});

test('a section reset restores only that section and patches its rows', async () => {
  const { rows, ctx, storage, indicators } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: {
      outerGap: 40,
      outerGapTop: 2,
      innerGap: 30,
      cycle: false,
    },
  });
  const reset = rowById(rows, 'reset-gaps').primaryAction;

  const result = await reset.handler(ctx);

  const saved = (await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any;
  assert.equal(saved.outerGap, 8);
  assert.equal(saved.outerGapTop, null);
  assert.equal(saved.innerGap, 4);
  assert.equal(saved.cycle, false);
  assert.deepEqual(
    result.patch.items.map((item: any) => item.id),
    GAP_ROW_IDS.slice(0, -1),
  );
  assert.equal(accessoryText(result.patch.items[0]), '8 px');
  assert.equal(indicators.at(-1).title, 'Gaps');
  assert.equal(indicators.at(-1).status, 'success');
});

test('global reset restores defaults, patches every setting row, and confirms with success feedback', async () => {
  const { rows, ctx, storage, indicators } = await settingsRootOf({
    [WINDOW_MANAGEMENT_SETTINGS_KEY]: { outerGap: 40, cycle: false },
  });
  const reset = rowById(rows, 'reset-defaults').primaryAction;

  const result = await reset.handler(ctx);

  const saved = (await storage.get(WINDOW_MANAGEMENT_SETTINGS_KEY)) as any;
  assert.equal(saved.outerGap, 8);
  assert.equal(saved.cycle, true);
  assert.equal(result.patch.items.length, 17);
  assert.equal(accessoryText(result.patch.items[0]), '8 px');
  assert.equal(result.patch.items[13].id, 'cycle');
  assert.equal(accessoryText(result.patch.items[13]), 'On');
  assert.equal(indicators.at(-1).status, 'success');
});
