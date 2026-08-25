import assert from 'node:assert/strict';
import test from 'node:test';
import { createSelectedTextReader } from './selected-text';

function reader(options: {
  accessibilityText?: string | null;
  copiedText?: string;
  paletteFocused?: boolean;
}) {
  let clipboard = 'original clipboard';
  const concealed: string[] = [];
  let copyCalls = 0;
  let restoreCalls = 0;
  const selectedText = createSelectedTextReader({
    readAccessibilityText: async () => options.accessibilityText,
    paletteIsFocused: () => Boolean(options.paletteFocused),
    clipboardSnapshot: () => clipboard,
    readClipboardText: () => clipboard,
    writeClipboardText: (text) => {
      clipboard = text;
    },
    restoreClipboardSnapshot: (snapshot) => {
      restoreCalls += 1;
      clipboard = snapshot;
    },
    copySelectionIntoClipboard: async () => {
      copyCalls += 1;
      if (options.copiedText !== undefined) clipboard = options.copiedText;
      return true;
    },
    concealClipboardText: (text) => concealed.push(text),
    delay: async () => {},
    sentinel: () => 'selection sentinel',
  });
  return {
    selectedText,
    concealed,
    clipboard: () => clipboard,
    copyCalls: () => copyCalls,
    restoreCalls: () => restoreCalls,
  };
}

test('returns accessibility text without touching the clipboard', async () => {
  const fixture = reader({ accessibilityText: '  selected text\n' });

  assert.equal(await fixture.selectedText(), '  selected text\n');
  assert.equal(fixture.copyCalls(), 0);
  assert.equal(fixture.restoreCalls(), 0);
});

test('copies selected text and restores the clipboard when accessibility returns null', async () => {
  const fixture = reader({
    accessibilityText: null,
    copiedText: 'fallback text',
  });

  assert.equal(await fixture.selectedText(), 'fallback text');
  assert.equal(fixture.clipboard(), 'original clipboard');
  assert.equal(fixture.restoreCalls(), 1);
  assert.deepEqual(fixture.concealed, ['selection sentinel', 'fallback text']);
});

test('does not copy palette input when the palette still has focus', async () => {
  const fixture = reader({
    accessibilityText: 'selected palette query',
    paletteFocused: true,
  });

  assert.equal(await fixture.selectedText(), null);
  assert.equal(fixture.copyCalls(), 0);
});
