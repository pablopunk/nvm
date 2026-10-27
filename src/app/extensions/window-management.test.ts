// biome-ignore-all lint/suspicious/noExplicitAny: built-in extension contexts are intentionally untyped at this bridge boundary.
import assert from 'node:assert/strict';
import test from 'node:test';
import { createWindowManagementExtension } from './window-management';

const OK_RESULT = {
  ok: true,
  outcome: 'exact',
  changed: true,
  frame: { x: 0, y: 0, width: 1, height: 1 },
};

const EXPECTED_LAYOUTS = [
  {
    localId: 'layout.left-half',
    title: 'Left Half',
    spec: { kind: 'fraction', x: 0, y: 0, width: 0.5, height: 1 },
  },
  {
    localId: 'layout.right-half',
    title: 'Right Half',
    spec: { kind: 'fraction', x: 0.5, y: 0, width: 0.5, height: 1 },
  },
  {
    localId: 'layout.top-half',
    title: 'Top Half',
    spec: { kind: 'fraction', x: 0, y: 0, width: 1, height: 0.5 },
  },
  {
    localId: 'layout.bottom-half',
    title: 'Bottom Half',
    spec: { kind: 'fraction', x: 0, y: 0.5, width: 1, height: 0.5 },
  },
  {
    localId: 'layout.maximize',
    title: 'Maximize',
    spec: { kind: 'fraction', x: 0, y: 0, width: 1, height: 1 },
  },
  { localId: 'layout.center', title: 'Center', spec: { kind: 'center' } },
] as const;

function systemContext(hasWindowManagement: boolean) {
  return {
    system: {
      capabilities: {
        has: (capability: string) =>
          hasWindowManagement && capability === 'window-management',
      },
    },
  };
}

function actionsFor(hasWindowManagement: boolean) {
  return createWindowManagementExtension().actions(
    systemContext(hasWindowManagement) as any,
  ) as Array<{ id: string; title: string; run: (ctx: any) => unknown }>;
}

function runContextWith(apply: (...args: unknown[]) => Promise<unknown>) {
  const indicators: unknown[] = [];
  return {
    indicators,
    ctx: {
      desktop: { windowManagement: { apply } },
      ui: {
        indicator: {
          show: (input: unknown) => {
            indicators.push(input);
          },
          hide: () => undefined,
        },
      },
    },
  };
}

function actionByLocalId(localId: string) {
  const action = actionsFor(true).find((item) => item.id === localId);
  if (!action) {
    throw new Error(`expected action ${localId}`);
  }
  return action;
}

test('persisted ids for every P1 layout are stable', () => {
  assert.deepEqual(
    actionsFor(true).map(
      (action) => `extension-action:window-management:${action.id}`,
    ),
    [
      'extension-action:window-management:layout.left-half',
      'extension-action:window-management:layout.right-half',
      'extension-action:window-management:layout.top-half',
      'extension-action:window-management:layout.bottom-half',
      'extension-action:window-management:layout.maximize',
      'extension-action:window-management:layout.center',
    ],
  );
});

test('every layout is contributed once with its title', () => {
  assert.deepEqual(
    actionsFor(true).map((action) => [action.id, action.title]),
    EXPECTED_LAYOUTS.map((layout) => [layout.localId, layout.title]),
  );
});

test('layout actions are omitted when window management is unsupported', () => {
  assert.deepEqual(actionsFor(false), []);
});

test('declares the desktop.window-management capability', () => {
  assert.deepEqual(createWindowManagementExtension().capabilities, [
    'desktop.window-management',
  ]);
});

for (const layout of EXPECTED_LAYOUTS) {
  test(`${layout.title} applies its spec exactly once`, async () => {
    const calls: unknown[][] = [];
    const { ctx, indicators } = runContextWith((...args) => {
      calls.push(args);
      return Promise.resolve(OK_RESULT);
    });

    await actionByLocalId(layout.localId).run(ctx);

    assert.deepEqual(calls, [[layout.spec]]);
    assert.deepEqual(indicators, []);
  });
}

test('a failed layout shows its message as palette feedback', async () => {
  const { ctx, indicators } = runContextWith(() =>
    Promise.resolve({
      ok: false,
      reason: 'no-window',
      message: 'The focused app has no window to move.',
    }),
  );

  await actionByLocalId('layout.left-half').run(ctx);

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
  const indicators: unknown[] = [];
  const ctx = {
    desktop: {},
    ui: {
      indicator: {
        show: (input: unknown) => indicators.push(input),
        hide: () => undefined,
      },
    },
  };

  await actionByLocalId('layout.maximize').run(ctx);

  assert.equal((indicators[0] as { status: string }).status, 'error');
  assert.equal((indicators[0] as { title: string }).title, 'Maximize');
});
