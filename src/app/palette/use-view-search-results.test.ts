import assert from 'node:assert/strict';
import test from 'node:test';
import type { CommandItem } from './model';
import {
  mergeViewSearchResults,
  type ViewSearchResults,
} from './use-view-search-results';

const view = { id: 'picker', title: 'Picker' };
const catalog: CommandItem[] = ['tiger', 'cat', 'dog', 'whale'].map((id) => ({
  id,
  title: id,
}));
const results: ViewSearchResults = {
  viewKey: 'picker',
  query: 'cute',
  itemIds: ['cat', 'dog', 'missing'],
};

function ids(items: CommandItem[]) {
  return items.map((item) => item.id);
}

test('appends the named items after local matches in the given order', () => {
  assert.deepEqual(
    ids(mergeViewSearchResults([catalog[0]], catalog, results, view, 'cute')),
    ['tiger', 'cat', 'dog'],
  );
});

test('does not repeat items already matched locally', () => {
  assert.deepEqual(
    ids(mergeViewSearchResults([catalog[2]], catalog, results, view, 'cute')),
    ['dog', 'cat'],
  );
});

test('ignores results for other text, other views, or no results', () => {
  const local = [catalog[0]];
  assert.equal(
    mergeViewSearchResults(local, catalog, results, view, 'cut'),
    local,
  );
  assert.equal(
    mergeViewSearchResults(local, catalog, results, { title: 'Other' }, 'cute'),
    local,
  );
  assert.equal(
    mergeViewSearchResults(local, catalog, null, view, 'cute'),
    local,
  );
  assert.equal(
    mergeViewSearchResults(
      local,
      catalog,
      { ...results, itemIds: ['missing'] },
      view,
      'cute',
    ),
    local,
  );
});

test('compares against the trimmed search text', () => {
  assert.deepEqual(
    ids(mergeViewSearchResults([], catalog, results, view, '  cute ')),
    ['cat', 'dog'],
  );
});

test('keeps the second batch visible when the render cap is smaller than the local list', () => {
  const merged = mergeViewSearchResults(
    catalog.slice(0, 3),
    catalog,
    { ...results, itemIds: ['whale'] },
    { ...view, maxVisibleItems: 3 },
    'cute',
  );
  assert.deepEqual(ids(merged), ['tiger', 'cat', 'whale']);
});
