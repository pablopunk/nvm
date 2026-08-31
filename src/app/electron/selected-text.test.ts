import assert from 'node:assert/strict';
import test from 'node:test';
import { createSelectedTextReader } from './selected-text';

const RESTORED_ACCESSIBILITY_CALLS = 3;

function reader(options: {
  accessibilityText?: string | null;
  accessibilityTexts?: Array<string | null>;
  copiedText?: string;
  paletteFocused?: boolean;
  focusAfterAccessibility?: boolean;
}) {
  let clipboard = 'original clipboard';
  const concealed: string[] = [];
  let copyCalls = 0;
  let restoreCalls = 0;
  let accessibilityCalls = 0;
  const selectedText = createSelectedTextReader({
    selectionTarget: () => 'source-app',
    readAccessibilityText: () => {
      if (options.focusAfterAccessibility) {
        options.paletteFocused = true;
      }
      const text = options.accessibilityTexts
        ? options.accessibilityTexts[
            Math.min(accessibilityCalls, options.accessibilityTexts.length - 1)
          ]
        : options.accessibilityText;
      accessibilityCalls += 1;
      return Promise.resolve(text);
    },
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
    copySelectionIntoClipboard: () => {
      copyCalls += 1;
      if (options.copiedText !== undefined) {
        clipboard = options.copiedText;
      }
      return Promise.resolve(true);
    },
    concealClipboardText: (text) => concealed.push(text),
    delay: () => Promise.resolve(),
    sentinel: () => 'selection sentinel',
  });
  return {
    selectedText,
    concealed,
    clipboard: () => clipboard,
    copyCalls: () => copyCalls,
    restoreCalls: () => restoreCalls,
    accessibilityCalls: () => accessibilityCalls,
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

test('waits for the source app selection to return after palette dismissal', async () => {
  const fixture = reader({
    accessibilityTexts: [null, null, 'restored selection'],
  });

  assert.equal(await fixture.selectedText(), 'restored selection');
  assert.equal(fixture.accessibilityCalls(), RESTORED_ACCESSIBILITY_CALLS);
  assert.equal(fixture.copyCalls(), 0);
  assert.equal(fixture.restoreCalls(), 0);
});

test('does not copy palette input when the palette still has focus', async () => {
  const fixture = reader({
    accessibilityText: 'selected palette query',
    paletteFocused: true,
  });

  assert.equal(await fixture.selectedText(), null);
  assert.equal(fixture.copyCalls(), 0);
});

test('does not copy when the palette regains focus during accessibility capture', async () => {
  const fixture = reader({
    accessibilityText: null,
    copiedText: 'selected palette query',
    focusAfterAccessibility: true,
  });

  assert.equal(await fixture.selectedText(), null);
  assert.equal(fixture.copyCalls(), 0);
});
