import assert from 'node:assert/strict';
import test from 'node:test';
import { titleFromFirstContentLine } from './editor-title';

test('derives note titles from the first non-empty Markdown line', () => {
  assert.equal(
    titleFromFirstContentLine('\n# A durable idea\nMore detail'),
    'A durable idea',
  );
  assert.equal(titleFromFirstContentLine(''), 'Untitled note');
  assert.equal(
    titleFromFirstContentLine('***[Formatted](https://example.com)***'),
    'Formatted',
  );
});
