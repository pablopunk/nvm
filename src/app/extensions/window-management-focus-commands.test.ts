// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createWindowManagementExtension } from './window-management';
import { fakeWindowManagementContext } from './window-management-test-doubles';

const FOCUS_ACTION_IDS = [
  'layout.focus-left',
  'layout.focus-right',
  'layout.focus-up',
  'layout.focus-down',
];

const FOCUS_ICONS = ['arrow-left', 'arrow-right', 'arrow-up', 'arrow-down'];

const NO_NEIGHBOR_MESSAGE = 'No window lies in that direction.';

function focusContext(focusResult: unknown) {
  const focusCalls: unknown[][] = [];
  const fake = fakeWindowManagementContext();
  const ctx = fake.ctx as any;
  ctx.desktop.windowManagement.focusNeighbor = (...args: unknown[]) => {
    focusCalls.push(args);
    return Promise.resolve(focusResult);
  };
  return { ctx, focusCalls, indicators: fake.indicators };
}

async function focusActionsFor(ctx: any) {
  const actions = (await createWindowManagementExtension().actions(
    ctx,
  )) as any[];
  return actions.filter((action) => FOCUS_ACTION_IDS.includes(action.id));
}

test('contributes four focus actions with arrow icons under Window Management', async () => {
  const { ctx } = focusContext({ ok: true });
  const actions = await focusActionsFor(ctx);
  assert.deepEqual(
    actions.map((action) => action.id),
    FOCUS_ACTION_IDS,
  );
  assert.deepEqual(
    actions.map((action) => action.icon),
    FOCUS_ICONS,
  );
  assert.deepEqual(
    actions.map((action) => action.subtitle),
    [
      'Window Management',
      'Window Management',
      'Window Management',
      'Window Management',
    ],
  );
  assert.deepEqual(
    actions.map((action) => action.title),
    [
      'Focus Window Left',
      'Focus Window Right',
      'Focus Window Up',
      'Focus Window Down',
    ],
  );
});

test('each focus action asks the desktop facet for its own direction', async () => {
  const { ctx, focusCalls } = focusContext({ ok: true });
  await Promise.all(
    (await focusActionsFor(ctx)).map((action) => action.run(ctx)),
  );
  assert.deepEqual(focusCalls, [['left'], ['right'], ['up'], ['down']]);
});

test('a rejected focus result shows its message as an error', async () => {
  const { ctx, indicators } = focusContext({
    ok: false,
    reason: 'no-neighbor',
    message: NO_NEIGHBOR_MESSAGE,
  });
  const [left] = await focusActionsFor(ctx);
  await left.run(ctx);
  assert.equal(indicators.length, 1);
  assert.equal(indicators[0].title, 'Focus Window Left');
  assert.equal(indicators[0].subtitle, NO_NEIGHBOR_MESSAGE);
  assert.equal(indicators[0].status, 'error');
});

test('a successful focus shows no feedback', async () => {
  const { ctx, indicators } = focusContext({ ok: true });
  const [, right] = await focusActionsFor(ctx);
  await right.run(ctx);
  assert.equal(indicators.length, 0);
});

test('a missing desktop facet shows the unavailable message', async () => {
  const { ctx, indicators } = focusContext({ ok: true });
  ctx.desktop.windowManagement = undefined;
  const [, , up] = await focusActionsFor(ctx);
  await up.run(ctx);
  assert.equal(indicators.length, 1);
  assert.equal(indicators[0].status, 'error');
  assert.equal(
    indicators[0].subtitle,
    'Window management is not available on this system.',
  );
});
