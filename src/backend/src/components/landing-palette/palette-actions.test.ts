import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AUTOMATE_ID,
  menuActions,
  primaryTitle,
  WEB_SEARCH_ID,
  withQueryFallbacks,
} from './palette-actions';
import type { PaletteItem } from './palette-types';

function item(title: string, overrides: Partial<PaletteItem> = {}): PaletteItem {
  return { id: title, title, subtitle: '', aliases: [], kind: 'command', ...overrides };
}

test('an empty query has no fallbacks', () => {
  const results = [item('Calendar')];
  assert.deepEqual(withQueryFallbacks(results, '   '), results);
});

test('a query gets Automate, then the web search, after the strong matches', () => {
  const results = Array.from({ length: 9 }, (_, index) => item(`Result ${index}`));
  const rows = withQueryFallbacks(results, '  theme   switcher ');
  assert.equal(rows.length, 11);
  assert.equal(rows[7]?.id, AUTOMATE_ID);
  assert.equal(rows[7]?.title, 'Automate "theme switcher"');
  assert.equal(rows[8]?.id, WEB_SEARCH_ID);
  assert.equal(rows[8]?.url, 'https://www.google.com/search?q=theme%20switcher');
});

test('a query with no matches still offers the fallbacks', () => {
  const rows = withQueryFallbacks([], 'zzz');
  assert.deepEqual(
    rows.map((row) => row.id),
    [AUTOMATE_ID, WEB_SEARCH_ID],
  );
});

test('the Enter action is named for the kind unless the item names it', () => {
  assert.equal(primaryTitle(item('Helium', { kind: 'app' })), 'Open Helium');
  assert.equal(primaryTitle(item('#f4cf86', { kind: 'clipboard' })), 'Paste Text');
  assert.equal(primaryTitle(item('Lock Screen')), 'Lock Screen');
  assert.equal(primaryTitle(item('Settings', { primary: 'Open Settings' })), 'Open Settings');
});

test('the action menu starts with the Enter action', () => {
  const actions = menuActions(item('Helium', { kind: 'app' }));
  assert.equal(actions[0]?.title, 'Open Helium');
  assert.equal(actions[0]?.primary, true);
  assert.ok(actions.some((action) => action.title === 'Uninstall Helium…'));
});
