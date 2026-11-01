import assert from 'node:assert/strict';
import test from 'node:test';
import {
  extensionMenuBarRenderer,
  extensionMenuBarTriggers,
  normalizeExtensionMenuBars,
} from './extension-menubar';

const extension = { id: 'pomodoro', title: 'Pomodoro' };

test('a single descriptor becomes the main menu bar with defaults', () => {
  const { menuBars, issues } = normalizeExtensionMenuBars(extension, {
    items: [{ id: 'start', title: 'Start Focus', action: { type: 'x' } }],
  });

  assert.deepEqual(issues, []);
  assert.deepEqual(menuBars, [
    {
      id: 'main',
      icon: 'zap',
      tooltip: 'Pomodoro',
      sections: [
        {
          items: [{ id: 'start', title: 'Start Focus', action: { type: 'x' } }],
        },
      ],
    },
  ]);
});

test('unknown icons fall back to zap and are reported', () => {
  const { menuBars, issues } = normalizeExtensionMenuBars(extension, {
    icon: 'not-an-icon',
    items: [{ id: 'a', title: 'A', icon: 'also-wrong' }],
  });

  assert.equal(menuBars[0].icon, 'zap');
  assert.equal(menuBars[0].sections[0].items[0].icon, undefined);
  assert.equal(issues.length, 2);
});

test('status titles are capped so they cannot flood the menu bar', () => {
  const { menuBars } = normalizeExtensionMenuBars(extension, {
    title: 'A very long status title that keeps going',
    items: [],
  });

  assert.equal(menuBars[0].title?.length, 24);
  assert.ok(menuBars[0].title?.endsWith('…'));
});

test('rows without id or title are dropped and empty sections removed', () => {
  const { menuBars, issues } = normalizeExtensionMenuBars(extension, {
    sections: [
      { title: 'Empty', items: [{ title: 'No id' }] },
      { title: 'Kept', items: [{ id: 'ok', title: 'OK' }] },
    ],
  });

  assert.deepEqual(
    menuBars[0].sections.map((section) => section.title),
    ['Kept'],
  );
  assert.equal(issues.length, 1);
});

test('submenus nest one level and drop the parent action', () => {
  const { menuBars } = normalizeExtensionMenuBars(extension, {
    items: [
      {
        id: 'parent',
        title: 'Parent',
        action: { type: 'ignored' },
        items: [
          {
            id: 'child',
            title: 'Child',
            items: [{ id: 'grandchild', title: 'Too deep' }],
          },
        ],
      },
    ],
  });

  const parent = menuBars[0].sections[0].items[0];
  assert.equal(parent.action, undefined);
  assert.equal(parent.items?.[0].id, 'child');
  assert.equal(parent.items?.[0].items, undefined);
});

test('extra menu bars need stable unique ids and are capped at three', () => {
  const { menuBars, issues } = normalizeExtensionMenuBars(extension, [
    { items: [] },
    { items: [] },
    { id: 'b', items: [] },
    { id: 'b', items: [] },
    { id: 'c', items: [] },
  ]);

  assert.deepEqual(
    menuBars.map((menuBar) => menuBar.id),
    ['main', 'b'],
  );
  assert.ok(issues.some((issue) => issue.message.includes('at most 3')));
});

test('null hides every menu bar item', () => {
  assert.deepEqual(normalizeExtensionMenuBars(extension, null).menuBars, []);
});

test('only menuBar.render contributions are recognised', () => {
  const render = () => null;
  assert.equal(extensionMenuBarRenderer({ menuBar: { render } }), render);
  assert.equal(extensionMenuBarRenderer({ menuBar: render }), null);
  assert.equal(extensionMenuBarRenderer({}), null);
  assert.deepEqual(
    extensionMenuBarTriggers({
      menuBar: { render, triggers: [{ type: 'interval', every: '1m' }] },
    }),
    [{ type: 'interval', every: '1m' }],
  );
});
