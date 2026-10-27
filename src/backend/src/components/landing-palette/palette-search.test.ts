import assert from 'node:assert/strict';
import test from 'node:test';
import { searchPalette } from './palette-search';
import type { PaletteItem } from './palette-types';

function item(
  title: string,
  overrides: Partial<PaletteItem> = {},
): PaletteItem {
  return {
    id: title,
    title,
    subtitle: '',
    aliases: [],
    kind: 'command',
    ...overrides,
  };
}

const items = [
  item('Calendar', { kind: 'app', subtitle: 'Launch application' }),
  item('Start Dictation', { keywords: ['dictate', 'voice'] }),
  item('Design system.fig', { kind: 'file', keywords: ['design'] }),
  item('Force Quit Apps', { aliases: ['kill'] }),
];

test('an empty query keeps the original order', () => {
  assert.deepEqual(
    searchPalette(items, '  ').map((result) => result.title),
    items.map((entry) => entry.title),
  );
});

test('matches titles, aliases, and hidden keywords', () => {
  assert.equal(searchPalette(items, 'calen')[0]?.title, 'Calendar');
  assert.equal(searchPalette(items, 'kill')[0]?.title, 'Force Quit Apps');
  assert.equal(searchPalette(items, 'dictate')[0]?.title, 'Start Dictation');
});

test('title matches outrank subtitle matches', () => {
  const ranked = searchPalette(
    [
      item('Notes', { subtitle: 'Open calendar notes' }),
      item('Calendar'),
    ],
    'calendar',
  );
  assert.equal(ranked[0]?.title, 'Calendar');
});

test('returns nothing when no field matches', () => {
  assert.deepEqual(searchPalette(items, 'zzzzqq'), []);
});
