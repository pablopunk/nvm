import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_SPECS } from '../electron/os/window-layout-catalog';
import {
  applyWindowLayout,
  windowLayoutAction,
  windowLayoutActions,
} from './window-management-commands';
import { WINDOW_MANAGEMENT_SETTINGS_KEY } from './window-management-settings';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const OK_RESULT = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: { x: 0, y: 0, width: 1, height: 1 },
};

const PARTIAL_WITH_REASON = {
  ok: true,
  outcome: 'partial',
  changed: true,
  frame: { x: 0, y: 0, width: 600, height: 900 },
  reason: 'The app enforces a minimum width of 600 points.',
};

test('applies the spec and the stored gap and cycle options', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    storage: {
      [WINDOW_MANAGEMENT_SETTINGS_KEY]: { gap: 8, cycle: false },
    },
    apply: () => Promise.resolve(OK_RESULT),
  });

  await applyWindowLayout(ctx, 'left-half');

  assert.deepEqual(applyCalls, [
    [WINDOW_LAYOUT_SPECS['left-half'], { gap: 8, cycle: false }],
  ]);
});

test('settings-dependent layouts apply the configured spec', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    storage: {
      [WINDOW_MANAGEMENT_SETTINGS_KEY]: { almostMaximizePercent: 80 },
    },
    apply: () => Promise.resolve(OK_RESULT),
  });

  await applyWindowLayout(ctx, 'almost-maximize');

  assert.deepEqual(applyCalls, [
    [
      { kind: 'scaled-center', widthFraction: 0.8, heightFraction: 0.8 },
      { gap: 0, cycle: true },
    ],
  ]);
});

test('defaults apply when no settings are stored', async () => {
  const { ctx, applyCalls } = fakeWindowManagementContext({
    apply: () => Promise.resolve(OK_RESULT),
  });

  await applyWindowLayout(ctx, 'make-larger');

  assert.deepEqual(applyCalls, [
    [
      { kind: 'resize-step', direction: 'larger', step: 0.05 },
      { gap: 0, cycle: true },
    ],
  ]);
});

test('a successful exact layout is silent', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    apply: () => Promise.resolve(OK_RESULT),
  });

  await applyWindowLayout(ctx, 'maximize');

  assert.deepEqual(indicators, []);
});

test('a partial layout with a reason shows that reason as neutral feedback', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    apply: () => Promise.resolve(PARTIAL_WITH_REASON),
  });

  await applyWindowLayout(ctx, 'right-half');

  assert.deepEqual(indicators, [
    {
      id: 'feedback',
      title: 'Right Half',
      subtitle: 'The app enforces a minimum width of 600 points.',
      durationMs: 2200,
    },
  ]);
});

test('a partial layout without a reason stays silent', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    apply: () => Promise.resolve({ ...PARTIAL_WITH_REASON, reason: undefined }),
  });

  await applyWindowLayout(ctx, 'right-half');

  assert.deepEqual(indicators, []);
});

test('a failed layout shows its message as error feedback', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    apply: () =>
      Promise.resolve({
        ok: false,
        reason: 'no-window',
        message: 'The focused app has no window to move.',
      }),
  });

  await applyWindowLayout(ctx, 'left-half');

  assert.deepEqual(indicators, [
    {
      id: 'feedback',
      title: 'Left Half',
      subtitle: 'The focused app has no window to move.',
      status: 'error',
      durationMs: 4000,
    },
  ]);
});

test('a missing window management facet shows unavailable feedback', async () => {
  const { ctx, indicators } = fakeWindowManagementContext({
    withWindowManagementFacet: false,
  });

  await applyWindowLayout(ctx, 'maximize');

  assert.deepEqual(indicators, [
    {
      id: 'feedback',
      title: 'Maximize',
      subtitle: 'Window management is not available on this system.',
      status: 'error',
      durationMs: 4000,
    },
  ]);
});

test('each catalog layout contributes a layout.<id> action with a run handler', () => {
  const actions = windowLayoutActions();
  assert.equal(actions.length, Object.keys(WINDOW_LAYOUT_SPECS).length);
  assert.equal(windowLayoutAction('restore').id, 'layout.restore');
  for (const action of actions) {
    assert.equal(action.mode, 'noView');
    assert.equal(typeof action.run, 'function');
  }
});
