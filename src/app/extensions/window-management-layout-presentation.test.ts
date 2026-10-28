import assert from 'node:assert/strict';
import test from 'node:test';
import { WINDOW_LAYOUT_IDS } from '../electron/os/window-layout-catalog';
import { WINDOW_LAYOUT_PRESENTATION } from './window-management-layout-presentation';

const ICON_NAME_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;

test('presentation table covers exactly the catalog layout ids', () => {
  assert.deepEqual(
    Object.keys(WINDOW_LAYOUT_PRESENTATION).sort(),
    [...WINDOW_LAYOUT_IDS].sort(),
  );
});

test('every layout has a unique title and a kebab-case lucide icon name', () => {
  const presentations = WINDOW_LAYOUT_IDS.map(
    (id) => WINDOW_LAYOUT_PRESENTATION[id],
  );
  const titles = presentations.map((presentation) => presentation.title);
  assert.equal(new Set(titles).size, titles.length);
  for (const presentation of presentations) {
    assert.ok(presentation.title.trim().length > 0);
    assert.match(presentation.icon, ICON_NAME_PATTERN);
    assert.ok(presentation.keywords.length > 0);
  }
});

test('P1 layout titles are unchanged', () => {
  assert.deepEqual(
    [
      'left-half',
      'right-half',
      'top-half',
      'bottom-half',
      'maximize',
      'center',
    ].map(
      (id) =>
        WINDOW_LAYOUT_PRESENTATION[
          id as keyof typeof WINDOW_LAYOUT_PRESENTATION
        ].title,
    ),
    [
      'Left Half',
      'Right Half',
      'Top Half',
      'Bottom Half',
      'Maximize',
      'Center',
    ],
  );
});
