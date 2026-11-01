// biome-ignore-all lint: Service harness stubs dynamic extension shapes by design.
import assert from 'node:assert/strict';
import test, { mock } from 'node:test';

mock.module('electron', {
  namedExports: {
    app: { isPackaged: false, getPath: () => '/tmp/nvm-menubar-test' },
  },
});

mock.module('electron-log/main', {
  defaultExport: { error() {}, info() {}, warn() {}, debug() {} },
});

const { createExtensionMenuBarService } = await import(
  './extension-menubar-service'
);

function createHarness(extensions: any[], hidden = new Set<string>()) {
  const extensionModules = new Map(
    extensions.map((extension) => [extension.id, extension]),
  );
  const specs = new Map<string, any>();
  const ran: any[] = [];
  const bound: any[] = [];
  const service = createExtensionMenuBarService({
    extensionModules,
    statusItems: {
      supported: true,
      upsert(id, spec) {
        specs.set(id, spec);
        return true;
      },
      retainOnly(ids) {
        const keep = new Set(ids);
        for (const id of Array.from(specs.keys()))
          if (!keep.has(id)) specs.delete(id);
      },
    },
    createExtensionContext: (extension) => ({ extensionId: extension.id }),
    extensionRootActionFromItem: (entry, item) => {
      bound.push(item);
      return {
        kind: 'extension-root-item',
        extensionId: entry.extension.id,
        rootAction: item.primaryAction,
      };
    },
    runMenuAction: (action) => ran.push(action),
    renderTimeoutMs: 50,
    isVisibleExtension: (extension) => !hidden.has(extension.id),
  });
  return { service, specs, ran, bound, extensionModules };
}

function timerExtension(render: (ctx: any) => any) {
  return { id: 'pomodoro', title: 'Pomodoro', menuBar: { render } };
}

test('rendered menu bars become status item specs with sections', async () => {
  const { service, specs } = createHarness([
    timerExtension(() => ({
      icon: 'timer',
      title: '25:00',
      sections: [
        {
          title: 'Focus',
          items: [{ id: 'start', title: 'Start', action: { type: 'a' } }],
        },
        { items: [{ id: 'reset', title: 'Reset', disabled: true }] },
      ],
    })),
  ]);

  await service.syncAll();

  const spec = specs.get('pomodoro:main');
  assert.equal(spec.icon, 'timer');
  assert.equal(spec.title, '25:00');
  assert.deepEqual(
    spec.menu.map((entry) =>
      entry.type === 'item' ? entry.label : entry.type,
    ),
    ['header', 'Start', 'separator', 'Reset'],
  );
  assert.equal(spec.menu[3].enabled, false);
});

test('row actions bind lazily and run through the menu action runner', async () => {
  const { service, specs, ran, bound } = createHarness([
    timerExtension(() => ({
      items: [{ id: 'start', title: 'Start', action: { type: 'start' } }],
    })),
  ]);
  await service.syncAll();
  await service.refresh('pomodoro');
  assert.equal(bound.length, 0);

  specs.get('pomodoro:main').menu[0].onClick();

  assert.equal(bound.length, 1);
  assert.equal(bound[0].id, 'menubar:main:start');
  assert.deepEqual(ran[0].rootAction, { type: 'start' });
});

test('hidden extensions contribute no status items', async () => {
  const { service, specs } = createHarness(
    [timerExtension(() => ({ items: [] }))],
    new Set(['pomodoro']),
  );

  await service.syncAll();

  assert.equal(specs.size, 0);
});

test('a failing render keeps the last good menu', async () => {
  let fail = false;
  const { service, specs } = createHarness([
    timerExtension(() => {
      if (fail) throw new Error('boom');
      return { title: 'ok', items: [] };
    }),
  ]);
  await service.syncAll();
  fail = true;

  await service.refresh('pomodoro');

  assert.equal(specs.get('pomodoro:main').title, 'ok');
});

test('a render that never settles times out without blocking others', async () => {
  const { service, specs } = createHarness([
    timerExtension(() => new Promise(() => {})),
    {
      id: 'clock',
      title: 'Clock',
      menuBar: { render: () => ({ title: '12:00', items: [] }) },
    },
  ]);

  await service.syncAll();

  assert.equal(specs.has('pomodoro:main'), false);
  assert.equal(specs.get('clock:main').title, '12:00');
});

test('removed extensions lose their status items on the next sync', async () => {
  const harness = createHarness([timerExtension(() => ({ items: [] }))]);
  await harness.service.syncAll();
  harness.extensionModules.delete('pomodoro');

  await harness.service.syncAll();

  assert.equal(harness.specs.size, 0);
});

test('opening a status item refreshes its owning extension', async () => {
  let renders = 0;
  const { service } = createHarness([
    timerExtension(() => {
      renders += 1;
      return { items: [] };
    }),
  ]);
  await service.syncAll();

  service.refreshForStatusItem('pomodoro:main');
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(renders, 2);
});
