// biome-ignore-all lint/suspicious/noExplicitAny: fake extension contexts mirror the untyped runtime bridge.
import assert from 'node:assert/strict';
import test from 'node:test';
import {
  EXACT_WINDOW_PRESENTATION,
  exactWindowAction,
  exactWindowFields,
} from './window-management-exact-commands';
import { WINDOW_MANAGEMENT_SETTINGS_KEY } from './window-management-settings';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const OK_RESULT = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: { x: 10, y: 0, width: 400, height: 300 },
};

function submitOf(kind: 'move' | 'resize', ctx: any) {
  return exactWindowAction(ctx, kind).action.action;
}

test('move and resize open a palette prompt with the default unit', () => {
  const { ctx } = fakeWindowManagementContext();
  const move = exactWindowAction(ctx, 'move');
  const resize = exactWindowAction(ctx, 'resize');

  assert.equal(move.title, 'Move Window…');
  assert.equal(resize.title, 'Resize Window…');
  assert.equal(move.action.type, 'prompt-action');
  assert.equal(exactWindowFields('move')[0]?.value, 'points');
  assert.equal(exactWindowFields('resize')[0]?.value, 'percent');
  assert.equal(EXACT_WINDOW_PRESENTATION.move.id, 'move-window');
  assert.equal(EXACT_WINDOW_PRESENTATION.resize.id, 'resize-window');
});

test('move collects an X and Y offset with a shared unit field', () => {
  const ids = exactWindowFields('move').map((field) => field.id);
  assert.deepEqual(ids, ['unit', 'x', 'y']);
});

test('resize collects width and height with a shared unit field', () => {
  const ids = exactWindowFields('resize').map((field) => field.id);
  assert.deepEqual(ids, ['unit', 'width', 'height']);
});

test('submitting an offset applies an exact spec with the chosen unit', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    apply: () => Promise.resolve(OK_RESULT),
  });

  await submitOf('move', ctx).handler(ctx, {
    formValues: { unit: 'percent', x: '10', y: '' },
  });

  assert.deepEqual(applyCalls, [
    [
      { kind: 'exact', x: { value: 10, unit: 'percent' } },
      { gap: 0, cycle: true },
    ],
  ]);
});

test('submitting sizes applies width and height and forwards stored options', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    storage: { [WINDOW_MANAGEMENT_SETTINGS_KEY]: { gap: 4, cycle: false } },
    apply: () => Promise.resolve(OK_RESULT),
  });

  await submitOf('resize', ctx).handler(ctx, {
    formValues: { unit: 'points', width: '800', height: '600' },
  });

  assert.deepEqual(applyCalls, [
    [
      {
        kind: 'exact',
        width: { value: 800, unit: 'points' },
        height: { value: 600, unit: 'points' },
      },
      { gap: 4, cycle: false },
    ],
  ]);
});

test('an invalid value re-shows the prompt with the error on that field', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    apply: () => Promise.resolve(OK_RESULT),
  });

  const result = await submitOf('move', ctx).handler(ctx, {
    formValues: { unit: 'points', x: '-3', y: '5' },
  });

  assert.equal(applyCalls.length, 0);
  assert.equal(result.navigation, 'replace');
  const xField = result.view.fields.find((field: any) => field.id === 'x');
  assert.equal(xField.error, 'Enter a number from 0 to 100000.');
  const yField = result.view.fields.find((field: any) => field.id === 'y');
  assert.equal(yField.error, undefined);
});

test('submitting with every field blank explains that a value is needed', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    apply: () => Promise.resolve(OK_RESULT),
  });

  const result = await submitOf('resize', ctx).handler(ctx, {
    formValues: { unit: 'percent', width: '', height: '' },
  });

  assert.equal(applyCalls.length, 0);
  assert.equal(result.view.subtitle, 'Enter at least one value.');
});

test('a failed apply shows its palette message as an error', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    apply: () =>
      Promise.resolve({
        ok: false,
        reason: 'target-lost',
        message: 'The window closed before it could move.',
      }),
  });

  await submitOf('move', ctx).handler(ctx, {
    formValues: { unit: 'points', x: '20' },
  });

  assert.deepEqual(indicators.at(-1), {
    id: 'feedback',
    title: 'Move Window…',
    subtitle: 'The window closed before it could move.',
    status: 'error',
    durationMs: 4000,
  });
});

test('a missing windowManagement facet reports the feature as unavailable', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    withWindowManagementFacet: false,
  });

  await submitOf('resize', ctx).handler(ctx, {
    formValues: { unit: 'percent', width: '50' },
  });

  assert.equal(
    indicators.at(-1)?.subtitle,
    'Window management is not available on this system.',
  );
});
