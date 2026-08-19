import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { CommandView } from './model';
import { patchCommandView } from './view-patches';

const baseView: CommandView = {
  type: 'list',
  title: 'Base',
  items: [
    { id: 'one', title: 'One' },
    { id: 'two', title: 'Two' },
  ],
};

test('patchCommandView patches existing items and removes deleted items', () => {
  const next = patchCommandView(baseView, {
    items: [{ id: 'two', title: 'Updated Two', subtitle: 'Changed' }],
    removeItemIds: ['one'],
  });

  assert.deepEqual(next.items, [
    { id: 'two', title: 'Updated Two', subtitle: 'Changed' },
  ]);
});

test('patchCommandView supports prepend and append modes', () => {
  assert.deepEqual(
    patchCommandView(baseView, {
      mode: 'prepend',
      items: [{ id: 'zero', title: 'Zero' }],
    }).items,
    [
      { id: 'zero', title: 'Zero' },
      { id: 'one', title: 'One' },
      { id: 'two', title: 'Two' },
    ],
  );

  assert.deepEqual(
    patchCommandView(baseView, {
      mode: 'append',
      items: [{ id: 'three', title: 'Three' }],
    }).items,
    [
      { id: 'one', title: 'One' },
      { id: 'two', title: 'Two' },
      { id: 'three', title: 'Three' },
    ],
  );
});

test('patchCommandView patches sections and can preserve missing root items', () => {
  const sectioned: CommandView = {
    type: 'list',
    title: 'Sectioned',
    sections: [{ title: 'A', items: [{ id: 'one', title: 'One' }] }],
  };
  const next = patchCommandView(
    sectioned,
    { items: [{ id: 'one', subtitle: 'Patched' }] },
    { preserveMissingItems: true },
  );

  assert.deepEqual(next.items, []);
  assert.deepEqual(next.sections?.[0]?.items, [
    { id: 'one', title: 'One', subtitle: 'Patched' },
  ]);

  const passive = patchCommandView(
    sectioned,
    { isLoading: false },
    { preserveMissingItems: true },
  );
  assert.equal(passive.items, undefined);
});

test('patchCommandView applies item patches to views that omitted items', () => {
  const loadingView: CommandView = { type: 'list', title: 'Loading' };

  assert.deepEqual(
    patchCommandView(loadingView, { mode: 'replace', items: [{ id: 'one' }] })
      .items,
    [{ id: 'one', title: 'one' }],
  );
  assert.deepEqual(
    patchCommandView(loadingView, {
      mode: 'prepend',
      items: [{ id: 'one', title: 'One' }],
    }).items,
    [{ id: 'one', title: 'One' }],
  );
  assert.deepEqual(
    patchCommandView(loadingView, {
      mode: 'append',
      items: [{ id: 'one', title: 'One' }],
    }).items,
    [{ id: 'one', title: 'One' }],
  );
});
