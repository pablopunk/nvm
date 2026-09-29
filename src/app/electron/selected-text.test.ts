import assert from 'node:assert/strict';
import test from 'node:test';
import { createSelectedTextReader } from './selected-text';

function reader(options: {
  accessibilityText?: string | null;
  copiedText?: string;
  paletteFocused?: boolean;
  focusAfterAccessibility?: boolean;
  initialClipboard?: string;
  copyAfterPoll?: number;
  copyError?: Error;
  copySucceeded?: boolean;
  accessibilityError?: Error;
  clipboardReadError?: Error;
  missingTarget?: boolean;
}) {
  let clipboard = options.initialClipboard ?? 'original clipboard';
  const concealed: string[] = [];
  let copyCalls = 0;
  let restoreCalls = 0;
  let delayCalls = 0;
  const selectedText = createSelectedTextReader({
    selectionTarget: () => (options.missingTarget ? null : 'source-app'),
    readAccessibilityText: () => {
      if (options.accessibilityError)
        return Promise.reject(options.accessibilityError);
      if (options.focusAfterAccessibility) {
        options.paletteFocused = true;
      }
      return Promise.resolve(options.accessibilityText);
    },
    paletteIsFocused: () => Boolean(options.paletteFocused),
    clipboardSnapshot: () => clipboard,
    readClipboardText: async () => {
      if (options.clipboardReadError) throw options.clipboardReadError;
      await Promise.resolve();
      return clipboard;
    },
    writeClipboardText: async (text) => {
      clipboard = text;
    },
    restoreClipboardSnapshot: async (snapshot) => {
      restoreCalls += 1;
      clipboard = snapshot;
    },
    copySelectionIntoClipboard: () => {
      copyCalls += 1;
      if (options.copyError) return Promise.reject(options.copyError);
      if (options.copiedText !== undefined && !options.copyAfterPoll) {
        clipboard = options.copiedText;
      }
      return Promise.resolve(options.copySucceeded ?? true);
    },
    concealClipboardText: (text) => concealed.push(text),
    delay: async () => {
      delayCalls += 1;
      await Promise.resolve();
      assert.equal(
        restoreCalls,
        0,
        'clipboard must stay owned until capture finishes',
      );
      if (options.copyAfterPoll === delayCalls)
        clipboard = options.copiedText ?? '';
    },
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

  await assert.rejects(fixture.selectedText(), /Nevermind still has focus/);
  assert.equal(fixture.copyCalls(), 0);
});

test('does not copy when the palette regains focus during accessibility capture', async () => {
  const fixture = reader({
    accessibilityText: null,
    copiedText: 'selected palette query',
    focusAfterAccessibility: true,
  });

  await assert.rejects(fixture.selectedText(), /Nevermind regained focus/);
  assert.equal(fixture.copyCalls(), 0);
});

test('waits for delayed Copy before restoring an asynchronous clipboard', async () => {
  const fixture = reader({ copiedText: '  hello there\n', copyAfterPoll: 3 });
  assert.equal(await fixture.selectedText(), '  hello there\n');
  assert.equal(fixture.clipboard(), 'original clipboard');
  assert.equal(fixture.restoreCalls(), 1);
  assert.deepEqual(fixture.concealed, [
    'selection sentinel',
    '  hello there\n',
  ]);
});

for (const initialClipboard of ['', 'unrelated clipboard text']) {
  test(`does not return the original clipboard when Copy produces no text (${JSON.stringify(initialClipboard)})`, async () => {
    const fixture = reader({ initialClipboard });
    assert.equal(await fixture.selectedText(), null);
    assert.equal(fixture.clipboard(), initialClipboard);
    assert.equal(fixture.restoreCalls(), 1);
    assert.deepEqual(fixture.concealed, ['selection sentinel']);
  });
}

test('restores the clipboard before a Copy permission failure reaches the caller', async () => {
  const error = new Error('Accessibility access is blocked');
  const fixture = reader({ copyError: error });
  await assert.rejects(fixture.selectedText(), error);
  assert.equal(fixture.clipboard(), 'original clipboard');
  assert.equal(fixture.restoreCalls(), 1);
});

test('does not touch the clipboard after an Accessibility failure', async () => {
  const error = new Error('Accessibility access is blocked');
  const fixture = reader({ accessibilityError: error });
  await assert.rejects(fixture.selectedText(), error);
  assert.equal(fixture.copyCalls(), 0);
  assert.equal(fixture.restoreCalls(), 0);
});

test('restores the clipboard on clipboard read failure', async () => {
  const error = new Error('Clipboard unavailable');
  const fixture = reader({ clipboardReadError: error });
  await assert.rejects(fixture.selectedText(), error);
  assert.equal(fixture.clipboard(), 'original clipboard');
  assert.equal(fixture.restoreCalls(), 1);
});

test('reports a missing source app instead of an empty selection', async () => {
  const fixture = reader({ missingTarget: true });
  await assert.rejects(
    fixture.selectedText(),
    /Could not identify the source app/,
  );
  assert.equal(fixture.copyCalls(), 0);
});

test('reports failed Copy instead of an empty selection', async () => {
  const fixture = reader({ copySucceeded: false });
  await assert.rejects(fixture.selectedText(), /Could not copy selected text/);
  assert.equal(fixture.clipboard(), 'original clipboard');
});

test('shares one clipboard transaction between concurrent readers', async () => {
  const fixture = reader({ copiedText: 'selection', copyAfterPoll: 2 });
  assert.deepEqual(
    await Promise.all([fixture.selectedText(), fixture.selectedText()]),
    ['selection', 'selection'],
  );
  assert.equal(fixture.copyCalls(), 1);
  assert.equal(fixture.restoreCalls(), 1);
});
