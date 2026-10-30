import assert from 'node:assert/strict';
import test from 'node:test';
import { filterCommandItems } from './filtering';
import type { CommandItem } from './model';

const GRID_MIN_SCORE = 50;

function emojiItem(id: string, title: string, keywords: string[]) {
  return { id, title, keywords } as CommandItem;
}

const EMOJI_ITEMS = [
  emojiItem('grinning-cat', 'Grinning Cat', ['😺', 'grinning cat', 'cat']),
  emojiItem('cat-face', 'Cat Face', ['🐱', 'cat face', 'pet']),
  emojiItem('cat', 'Cat', ['🐈', 'cat', 'animal']),
  emojiItem('dog', 'Dog', ['🐕', 'dog', 'animal']),
];

function ids(items: CommandItem[]) {
  return items.map((item) => item.id);
}

test('exact title matches lead exact keyword matches, then catalog order', () => {
  assert.deepEqual(
    ids(filterCommandItems(EMOJI_ITEMS, 'Cat', { minScore: GRID_MIN_SCORE })),
    ['cat', 'grinning-cat', 'cat-face'],
  );
  assert.deepEqual(
    ids(filterCommandItems(EMOJI_ITEMS, 'pet', { minScore: GRID_MIN_SCORE })),
    ['cat-face'],
  );
});

test('exact keyword matches such as the glyph itself lead filtered items', () => {
  assert.deepEqual(
    ids(filterCommandItems(EMOJI_ITEMS, '🐈', { minScore: GRID_MIN_SCORE })),
    ['cat'],
  );
  assert.deepEqual(
    ids(
      filterCommandItems(EMOJI_ITEMS, 'animal', { minScore: GRID_MIN_SCORE }),
    ),
    ['cat', 'dog'],
  );
});

test('queries without exact matches keep catalog order', () => {
  assert.deepEqual(
    ids(filterCommandItems(EMOJI_ITEMS, 'ca', { minScore: GRID_MIN_SCORE })),
    ['grinning-cat', 'cat-face', 'cat'],
  );
  assert.deepEqual(ids(filterCommandItems(EMOJI_ITEMS, '')), ids(EMOJI_ITEMS));
});
