import assert from 'node:assert/strict';
import test from 'node:test';
import {
  COMPACT_PANEL_PREFERRED_MAX_HEIGHT,
  compactPanelMaxHeight,
} from './compact-panel-placement';

test('a tall palette lets the panel use its preferred height', () => {
  assert.equal(
    compactPanelMaxHeight({ paletteTop: 96, cardBottom: 900 }),
    COMPACT_PANEL_PREFERRED_MAX_HEIGHT,
  );
});

test('a short card limits the panel to the room between the palette top and the card edge', () => {
  assert.equal(compactPanelMaxHeight({ paletteTop: 96, cardBottom: 282 }), 162);
});

test('a card edge at the palette top still leaves room for the filter row and one action', () => {
  const height = compactPanelMaxHeight({ paletteTop: 96, cardBottom: 100 });
  assert.ok(height >= 96, `expected a usable height, received ${height}`);
});

test('fractional layout positions produce whole pixel heights', () => {
  assert.equal(
    Number.isInteger(
      compactPanelMaxHeight({ paletteTop: 96.4, cardBottom: 250.7 }),
    ),
    true,
  );
});
