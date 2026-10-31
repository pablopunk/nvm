import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

const stylesDirectory = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  'styles',
);
const windowsCss = fs.readFileSync(
  path.join(stylesDirectory, 'extension-windows.css'),
  'utf8',
);
const paletteCss = fs.readFileSync(
  path.join(stylesDirectory, 'palette.css'),
  'utf8',
);
const ANCHOR_RULE_PATTERN = /\.paletteCompactPanelAnchor \{([\s\S]*?)\n\}/;

function declarationsOf(rule: RegExpMatchArray | null) {
  return rule?.[1] ?? '';
}

test('the compact action panel hangs from a zero-height anchor at the end of the pane flow', () => {
  const anchor = declarationsOf(windowsCss.match(ANCHOR_RULE_PATTERN));
  assert.match(anchor, /position: relative;/);
  assert.match(anchor, /height: 0;/);
  assert.match(anchor, /flex: none;/);
});

test('the anchor cancels the pane gap so the panel measures from the last card edge', () => {
  const anchor = declarationsOf(windowsCss.match(ANCHOR_RULE_PATTERN));
  assert.match(anchor, /margin-top: calc\(-1 \* var\(--palette-stack-gap\)\);/);
  assert.match(paletteCss, /gap: var\(--palette-stack-gap\);/);
});

test('stacked panes widen the anchor over their right padding like the panel always assumed', () => {
  assert.match(
    windowsCss,
    /\.palette\.isStacked \.paletteCompactPanelAnchor \{\s*margin-right: calc\(-1 \* var\(--stack-pane-offset\)\);/,
  );
  assert.match(paletteCss, /padding-right: var\(--stack-pane-offset\);/);
});
